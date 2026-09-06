package roo

import (
	"testing"
)

func TestLoadSession_OldFormat(t *testing.T) {
	data := []byte(`{
		"taskId": "test-task",
		"createdAt": 1690000000000,
		"uiMessages": [
			{"ts": 1690000000000, "type": "say", "say": "task", "text": "Do something"}
		]
	}`)
	session, err := LoadSession(data)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if session.SessionID != "test-task" {
		t.Errorf("Expected SessionID 'test-task', got '%s'", session.SessionID)
	}
	if len(session.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(session.Events))
	}
	if session.Events[0].Type != EventUserPrompt {
		t.Errorf("Expected EventUserPrompt, got %s", session.Events[0].Type)
	}
}

func TestLoadSession_NewFormat(t *testing.T) {
	data := []byte(`{"timestamp": "2026-09-06T09:00:00.000+05:30", "event": "session_started", "session_id": "track3-test"}
{"timestamp": "2026-09-06T09:00:08.214+05:30", "event": "user_prompt", "text": "Add coupon validation"}
{"timestamp": "2026-09-06T09:01:06.120+05:30", "event": "file_changed", "path": "src/checkout/apply_coupon.ts"}`)

	session, err := LoadSession(data)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if session.SessionID != "track3-test" {
		t.Errorf("Expected session ID 'track3-test', got '%s'", session.SessionID)
	}
	if len(session.Events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(session.Events))
	}
	if session.Events[0].Type != EventSessionStarted {
		t.Errorf("Expected EventSessionStarted, got %s", session.Events[0].Type)
	}
	if session.Events[1].Type != EventUserPrompt {
		t.Errorf("Expected EventUserPrompt, got %s", session.Events[1].Type)
	}
	if session.Events[2].Type != EventFileChanged {
		t.Errorf("Expected EventFileChanged, got %s", session.Events[2].Type)
	}
	if len(session.Events[2].ModifiedFiles) != 1 || session.Events[2].ModifiedFiles[0] != "src/checkout/apply_coupon.ts" {
		t.Errorf("Expected modified file 'src/checkout/apply_coupon.ts', got %v", session.Events[2].ModifiedFiles)
	}
}

func TestLoadSession_UnknownEvent(t *testing.T) {
	data := []byte(`{"timestamp": "2026-09-06T09:00:00.000+05:30", "event": "session_started", "session_id": "test-unknown"}
{"timestamp": "2026-09-06T09:00:08.214+05:30", "event": "some_future_event_type", "foo": "bar"}`)

	session, err := LoadSession(data)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if len(session.Events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(session.Events))
	}
	if session.Events[1].Type != EventUnknown {
		t.Errorf("Expected EventUnknown, got %s", session.Events[1].Type)
	}
}

func TestLoadSession_IncompleteTranscript(t *testing.T) {
	data := []byte(`{"timestamp": "2026-09-06T09:00:00.000+05:30", "event": "session_started", "session_id": "test-partial"}
{"timestamp": "2026-09-06T09:00:08.214+05:30", "event": "user_prompt", "text": "Start task"}
{"timest`)

	session, err := LoadSession(data)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if len(session.Events) != 2 {
		t.Fatalf("Expected 2 valid events before truncation, got %d", len(session.Events))
	}
	if !session.Partial {
		t.Errorf("Expected session.Partial to be true")
	}
}

func TestModifiedFilesExtraction(t *testing.T) {
	data := []byte(`{"timestamp": "2026-09-06T09:00:00.000+05:30", "event": "file_changed", "path": "a.txt"}
{"timestamp": "2026-09-06T09:00:00.000+05:30", "event": "file_changed", "path": "b.txt"}
{"timestamp": "2026-09-06T09:00:00.000+05:30", "event": "file_changed", "path": "a.txt"}`)

	session, _ := LoadSession(data)
	files := modifiedFilesFromSession(session, 0)
	if len(files) != 2 || files[0] != "a.txt" || files[1] != "b.txt" {
		t.Errorf("Expected [a.txt, b.txt], got %v", files)
	}
}

func TestCalculateTokens(t *testing.T) {
	data := []byte(`{"event": "usage", "input_tokens": 100, "output_tokens": 50}
{"event": "usage", "input_tokens": 200, "output_tokens": 10}`)

	session, _ := LoadSession(data)
	
	// Simulate what Agent.CalculateTokens does
	var inputTokens, outputTokens int
	for _, msg := range session.Events {
		if msg.Type == EventUsage {
			inputTokens += msg.InputTokens
			outputTokens += msg.OutputTokens
		}
	}
	
	if inputTokens != 300 {
		t.Errorf("Expected 300 input tokens, got %d", inputTokens)
	}
	if outputTokens != 60 {
		t.Errorf("Expected 60 output tokens, got %d", outputTokens)
	}
}
