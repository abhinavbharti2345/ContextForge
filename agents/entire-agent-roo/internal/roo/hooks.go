package roo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

const (
	HookNameSessionStart = "session-start"
	HookNameTurnStart    = "turn-start"
	HookNameTurnEnd      = "turn-end"
	HookNameSessionEnd   = "session-end"
)

type RooHookPayload struct {
	Event      string          `json:"event,omitempty"`
	HookEvent  string          `json:"hook_event_name,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
	TaskID     string          `json:"taskId,omitempty"`
	SessionRef string          `json:"session_ref,omitempty"`
	Message    string          `json:"message,omitempty"`
	Prompt     string          `json:"prompt,omitempty"`
	UserPrompt string          `json:"user_prompt,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	WorkingDir string          `json:"cwd,omitempty"`
}

// ParseHook converts a hook payload (emitted by the storage watcher or protocol tests) into typed protocol.EventJSON.
func (a *Agent) ParseHook(hookName string, input []byte) (*protocol.EventJSON, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, nil
	}

	var payload RooHookPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, fmt.Errorf("parse hook payload: %w", err)
	}

	sessionID := payload.SessionID
	if sessionID == "" {
		sessionID = payload.TaskID
	}
	if sessionID == "" {
		return nil, nil
	}

	sessionRef := payload.SessionRef
	if sessionRef == "" {
		sessionRef = transcriptPath(sessionID)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	switch hookName {
	case HookNameSessionStart, "SessionStart", "session_start":
		if err := a.PrepareTranscript(sessionRef); err != nil {
			return nil, fmt.Errorf("prepare transcript on session-start: %w", err)
		}
		return &protocol.EventJSON{
			Type:       1,
			SessionID:  sessionID,
			SessionRef: sessionRef,
			Timestamp:  now,
		}, nil

	case HookNameTurnStart, "UserPromptSubmit", "turn_start":
		prompt := payload.Prompt
		if prompt == "" {
			prompt = payload.UserPrompt
		}
		if prompt == "" {
			prompt = payload.Message
		}
		return &protocol.EventJSON{
			Type:       2,
			SessionID:  sessionID,
			SessionRef: sessionRef,
			Prompt:     prompt,
			Timestamp:  now,
		}, nil

	case HookNameTurnEnd, "Stop", "turn_end":
		if err := a.PrepareTranscript(sessionRef); err != nil {
			return nil, fmt.Errorf("prepare transcript on turn-end: %w", err)
		}
		return &protocol.EventJSON{
			Type:       3,
			SessionID:  sessionID,
			SessionRef: sessionRef,
			Timestamp:  now,
		}, nil

	case HookNameSessionEnd, "SessionEnd", "session_end":
		if err := a.PrepareTranscript(sessionRef); err != nil {
			return nil, fmt.Errorf("prepare transcript on session-end: %w", err)
		}
		return &protocol.EventJSON{
			Type:       5,
			SessionID:  sessionID,
			SessionRef: sessionRef,
			Timestamp:  now,
		}, nil

	default:
		return nil, nil
	}
}

func (a *Agent) InstallHooks(localDev bool, force bool) (int, error) {
	// Roo Code storage-driven integration does not write fake .roo/hooks.json files.
	return 0, nil
}

func (a *Agent) UninstallHooks() error {
	return nil
}

func (a *Agent) AreHooksInstalled() bool {
	return true
}

