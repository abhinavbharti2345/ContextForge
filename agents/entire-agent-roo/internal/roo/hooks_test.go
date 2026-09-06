package roo

import (
	"testing"
)

func TestParseHookSessionStart(t *testing.T) {
	agent := New()
	payload := []byte(`{"session_id":"task-123","event":"SessionStart"}`)

	event, err := agent.ParseHook(HookNameSessionStart, payload)
	if err != nil {
		t.Fatalf("ParseHook failed: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Type != 1 {
		t.Errorf("event.Type = %d, want 1", event.Type)
	}
	if event.SessionID != "task-123" {
		t.Errorf("event.SessionID = %q, want task-123", event.SessionID)
	}
	if event.SessionRef == "" {
		t.Error("expected non-empty SessionRef")
	}
}

func TestParseHookTurnStart(t *testing.T) {
	agent := New()
	payload := []byte(`{"session_id":"task-123","prompt":"Refactor database queries"}`)

	event, err := agent.ParseHook(HookNameTurnStart, payload)
	if err != nil {
		t.Fatalf("ParseHook failed: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Type != 2 {
		t.Errorf("event.Type = %d, want 2", event.Type)
	}
	if event.Prompt != "Refactor database queries" {
		t.Errorf("event.Prompt = %q, want 'Refactor database queries'", event.Prompt)
	}
}

func TestParseHookTurnEnd(t *testing.T) {
	agent := New()
	payload := []byte(`{"session_id":"task-123"}`)

	event, err := agent.ParseHook(HookNameTurnEnd, payload)
	if err != nil {
		t.Fatalf("ParseHook failed: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Type != 3 {
		t.Errorf("event.Type = %d, want 3", event.Type)
	}
	if event.SessionRef == "" {
		t.Error("expected non-empty SessionRef for turn-end")
	}
}

func TestParseHookSessionEnd(t *testing.T) {
	agent := New()
	payload := []byte(`{"session_id":"task-123"}`)

	event, err := agent.ParseHook(HookNameSessionEnd, payload)
	if err != nil {
		t.Fatalf("ParseHook failed: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Type != 5 {
		t.Errorf("event.Type = %d, want 5", event.Type)
	}
}

func TestInstallAndUninstallHooksNoOp(t *testing.T) {
	agent := New()

	installed, err := agent.InstallHooks(false, true)
	if err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}
	if installed != 0 {
		t.Errorf("installed = %d, want 0", installed)
	}

	if !agent.AreHooksInstalled() {
		t.Fatal("expected AreHooksInstalled = true")
	}

	if err := agent.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks failed: %v", err)
	}
}

