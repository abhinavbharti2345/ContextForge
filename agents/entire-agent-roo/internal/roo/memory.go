package roo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

type MemoryDecision struct {
	Statement string   `json:"statement"`
	Reason    string   `json:"reason,omitempty"`
	Evidence  []string `json:"evidence,omitempty"`
}

type MemoryFileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"` // "created", "modified", "deleted"
	Tool   string `json:"tool"`
}

type MemoryProblem struct {
	Description string `json:"description"`
	Tool        string `json:"tool,omitempty"`
	Error       string `json:"error,omitempty"`
}

type MemoryEvidence struct {
	Kind   string `json:"kind"` // "prompt", "checkpoint", "tool_trace", "transcript"
	Target string `json:"target"`
	Detail string `json:"detail,omitempty"`
}

type DevelopmentMemory struct {
	SessionID    string             `json:"session_id"`
	CheckpointID string             `json:"checkpoint_id,omitempty"`
	Timestamp    string             `json:"timestamp"`
	Intent       string             `json:"intent"`
	Decisions    []MemoryDecision   `json:"decisions,omitempty"`
	Changes      []MemoryFileChange `json:"changes"`
	Problems     []MemoryProblem    `json:"problems,omitempty"`
	Outcomes     []string           `json:"outcomes,omitempty"`
	Evidence     []MemoryEvidence   `json:"evidence"`
}

var (
	// Explicit decision patterns: look for explicit reasoning markers in assistant text
	reDecidedBecause = regexp.MustCompile(`(?i)(?:decided to|we choose to|choosing to|we separate|separating|refactored|switched to|using)\s+([^.\n,]+?)\s+(?:because|in order to|to prevent|to ensure|since|so that)\s+([^.\n]+)`)
	reDecisionOnly   = regexp.MustCompile(`(?i)(?:decision|architectural choice|we will):\s*([^.\n]+)`)
)

// ExtractMemory extracts structured DevelopmentMemory from a populated RooTaskEnvelope.
func ExtractMemory(env RooTaskEnvelope, sessionID, checkpointID string) DevelopmentMemory {
	if sessionID == "" {
		sessionID = env.TaskID
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if env.CreatedAt > 0 {
		now = time.UnixMilli(env.CreatedAt).UTC().Format(time.RFC3339)
	}

	memory := DevelopmentMemory{
		SessionID:    sessionID,
		CheckpointID: checkpointID,
		Timestamp:    now,
		Changes:      []MemoryFileChange{},
		Evidence:     []MemoryEvidence{},
	}

	// 1. Extract Intent
	for _, msg := range env.UiMessages {
		if msg.Type == "say" && (msg.Say == "task" || msg.Say == "user_feedback") {
			if text := strings.TrimSpace(msg.Text); text != "" {
				if memory.Intent == "" {
					memory.Intent = text
				}
				memory.Evidence = append(memory.Evidence, MemoryEvidence{
					Kind:   "prompt",
					Target: text,
					Detail: fmt.Sprintf("Timestamp: %d", msg.Ts),
				})
			}
		}
	}

	if memory.Intent == "" && len(env.ApiConversationHistory) > 0 {
		for _, msg := range env.ApiConversationHistory {
			if msg.Role == "user" {
				for _, part := range msg.Content {
					if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
						memory.Intent = strings.TrimSpace(part.Text)
						break
					}
				}
			}
			if memory.Intent != "" {
				break
			}
		}
	}

	// 2. Extract Changes from UI messages & API history
	seenFiles := map[string]bool{}
	for _, msg := range env.UiMessages {
		if msg.Say == "tool" && msg.Text != "" {
			var toolCall struct {
				Tool string `json:"tool"`
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(msg.Text), &toolCall); err == nil {
				if isMutatingToolName(toolCall.Tool) && toolCall.Path != "" {
					clean := cleanFile(toolCall.Path)
					if clean != "" && !seenFiles[clean] {
						seenFiles[clean] = true
						action := "modified"
						if strings.Contains(strings.ToLower(toolCall.Tool), "create") || strings.Contains(strings.ToLower(toolCall.Tool), "write") {
							action = "created/modified"
						}
						memory.Changes = append(memory.Changes, MemoryFileChange{
							Path:   clean,
							Action: action,
							Tool:   toolCall.Tool,
						})
						memory.Evidence = append(memory.Evidence, MemoryEvidence{
							Kind:   "tool_trace",
							Target: clean,
							Detail: fmt.Sprintf("Tool: %s at %d", toolCall.Tool, msg.Ts),
						})
					}
				}
			}
		}
	}

	for _, msg := range env.ApiConversationHistory {
		for _, part := range msg.Content {
			if part.Type == "tool_use" && isMutatingToolName(part.Name) && len(part.Input) > 0 {
				var input struct {
					Path     string `json:"path"`
					FilePath string `json:"filePath"`
					File     string `json:"file"`
				}
				if err := json.Unmarshal(part.Input, &input); err == nil {
					target := input.Path
					if target == "" {
						target = input.FilePath
					}
					if target == "" {
						target = input.File
					}
					clean := cleanFile(target)
					if clean != "" && !seenFiles[clean] {
						seenFiles[clean] = true
						memory.Changes = append(memory.Changes, MemoryFileChange{
							Path:   clean,
							Action: "modified",
							Tool:   part.Name,
						})
					}
				}
			}
		}
	}

	// 3. Extract Explicit Decisions (Zero Hallucination Rule)
	for _, msg := range env.UiMessages {
		if msg.Type == "say" && msg.Say == "text" && msg.Text != "" {
			extractDecisionsFromText(msg.Text, &memory)
		}
	}
	for _, msg := range env.ApiConversationHistory {
		if msg.Role == "assistant" {
			for _, part := range msg.Content {
				if part.Type == "text" && part.Text != "" {
					extractDecisionsFromText(part.Text, &memory)
				}
			}
		}
	}

	// 4. Extract Problems & Errors
	for _, msg := range env.UiMessages {
		if msg.Say == "error" || (msg.Say == "command" && strings.Contains(msg.Text, "failed")) {
			memory.Problems = append(memory.Problems, MemoryProblem{
				Description: strings.TrimSpace(msg.Text),
				Tool:        msg.Say,
			})
		}
	}
	for _, msg := range env.ApiConversationHistory {
		for _, part := range msg.Content {
			if part.Type == "tool_result" && (strings.Contains(part.Content, "Error") || strings.Contains(part.Content, "failed")) {
				memory.Problems = append(memory.Problems, MemoryProblem{
					Description: "Tool execution returned an error",
					Error:       strings.TrimSpace(part.Content),
				})
			}
		}
	}

	// 5. Extract Outcomes
	for _, msg := range env.UiMessages {
		if msg.Say == "completion_result" && strings.TrimSpace(msg.Text) != "" {
			memory.Outcomes = append(memory.Outcomes, strings.TrimSpace(msg.Text))
		}
	}

	return memory
}

// ExtractMemoryFromSession extracts structured DevelopmentMemory from a NormalizedSession.
func ExtractMemoryFromSession(session NormalizedSession, sessionID, checkpointID string) DevelopmentMemory {
	if sessionID == "" {
		sessionID = session.SessionID
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if len(session.Events) > 0 && !session.Events[0].Timestamp.IsZero() {
		now = session.Events[0].Timestamp.UTC().Format(time.RFC3339)
	}

	memory := DevelopmentMemory{
		SessionID:    sessionID,
		CheckpointID: checkpointID,
		Timestamp:    now,
		Changes:      []MemoryFileChange{},
		Evidence:     []MemoryEvidence{},
	}

	// 0. Extract CheckpointID if not provided
	if memory.CheckpointID == "" {
		for _, evt := range session.Events {
			if evt.Type == EventCheckpoint {
				var rawData struct {
					CheckpointID string `json:"checkpoint_id"`
				}
				if err := json.Unmarshal(evt.Raw, &rawData); err == nil && rawData.CheckpointID != "" {
					memory.CheckpointID = rawData.CheckpointID
					break
				}
			}
		}
	}

	// 1. Extract Intent
	for _, evt := range session.Events {
		if evt.Type == EventUserPrompt {
			text := strings.TrimSpace(evt.Text)
			if text != "" {
				if memory.Intent == "" {
					memory.Intent = text
				}
				detail := ""
				if !evt.Timestamp.IsZero() {
					detail = fmt.Sprintf("Timestamp: %s", evt.Timestamp.UTC().Format(time.RFC3339))
				}
				memory.Evidence = append(memory.Evidence, MemoryEvidence{
					Kind:   "prompt",
					Target: text,
					Detail: detail,
				})
			}
		}
	}

	// 2. Extract Changes
	seenFiles := map[string]bool{}
	for _, evt := range session.Events {
		for _, clean := range evt.ModifiedFiles {
			if clean != "" && !seenFiles[clean] {
				seenFiles[clean] = true
				action := "modified"
				toolName := evt.ToolName
				if toolName == "" {
					toolName = string(evt.Type)
				}
				if strings.Contains(strings.ToLower(toolName), "create") || strings.Contains(strings.ToLower(toolName), "write") {
					action = "created/modified"
				}
				memory.Changes = append(memory.Changes, MemoryFileChange{
					Path:   clean,
					Action: action,
					Tool:   toolName,
				})
				memory.Evidence = append(memory.Evidence, MemoryEvidence{
					Kind:   "tool_trace",
					Target: clean,
					Detail: fmt.Sprintf("Tool: %s", toolName),
				})
			}
		}
	}

	// 3. Extract Explicit Decisions
	for _, evt := range session.Events {
		if evt.Role == "assistant" && evt.Text != "" {
			extractDecisionsFromText(evt.Text, &memory)
		}
	}

	// 4. Extract Problems & Errors
	for _, evt := range session.Events {
		if evt.Type == EventToolResult && (strings.Contains(evt.Text, "Error") || strings.Contains(evt.Text, "failed")) {
			memory.Problems = append(memory.Problems, MemoryProblem{
				Description: "Tool execution returned an error",
				Error:       strings.TrimSpace(evt.Text),
			})
		}
	}

	// 5. Extract Outcomes
	if !session.Partial {
		for _, evt := range session.Events {
			if evt.IsTaskComplete && strings.TrimSpace(evt.Text) != "" {
				memory.Outcomes = append(memory.Outcomes, strings.TrimSpace(evt.Text))
			}
		}
	}

	return memory
}

func extractDecisionsFromText(text string, memory *DevelopmentMemory) {
	matches := reDecidedBecause.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			statement := strings.TrimSpace(m[1])
			reason := strings.TrimSpace(m[2])
			if statement != "" && reason != "" {
				// Avoid duplicate decisions
				exists := false
				for _, d := range memory.Decisions {
					if d.Statement == statement {
						exists = true
						break
					}
				}
				if !exists {
					memory.Decisions = append(memory.Decisions, MemoryDecision{
						Statement: statement,
						Reason:    reason,
						Evidence:  []string{fmt.Sprintf("Explicit reasoning in transcript: %q", strings.TrimSpace(m[0]))},
					})
				}
			}
		}
	}

	onlyMatches := reDecisionOnly.FindAllStringSubmatch(text, -1)
	for _, m := range onlyMatches {
		if len(m) >= 2 {
			statement := strings.TrimSpace(m[1])
			if statement != "" {
				exists := false
				for _, d := range memory.Decisions {
					if d.Statement == statement {
						exists = true
						break
					}
				}
				if !exists {
					memory.Decisions = append(memory.Decisions, MemoryDecision{
						Statement: statement,
						Reason:    "", // Zero hallucination: no reason stated
						Evidence:  []string{fmt.Sprintf("Explicit decision in transcript: %q", strings.TrimSpace(m[0]))},
					})
				}
			}
		}
	}
}

// QueryWhy explains why a given file exists by searching development memories in repoRoot.
func QueryWhy(repoRoot, filePath string, stdout io.Writer) error {
	if repoRoot == "" {
		repoRoot = protocol.RepoRoot()
	}
	targetFile := cleanFile(filePath)
	if targetFile == "" {
		return fmt.Errorf("file path is required")
	}

	memories := loadAllMemories(repoRoot)
	var matchingMemories []DevelopmentMemory
	for _, m := range memories {
		for _, change := range m.Changes {
			if change.Path == targetFile || strings.HasSuffix(change.Path, targetFile) || strings.HasSuffix(targetFile, change.Path) {
				matchingMemories = append(matchingMemories, m)
				break
			}
		}
	}

	_, _ = fmt.Fprintf(stdout, "================================================================================\n")
	_, _ = fmt.Fprintf(stdout, "WHY THIS FILE EXISTS: %s\n", targetFile)
	_, _ = fmt.Fprintf(stdout, "================================================================================\n\n")

	if len(matchingMemories) == 0 {
		_, _ = fmt.Fprintf(stdout, "No explicit development memory captured for: %s\n", targetFile)
		_, _ = fmt.Fprintf(stdout, "Run 'entire enable --agent roo' and use Roo Code to track development history.\n")
		return nil
	}

	// Sort chronologically
	sort.Slice(matchingMemories, func(i, j int) bool {
		return matchingMemories[i].Timestamp < matchingMemories[j].Timestamp
	})

	latest := matchingMemories[len(matchingMemories)-1]

	_, _ = fmt.Fprintf(stdout, "Intent:\n")
	if latest.Intent != "" {
		_, _ = fmt.Fprintf(stdout, "  %s\n\n", latest.Intent)
	} else {
		_, _ = fmt.Fprintf(stdout, "  (No explicit intent recorded)\n\n")
	}

	_, _ = fmt.Fprintf(stdout, "Related Decisions:\n")
	hasDecisions := false
	for _, m := range matchingMemories {
		for _, d := range m.Decisions {
			hasDecisions = true
			if d.Reason != "" {
				_, _ = fmt.Fprintf(stdout, "  - %s (Reason: %s)\n", d.Statement, d.Reason)
			} else {
				_, _ = fmt.Fprintf(stdout, "  - %s\n", d.Statement)
			}
		}
	}
	if !hasDecisions {
		_, _ = fmt.Fprintf(stdout, "  No explicit historical reason was captured.\n")
	}
	_, _ = fmt.Fprintf(stdout, "\n")

	_, _ = fmt.Fprintf(stdout, "Evidence & Provenance:\n")
	for _, m := range matchingMemories {
		_, _ = fmt.Fprintf(stdout, "  - Roo Session: %s\n", m.SessionID)
		if m.CheckpointID != "" {
			_, _ = fmt.Fprintf(stdout, "    Entire Checkpoint: %s\n", m.CheckpointID)
		}
		for _, c := range m.Changes {
			if c.Path == targetFile || strings.HasSuffix(c.Path, targetFile) || strings.HasSuffix(targetFile, c.Path) {
				_, _ = fmt.Fprintf(stdout, "    Tool Action: %s via %s\n", c.Action, c.Tool)
			}
		}
	}
	_, _ = fmt.Fprintf(stdout, "\n")

	_, _ = fmt.Fprintf(stdout, "Modification History:\n")
	for i, m := range matchingMemories {
		stage := "Modified"
		if i == 0 {
			stage = "Created"
		}
		_, _ = fmt.Fprintf(stdout, "  [%s] %s in session %s (Intent: %q)\n", m.Timestamp, stage, m.SessionID, m.Intent)
	}

	return nil
}

// QueryHistory outputs chronological development history for a file.
func QueryHistory(repoRoot, filePath string, stdout io.Writer) error {
	if repoRoot == "" {
		repoRoot = protocol.RepoRoot()
	}
	targetFile := cleanFile(filePath)
	if targetFile == "" {
		return fmt.Errorf("file path is required")
	}

	memories := loadAllMemories(repoRoot)
	var matchingMemories []DevelopmentMemory
	for _, m := range memories {
		for _, change := range m.Changes {
			if change.Path == targetFile || strings.HasSuffix(change.Path, targetFile) || strings.HasSuffix(targetFile, change.Path) {
				matchingMemories = append(matchingMemories, m)
				break
			}
		}
	}

	_, _ = fmt.Fprintf(stdout, "================================================================================\n")
	_, _ = fmt.Fprintf(stdout, "DEVELOPMENT HISTORY: %s\n", targetFile)
	_, _ = fmt.Fprintf(stdout, "================================================================================\n\n")

	if len(matchingMemories) == 0 {
		_, _ = fmt.Fprintf(stdout, "No development history recorded for: %s\n", targetFile)
		return nil
	}

	sort.Slice(matchingMemories, func(i, j int) bool {
		return matchingMemories[i].Timestamp < matchingMemories[j].Timestamp
	})

	for idx, m := range matchingMemories {
		_, _ = fmt.Fprintf(stdout, "Turn #%d | Session %s | %s\n", idx+1, m.SessionID, m.Timestamp)
		_, _ = fmt.Fprintf(stdout, "  Intent:   %s\n", m.Intent)
		for _, d := range m.Decisions {
			if d.Reason != "" {
				_, _ = fmt.Fprintf(stdout, "  Decision: %s (Reason: %s)\n", d.Statement, d.Reason)
			} else {
				_, _ = fmt.Fprintf(stdout, "  Decision: %s\n", d.Statement)
			}
		}
		if len(m.Outcomes) > 0 {
			_, _ = fmt.Fprintf(stdout, "  Outcome:  %s\n", strings.Join(m.Outcomes, "; "))
		}
		_, _ = fmt.Fprintf(stdout, "--------------------------------------------------------------------------------\n")
	}

	return nil
}

func loadAllMemories(repoRoot string) []DevelopmentMemory {
	var memories []DevelopmentMemory

	// 1. Check .entire/tmp/roo/
	rooTmp := filepath.Join(repoRoot, ".entire", "tmp", "roo")
	if entries, err := os.ReadDir(rooTmp); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "memory-") {
				path := filepath.Join(rooTmp, e.Name())
				if data, err := os.ReadFile(path); err == nil {
					if session, err := LoadSession(data); err == nil {
						sessionID := strings.TrimSuffix(e.Name(), ".json")
						memories = append(memories, ExtractMemoryFromSession(session, sessionID, ""))
					}
				}
			}
		}
	}

	// 2. Check globalStorage tasks
	globalTasks := GetGlobalStorageTasksDir()
	if globalTasks != "" {
		if entries, err := os.ReadDir(globalTasks); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					taskID := e.Name()
					// Avoid duplicates if already loaded from .entire/tmp/roo
					alreadyLoaded := false
					for _, m := range memories {
						if m.SessionID == taskID {
							alreadyLoaded = true
							break
						}
					}
					if alreadyLoaded {
						continue
					}
					uiPath := filepath.Join(globalTasks, taskID, "ui_messages.json")
					apiPath := filepath.Join(globalTasks, taskID, "api_conversation_history.json")
					if uiData, err := os.ReadFile(uiPath); err == nil {
						var uiMessages []ClineMessage
						if json.Unmarshal(uiData, &uiMessages) == nil {
							var apiHistory []ApiMessage
							if apiData, err := os.ReadFile(apiPath); err == nil {
								_ = json.Unmarshal(apiData, &apiHistory)
							}
							env := RooTaskEnvelope{
								TaskID:                 taskID,
								UiMessages:             uiMessages,
								ApiConversationHistory: apiHistory,
							}
							memories = append(memories, ExtractMemory(env, taskID, ""))
						}
					}
				}
			}
		}
	}

	return memories
}
