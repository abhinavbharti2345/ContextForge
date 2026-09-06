package roo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type MockHookEmitter struct {
	mu     sync.Mutex
	Events []struct {
		HookName string
		Payload  RooHookPayload
	}
}

func (m *MockHookEmitter) Emit(hookName string, payload RooHookPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Events = append(m.Events, struct {
		HookName string
		Payload  RooHookPayload
	}{HookName: hookName, Payload: payload})
	return nil
}

func (m *MockHookEmitter) GetEvents() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, e := range m.Events {
		out = append(out, e.HookName)
	}
	return out
}

func (m *MockHookEmitter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Events = nil
}

func TestStreamingMessageDoesNotTriggerTurnEnd(t *testing.T) {
	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{Emitter: emitter})

	uiMessages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Write a server"},
		{Ts: 105, Type: "say", Say: "text", Text: "I am currently generating...", Partial: true},
	}

	err := watcher.ProcessTask("task-1", uiMessages, nil)
	if err != nil {
		t.Fatalf("ProcessTask failed: %v", err)
	}

	events := emitter.GetEvents()
	// Should emit session-start and turn-start, but NOT turn-end
	if len(events) != 2 || events[0] != "session-start" || events[1] != "turn-start" {
		t.Errorf("events = %v, want [session-start, turn-start]", events)
	}
}

func TestToolExecutionInFlight(t *testing.T) {
	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{Emitter: emitter})

	uiMessages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Edit config"},
		{Ts: 105, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"config.json"}`},
		{Ts: 110, Type: "say", Say: "api_req_started", Text: `{"tokensIn":100,"tokensOut":50}`},
	}

	err := watcher.ProcessTask("task-1", uiMessages, nil)
	if err != nil {
		t.Fatalf("ProcessTask failed: %v", err)
	}

	for _, e := range emitter.GetEvents() {
		if e == "turn-end" {
			t.Fatal("unexpected turn-end event during in-flight tool execution")
		}
	}
}

func TestToolApprovalWaiting(t *testing.T) {
	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{Emitter: emitter})

	uiMessages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Deploy"},
		{Ts: 105, Type: "ask", Ask: "command", Text: "Run rm -rf dist?"},
	}

	err := watcher.ProcessTask("task-1", uiMessages, nil)
	if err != nil {
		t.Fatalf("ProcessTask failed: %v", err)
	}

	for _, e := range emitter.GetEvents() {
		if e == "turn-end" {
			t.Fatal("unexpected turn-end event while waiting for user approval")
		}
	}
}

func TestCompletedTurnAndSessionEnd(t *testing.T) {
	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{Emitter: emitter})

	uiMessages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Add tests"},
		{Ts: 105, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"app_test.go"}`},
		{Ts: 110, Type: "say", Say: "completion_result", Text: "Tests created successfully."},
	}
	apiHistory := []ApiMessage{
		{Role: "user", Content: []ApiMessagePart{{Type: "text", Text: "Add tests"}}},
		{Role: "assistant", Content: []ApiMessagePart{{Type: "text", Text: "Done"}}},
	}

	err := watcher.ProcessTask("task-1", uiMessages, apiHistory)
	if err != nil {
		t.Fatalf("ProcessTask failed: %v", err)
	}

	events := emitter.GetEvents()
	expected := []string{"session-start", "turn-start", "turn-end", "session-end"}
	if len(events) != len(expected) {
		t.Fatalf("events = %v, want %v", events, expected)
	}
	for i := range expected {
		if events[i] != expected[i] {
			t.Errorf("event[%d] = %s, want %s", i, events[i], expected[i])
		}
	}
}

func TestDeduplicationOnRepeatedWrites(t *testing.T) {
	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{Emitter: emitter})

	uiMessages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Refactor auth"},
		{Ts: 105, Type: "say", Say: "text", Text: "Refactored successfully.", Partial: false},
	}
	apiHistory := []ApiMessage{
		{Role: "assistant", Content: []ApiMessagePart{{Type: "text", Text: "Done"}}},
	}

	_ = watcher.ProcessTask("task-1", uiMessages, apiHistory)
	firstCount := len(emitter.GetEvents())

	// Simulate repeated file-system writes of identical content
	_ = watcher.ProcessTask("task-1", uiMessages, apiHistory)
	_ = watcher.ProcessTask("task-1", uiMessages, apiHistory)

	secondCount := len(emitter.GetEvents())
	if firstCount != secondCount {
		t.Fatalf("duplicate events emitted: firstCount=%d, secondCount=%d (events: %v)", firstCount, secondCount, emitter.GetEvents())
	}
}

func TestMultipleSimultaneousTasks(t *testing.T) {
	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{Emitter: emitter})

	task1Messages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Task 1: Bug fix"},
		{Ts: 105, Type: "say", Say: "text", Text: "Bug fixed."},
	}
	task2Messages := []ClineMessage{
		{Ts: 200, Type: "say", Say: "task", Text: "Task 2: New feature"},
		{Ts: 205, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"feature.go"}`},
	}

	_ = watcher.ProcessTask("task-1", task1Messages, nil)
	_ = watcher.ProcessTask("task-2", task2Messages, nil)

	events := emitter.Events
	var task1Events, task2Events []string
	for _, e := range events {
		if e.Payload.TaskID == "task-1" {
			task1Events = append(task1Events, e.HookName)
		} else if e.Payload.TaskID == "task-2" {
			task2Events = append(task2Events, e.HookName)
		}
	}

	// Task 1 should have finished turn-end
	if len(task1Events) != 3 || task1Events[0] != "session-start" || task1Events[1] != "turn-start" || task1Events[2] != "turn-end" {
		t.Errorf("task1Events = %v, want [session-start, turn-start, turn-end]", task1Events)
	}

	// Task 2 should still be in-flight (no turn-end)
	if len(task2Events) != 2 || task2Events[0] != "session-start" || task2Events[1] != "turn-start" {
		t.Errorf("task2Events = %v, want [session-start, turn-start]", task2Events)
	}
}

func TestBootstrapExistingTasksDoesNotEmitHistoricalEvents(t *testing.T) {
	tempTasksDir := t.TempDir()
	task1Dir := filepath.Join(tempTasksDir, "task-historical")
	if err := os.MkdirAll(task1Dir, 0o755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	uiMessages := []ClineMessage{
		{Ts: 100, Type: "say", Say: "task", Text: "Old completed task"},
		{Ts: 105, Type: "say", Say: "completion_result", Text: "Old work complete."},
	}
	uiBytes, _ := json.Marshal(uiMessages)
	if err := os.WriteFile(filepath.Join(task1Dir, "ui_messages.json"), uiBytes, 0o644); err != nil {
		t.Fatalf("failed to write ui_messages.json: %v", err)
	}

	emitter := &MockHookEmitter{}
	watcher := NewWatcher(WatcherOptions{
		TasksDir: tempTasksDir,
		Emitter:  emitter,
	})

	// Run ScanOnce simulating regular watch cycle after restart
	watcher.ScanOnce()

	// Should not emit any events for pre-existing settled task
	events := emitter.GetEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for historical tasks on startup, got %v", events)
	}
}

