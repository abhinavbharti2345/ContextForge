package roo

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventUserPrompt      EventType = "user_prompt"
	EventAgentResponse   EventType = "agent_response"
	EventToolCall        EventType = "tool_call"
	EventToolResult      EventType = "tool_result"
	EventFileChanged     EventType = "file_changed"
	EventFileRead        EventType = "file_read"
	EventUsage           EventType = "usage"
	EventSessionStarted  EventType = "session_started"
	EventSessionEnded    EventType = "session_ended"
	EventCheckpoint      EventType = "checkpoint_created"
	EventUnknown         EventType = "unknown"
)

type NormalizedEvent struct {
	Timestamp      time.Time
	Type           EventType
	Raw            json.RawMessage
	Role           string // "user" or "assistant"
	Text           string
	ToolName       string
	ToolInput      json.RawMessage
	ModifiedFiles  []string // Derived from tool input or file_changed event
	InputTokens    int
	OutputTokens   int
	IsTurnComplete bool // Flag indicating if this event completes a turn
	IsTaskComplete bool // Flag indicating if this event completes a task
}

type NormalizedSession struct {
	SessionID string
	Partial   bool
	Events    []NormalizedEvent
}

type ClineMessage struct {
	Ts                       int64    `json:"ts"`
	Type                     string   `json:"type"` // "say" or "ask"
	Say                      string   `json:"say,omitempty"`
	Ask                      string   `json:"ask,omitempty"`
	Text                     string   `json:"text,omitempty"`
	Partial                  bool     `json:"partial,omitempty"`
	Images                   []string `json:"images,omitempty"`
	ConversationHistoryIndex *int     `json:"conversationHistoryIndex,omitempty"`
}

func (m ClineMessage) Time() time.Time {
	if m.Ts == 0 {
		return time.Time{}
	}
	return time.UnixMilli(m.Ts).UTC()
}

type ApiMessage struct {
	Role    string           `json:"role"` // "user" or "assistant"
	Content []ApiMessagePart `json:"content"`
	Ts      int64            `json:"ts,omitempty"`
}

type ApiMessagePart struct {
	Type      string          `json:"type"` // "text", "tool_use", "tool_result"
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"` // tool name: "write_to_file", "replace_in_file", "execute_command", etc.
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type ApiReqStartedData struct {
	TokensIn    int     `json:"tokensIn"`
	TokensOut   int     `json:"tokensOut"`
	CacheWrites int     `json:"cacheWrites"`
	CacheReads  int     `json:"cacheReads"`
	TotalCost   float64 `json:"totalCost,omitempty"`
}

type RooTaskEnvelope struct {
	TaskID                 string         `json:"taskId"`
	CreatedAt              int64          `json:"createdAt"`
	UpdatedAt              int64          `json:"updatedAt"`
	UiMessages             []ClineMessage `json:"uiMessages"`
	ApiConversationHistory []ApiMessage   `json:"apiConversationHistory,omitempty"`
}
