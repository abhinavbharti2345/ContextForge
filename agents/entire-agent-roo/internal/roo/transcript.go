package roo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

func decodeEnvelope(data []byte) (RooTaskEnvelope, error) {
	var env RooTaskEnvelope
	if err := json.Unmarshal(data, &env); err == nil && (len(env.UiMessages) > 0 || env.TaskID != "") {
		return env, nil
	}
	var messages []ClineMessage
	if err := json.Unmarshal(data, &messages); err == nil && len(messages) > 0 {
		var firstTs int64
		if len(messages) > 0 {
			firstTs = messages[0].Ts
		}
		return RooTaskEnvelope{
			CreatedAt:  firstTs,
			UpdatedAt:  time.Now().UnixMilli(),
			UiMessages: messages,
		}, nil
	}
	return RooTaskEnvelope{}, errors.New("cannot decode as RooTaskEnvelope")
}

func LoadSession(data []byte) (NormalizedSession, error) {
	if len(data) == 0 {
		return NormalizedSession{}, errors.New("empty data")
	}

	// Heuristic for JSONL: starts with { and has newlines, or fails to unmarshal as a monolithic struct
	var env RooTaskEnvelope
	errEnv := json.Unmarshal(data, &env)
	if errEnv == nil && (len(env.UiMessages) > 0 || env.TaskID != "") {
		return convertEnvelopeToNormalized(env), nil
	}

	// Try as raw []ClineMessage
	var messages []ClineMessage
	if err := json.Unmarshal(data, &messages); err == nil && len(messages) > 0 {
		var firstTs int64
		if len(messages) > 0 {
			firstTs = messages[0].Ts
		}
		env = RooTaskEnvelope{
			CreatedAt:  firstTs,
			UpdatedAt:  time.Now().UnixMilli(),
			UiMessages: messages,
		}
		return convertEnvelopeToNormalized(env), nil
	}

	// Try as JSONL
	session, err := parseJSONLSession(data)
	if err == nil && len(session.Events) > 0 {
		return session, nil
	}

	return NormalizedSession{}, errors.New("unrecognized Roo transcript format")
}

func parseJSONLSession(data []byte) (NormalizedSession, error) {
	var session NormalizedSession
	lines := bytes.Split(data, []byte("\n"))
	
	for i, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(line, &raw); err != nil {
			if i == len(lines)-1 || (i == len(lines)-2 && len(bytes.TrimSpace(lines[len(lines)-1])) == 0) {
				// Last line is truncated
				session.Partial = true
				break
			}
			continue
		}

		event := NormalizedEvent{Raw: line}
		if tsStr, ok := raw["timestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				event.Timestamp = t
			}
		}

		if sid, ok := raw["session_id"].(string); ok && session.SessionID == "" {
			session.SessionID = sid
		}

		evtType, _ := raw["event"].(string)
		event.Type = EventType(evtType)

		switch event.Type {
		case EventSessionStarted:
			event.IsTurnComplete = false
		case EventToolResult:
			event.IsTurnComplete = false
		case EventFileRead:
			event.IsTurnComplete = false
		case EventUserPrompt:
			event.Role = "user"
			event.Text, _ = raw["text"].(string)
			event.IsTurnComplete = false
		case EventAgentResponse:
			event.Role = "assistant"
			event.Text, _ = raw["text"].(string)
			event.IsTurnComplete = true
		case EventToolCall:
			event.ToolName, _ = raw["tool"].(string)
			if inputMap, ok := raw["input"].(map[string]interface{}); ok {
				inputBytes, _ := json.Marshal(inputMap)
				event.ToolInput = inputBytes
			}
			event.IsTurnComplete = false
		case EventFileChanged:
			if path, ok := raw["path"].(string); ok {
				event.ModifiedFiles = append(event.ModifiedFiles, path)
			}
			event.IsTurnComplete = false
		case EventSessionEnded:
			event.IsTurnComplete = true
			event.IsTaskComplete = true
		case EventUsage:
			if in, ok := raw["input_tokens"].(float64); ok {
				event.InputTokens = int(in)
			}
			if out, ok := raw["output_tokens"].(float64); ok {
				event.OutputTokens = int(out)
			}
		case EventCheckpoint:
			event.IsTurnComplete = true
		default:
			// Ensure unknown events don't crash and are mapped safely
			event.Type = EventUnknown
		}

		session.Events = append(session.Events, event)
	}

	return session, nil
}

func convertEnvelopeToNormalized(env RooTaskEnvelope) NormalizedSession {
	var session NormalizedSession
	session.SessionID = env.TaskID

	for _, msg := range env.UiMessages {
		event := NormalizedEvent{
			Timestamp: msg.Time(),
			Raw:       nil,
		}

		if msg.Type == "say" && (msg.Say == "task" || msg.Say == "user_feedback") {
			event.Type = EventUserPrompt
			event.Role = "user"
			event.Text = msg.Text
		} else if msg.Type == "say" && msg.Say == "text" {
			event.Type = EventAgentResponse
			event.Role = "assistant"
			event.Text = msg.Text
			if !msg.Partial {
				event.IsTurnComplete = true
			}
		} else if msg.Type == "say" && msg.Say == "completion_result" {
			event.Type = EventAgentResponse
			event.Role = "assistant"
			event.Text = msg.Text
			event.IsTurnComplete = true
			event.IsTaskComplete = true
		} else if msg.Say == "tool" && msg.Text != "" {
			event.Type = EventToolCall
			var toolCall struct {
				Tool string `json:"tool"`
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(msg.Text), &toolCall); err == nil {
				event.ToolName = toolCall.Tool
				if isMutatingToolName(toolCall.Tool) && toolCall.Path != "" {
					event.ModifiedFiles = append(event.ModifiedFiles, cleanFile(toolCall.Path))
				}
			}
		} else if msg.Say == "api_req_started" && msg.Text != "" {
			event.Type = EventUsage
			var meta ApiReqStartedData
			if err := json.Unmarshal([]byte(msg.Text), &meta); err == nil {
				event.InputTokens = meta.TokensIn
				event.OutputTokens = meta.TokensOut
			}
		} else {
			event.Type = EventUnknown
		}

		session.Events = append(session.Events, event)
	}

	// Check ApiConversationHistory for tool mutations
	for _, msg := range env.ApiConversationHistory {
		for _, part := range msg.Content {
			if part.Type == "tool_use" && isMutatingToolName(part.Name) && len(part.Input) > 0 {
				event := NormalizedEvent{
					Type:     EventToolCall,
					ToolName: part.Name,
				}
				var input struct {
					Path     string   `json:"path"`
					FilePath string   `json:"filePath"`
					File     string   `json:"file"`
					Paths    []string `json:"paths"`
					Files    []string `json:"files"`
				}
				if err := json.Unmarshal(part.Input, &input); err == nil {
					if input.Path != "" {
						event.ModifiedFiles = append(event.ModifiedFiles, cleanFile(input.Path))
					}
					if input.FilePath != "" {
						event.ModifiedFiles = append(event.ModifiedFiles, cleanFile(input.FilePath))
					}
					if input.File != "" {
						event.ModifiedFiles = append(event.ModifiedFiles, cleanFile(input.File))
					}
					for _, p := range input.Paths {
						event.ModifiedFiles = append(event.ModifiedFiles, cleanFile(p))
					}
					for _, f := range input.Files {
						event.ModifiedFiles = append(event.ModifiedFiles, cleanFile(f))
					}
				}
				session.Events = append(session.Events, event)
			}
		}
	}

	return session
}

func (a *Agent) ReadSession(input *protocol.HookInputJSON) (protocol.AgentSessionJSON, error) {
	var sessionID string
	var sessionRef string
	if input != nil {
		sessionID = input.SessionID
		sessionRef = input.SessionRef
		if sessionRef == "" && sessionID != "" {
			sessionRef = transcriptPath(sessionID)
		}
	}
	if sessionRef == "" {
		return protocol.AgentSessionJSON{}, errors.New("session_ref or session_id is required")
	}

	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return protocol.AgentSessionJSON{}, err
	}
	session, err := LoadSession(data)
	if err != nil {
		return protocol.AgentSessionJSON{}, err
	}

	if sessionID == "" {
		sessionID = session.SessionID
	}
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(sessionRef), filepath.Ext(sessionRef))
	}

	startTime := time.Now().UTC()
	if len(session.Events) > 0 {
		startTime = session.Events[0].Timestamp
	}

	modified := modifiedFilesFromSession(session, 0)

	return protocol.AgentSessionJSON{
		SessionID:     sessionID,
		AgentName:     "roo",
		RepoPath:      protocol.RepoRoot(),
		SessionRef:    sessionRef,
		StartTime:     startTime.Format(time.RFC3339),
		NativeData:    data,
		ModifiedFiles: modified,
		NewFiles:      []string{},
		DeletedFiles:  []string{},
	}, nil
}

func (a *Agent) PrepareTranscript(sessionRef string) error {
	if strings.TrimSpace(sessionRef) == "" {
		return errors.New("session_ref is required")
	}

	taskID := strings.TrimSuffix(filepath.Base(sessionRef), filepath.Ext(sessionRef))
	tasksDir := GetGlobalStorageTasksDir()
	if tasksDir == "" {
		return nil
	}

	taskFolder := filepath.Join(tasksDir, taskID)
	
	// Check for new JSONL format
	jsonlPath := filepath.Join(taskFolder, "transcript.jsonl") // Assumption or try finding any .jsonl
	
	// Fallback to old format
	uiMessagesPath := filepath.Join(taskFolder, "ui_messages.json")
	apiHistoryPath := filepath.Join(taskFolder, "api_conversation_history.json")

	var encoded []byte

	if data, err := os.ReadFile(jsonlPath); err == nil {
		encoded = data
	} else if uiData, err := os.ReadFile(uiMessagesPath); err == nil {
		var uiMessages []ClineMessage
		if err := json.Unmarshal(uiData, &uiMessages); err != nil {
			return err
		}

		var apiHistory []ApiMessage
		if apiData, err := os.ReadFile(apiHistoryPath); err == nil {
			_ = json.Unmarshal(apiData, &apiHistory)
		}

		var firstTs, lastTs int64
		if len(uiMessages) > 0 {
			firstTs = uiMessages[0].Ts
			lastTs = uiMessages[len(uiMessages)-1].Ts
		}

		env := RooTaskEnvelope{
			TaskID:                 taskID,
			CreatedAt:              firstTs,
			UpdatedAt:              lastTs,
			UiMessages:             uiMessages,
			ApiConversationHistory: apiHistory,
		}

		encoded, err = json.MarshalIndent(env, "", "  ")
		if err != nil {
			return err
		}
	} else {
		// Task folder might not exist on disk
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(sessionRef), 0o750); err != nil {
		return err
	}
	return atomicWriteFile(sessionRef, encoded, 0o600)
}

func (a *Agent) WriteSession(session protocol.AgentSessionJSON) error {
	if session.SessionRef == "" {
		return errors.New("session_ref is required")
	}
	if err := os.MkdirAll(filepath.Dir(session.SessionRef), 0o700); err != nil {
		return err
	}
	return atomicWriteFile(session.SessionRef, session.NativeData, 0o600)
}

func (a *Agent) ReadTranscript(sessionRef string) ([]byte, error) {
	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return nil, err
	}
	if _, err := LoadSession(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (a *Agent) ChunkTranscript(content []byte, maxSize int) ([][]byte, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("max-size must be positive, got %d", maxSize)
	}
	var chunks [][]byte
	for len(content) > 0 {
		end := min(maxSize, len(content))
		chunks = append(chunks, content[:end])
		content = content[end:]
	}
	return chunks, nil
}

func (a *Agent) ReassembleTranscript(chunks [][]byte) ([]byte, error) {
	var data []byte
	for _, chunk := range chunks {
		data = append(data, chunk...)
	}
	return data, nil
}

func (a *Agent) GetTranscriptPosition(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	session, err := LoadSession(data)
	if err != nil {
		return 0, err
	}
	return len(session.Events), nil
}

func (a *Agent) ExtractModifiedFiles(path string, offset int) ([]string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	session, err := LoadSession(data)
	if err != nil {
		return nil, 0, err
	}
	files := modifiedFilesFromSession(session, offset)
	return files, len(session.Events), nil
}

func (a *Agent) ExtractPrompts(sessionRef string, offset int) ([]string, error) {
	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return nil, err
	}
	session, err := LoadSession(data)
	if err != nil {
		return nil, err
	}

	var prompts []string
	if offset < 0 {
		offset = 0
	}
	if offset < len(session.Events) {
		for _, msg := range session.Events[offset:] {
			if msg.Type == EventUserPrompt {
				if text := strings.TrimSpace(msg.Text); text != "" {
					prompts = append(prompts, text)
				}
			}
		}
	}

	return prompts, nil
}

func (a *Agent) ExtractSummary(sessionRef string) (string, bool, error) {
	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return "", false, err
	}
	session, err := LoadSession(data)
	if err != nil {
		return "", false, err
	}

	for i := len(session.Events) - 1; i >= 0; i-- {
		msg := session.Events[i]
		if msg.Type == EventAgentResponse && strings.TrimSpace(msg.Text) != "" {
			return strings.TrimSpace(msg.Text), true, nil
		}
	}

	return "", false, nil
}

func (a *Agent) CalculateTokens(data []byte, offset int) (protocol.TokenUsageResponse, error) {
	session, err := LoadSession(data)
	if err != nil {
		return protocol.TokenUsageResponse{}, err
	}

	var usage protocol.TokenUsageResponse
	if offset < 0 {
		offset = 0
	}
	if offset < len(session.Events) {
		for _, msg := range session.Events[offset:] {
			if msg.Type == EventUsage {
				usage.InputTokens += msg.InputTokens
				usage.OutputTokens += msg.OutputTokens
			}
			if msg.Type == EventToolCall || msg.Type == EventToolResult {
				usage.APICallCount++
			}
		}
	}

	return usage, nil
}

func (a *Agent) CompactTranscript(sessionRef string) (protocol.CompactTranscriptResponse, error) {
	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return protocol.CompactTranscriptResponse{}, err
	}
	session, err := LoadSession(data)
	if err != nil {
		return protocol.CompactTranscriptResponse{}, err
	}

	var lines []string
	for _, msg := range session.Events {
		if msg.Text == "" {
			continue
		}
		role := msg.Role
		if role == "" {
			role = "system"
		}
		entry := map[string]interface{}{
			"role":    role,
			"content": msg.Text,
			"ts":      msg.Timestamp.UnixMilli(),
		}
		if raw, err := json.Marshal(entry); err == nil {
			lines = append(lines, string(raw))
		}
	}

	joined := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(joined))
	return protocol.CompactTranscriptResponse{Transcript: encoded}, nil
}

func modifiedFilesFromSession(session NormalizedSession, offset int) []string {
	seen := map[string]bool{}

	if offset < 0 {
		offset = 0
	}
	if offset < len(session.Events) {
		for _, msg := range session.Events[offset:] {
			for _, file := range msg.ModifiedFiles {
				seen[file] = true
			}
		}
	}

	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

func isMutatingToolName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
	switch normalized {
	case "write_to_file", "write", "replace_in_file", "edit_file", "apply_diff",
		"create_file", "delete_file", "rename_file", "move_file", "patch":
		return true
	}
	return false
}

func cleanFile(file string) string {
	file = strings.TrimSpace(file)
	if file == "" {
		return ""
	}
	if u, err := url.Parse(file); err == nil && u.Scheme == "file" {
		file = u.Path
		if unescaped, err := url.PathUnescape(file); err == nil {
			file = unescaped
		}
	}
	return filepath.ToSlash(filepath.Clean(file))
}

func atomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	tmpFile, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filename)
}
