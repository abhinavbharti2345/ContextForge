package roo

import (
	"testing"
)

func TestParseHookNoOp(t *testing.T) {
	agent := New()
	event, err := agent.ParseHook("turn-end", []byte(`{"session_id":"123"}`))
	if err != nil {
		t.Fatalf("ParseHook failed: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event, got %+v", event)
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

	if agent.AreHooksInstalled() {
		t.Fatal("expected AreHooksInstalled = false")
	}

	if err := agent.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks failed: %v", err)
	}
}
