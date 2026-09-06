package roo

import (
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

	// Fallback: try parsing as raw []ClineMessage
	var messages []ClineMessage
	if err := json.Unmarshal(data, &messages); err == nil {
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

	return RooTaskEnvelope{}, errors.New("unrecognized Roo transcript format")
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
	env, err := decodeEnvelope(data)
	if err != nil {
		return protocol.AgentSessionJSON{}, err
	}

	if sessionID == "" {
		sessionID = env.TaskID
	}
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(sessionRef), filepath.Ext(sessionRef))
	}

	startTime := time.Now().UTC()
	if env.CreatedAt > 0 {
		startTime = time.UnixMilli(env.CreatedAt).UTC()
	} else if len(env.UiMessages) > 0 && env.UiMessages[0].Ts > 0 {
		startTime = time.UnixMilli(env.UiMessages[0].Ts).UTC()
	}

	modified := modifiedFilesFromEnvelope(env, 0)

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
	uiMessagesPath := filepath.Join(taskFolder, "ui_messages.json")
	apiHistoryPath := filepath.Join(taskFolder, "api_conversation_history.json")

	uiData, err := os.ReadFile(uiMessagesPath)
	if err != nil {
		// Task folder might not exist on disk in some environments; ignore error
		return nil
	}

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

	encoded, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
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
	if _, err := decodeEnvelope(data); err != nil {
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
	env, err := decodeEnvelope(data)
	if err != nil {
		return 0, err
	}
	return len(env.UiMessages), nil
}

func (a *Agent) ExtractModifiedFiles(path string, offset int) ([]string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	env, err := decodeEnvelope(data)
	if err != nil {
		return nil, 0, err
	}
	files := modifiedFilesFromEnvelope(env, offset)
	return files, len(env.UiMessages), nil
}

func (a *Agent) ExtractPrompts(sessionRef string, offset int) ([]string, error) {
	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return nil, err
	}
	env, err := decodeEnvelope(data)
	if err != nil {
		return nil, err
	}

	var prompts []string
	messages := env.UiMessages
	if offset < 0 {
		offset = 0
	}
	if offset < len(messages) {
		for _, msg := range messages[offset:] {
			if msg.Type == "say" && (msg.Say == "task" || msg.Say == "user_feedback") {
				if text := strings.TrimSpace(msg.Text); text != "" {
					prompts = append(prompts, text)
				}
			}
		}
	}

	// Fallback to ApiConversationHistory if uiMessages had no prompts
	if len(prompts) == 0 && len(env.ApiConversationHistory) > 0 {
		for _, msg := range env.ApiConversationHistory {
			if msg.Role == "user" {
				for _, part := range msg.Content {
					if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
						prompts = append(prompts, strings.TrimSpace(part.Text))
					}
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
	env, err := decodeEnvelope(data)
	if err != nil {
		return "", false, err
	}

	// 1. Look for completion_result
	for i := len(env.UiMessages) - 1; i >= 0; i-- {
		msg := env.UiMessages[i]
		if msg.Say == "completion_result" && strings.TrimSpace(msg.Text) != "" {
			return strings.TrimSpace(msg.Text), true, nil
		}
	}

	// 2. Look for last non-empty assistant text in ui_messages
	for i := len(env.UiMessages) - 1; i >= 0; i-- {
		msg := env.UiMessages[i]
		if msg.Type == "say" && msg.Say == "text" && strings.TrimSpace(msg.Text) != "" {
			return strings.TrimSpace(msg.Text), true, nil
		}
	}

	// 3. Fallback to last assistant text in api_conversation_history
	for i := len(env.ApiConversationHistory) - 1; i >= 0; i-- {
		msg := env.ApiConversationHistory[i]
		if msg.Role == "assistant" {
			for _, part := range msg.Content {
				if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
					return strings.TrimSpace(part.Text), true, nil
				}
			}
		}
	}

	return "", false, nil
}

func (a *Agent) CalculateTokens(data []byte, offset int) (protocol.TokenUsageResponse, error) {
	env, err := decodeEnvelope(data)
	if err != nil {
		return protocol.TokenUsageResponse{}, err
	}

	var usage protocol.TokenUsageResponse
	messages := env.UiMessages
	if offset < 0 {
		offset = 0
	}
	if offset < len(messages) {
		for _, msg := range messages[offset:] {
			if msg.Say == "api_req_started" && msg.Text != "" {
				var meta ApiReqStartedData
				if err := json.Unmarshal([]byte(msg.Text), &meta); err == nil {
					usage.InputTokens += meta.TokensIn
					usage.OutputTokens += meta.TokensOut
					usage.CacheReadTokens += meta.CacheReads
					usage.CacheCreationTokens += meta.CacheWrites
					usage.APICallCount++
				}
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
	env, err := decodeEnvelope(data)
	if err != nil {
		return protocol.CompactTranscriptResponse{}, err
	}

	var lines []string
	for _, msg := range env.UiMessages {
		if msg.Text == "" {
			continue
		}
		role := "assistant"
		if msg.Say == "task" || msg.Say == "user_feedback" {
			role = "user"
		}
		entry := map[string]interface{}{
			"role":    role,
			"content": msg.Text,
			"ts":      msg.Ts,
		}
		if raw, err := json.Marshal(entry); err == nil {
			lines = append(lines, string(raw))
		}
	}

	joined := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(joined))
	return protocol.CompactTranscriptResponse{Transcript: encoded}, nil
}

func modifiedFilesFromEnvelope(env RooTaskEnvelope, offset int) []string {
	seen := map[string]bool{}

	// Scan UI messages for mutating tools
	messages := env.UiMessages
	if offset < 0 {
		offset = 0
	}
	if offset < len(messages) {
		for _, msg := range messages[offset:] {
			if msg.Say == "tool" && msg.Text != "" {
				var toolCall struct {
					Tool    string `json:"tool"`
					Path    string `json:"path"`
					Diff    string `json:"diff"`
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(msg.Text), &toolCall); err == nil {
					if isMutatingToolName(toolCall.Tool) && toolCall.Path != "" {
						if clean := cleanFile(toolCall.Path); clean != "" {
							seen[clean] = true
						}
					}
				}
			}
		}
	}

	// Scan ApiConversationHistory for tool_use parts
	for _, msg := range env.ApiConversationHistory {
		for _, part := range msg.Content {
			if part.Type == "tool_use" && isMutatingToolName(part.Name) && len(part.Input) > 0 {
				var input struct {
					Path     string   `json:"path"`
					FilePath string   `json:"filePath"`
					File     string   `json:"file"`
					Paths    []string `json:"paths"`
					Files    []string `json:"files"`
				}
				if err := json.Unmarshal(part.Input, &input); err == nil {
					if input.Path != "" {
						if clean := cleanFile(input.Path); clean != "" {
							seen[clean] = true
						}
					}
					if input.FilePath != "" {
						if clean := cleanFile(input.FilePath); clean != "" {
							seen[clean] = true
						}
					}
					if input.File != "" {
						if clean := cleanFile(input.File); clean != "" {
							seen[clean] = true
						}
					}
					for _, p := range input.Paths {
						if clean := cleanFile(p); clean != "" {
							seen[clean] = true
						}
					}
					for _, f := range input.Files {
						if clean := cleanFile(f); clean != "" {
							seen[clean] = true
						}
					}
				}
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
