package roo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMemoryIntent(t *testing.T) {
	env := RooTaskEnvelope{
		TaskID: "task-intent-1",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Implement OAuth authentication flow"},
		},
	}

	memory := ExtractMemory(env, "task-intent-1", "cp-100")
	if memory.Intent != "Implement OAuth authentication flow" {
		t.Errorf("memory.Intent = %q, want 'Implement OAuth authentication flow'", memory.Intent)
	}
	if len(memory.Evidence) == 0 {
		t.Error("expected prompt evidence")
	}
}

func TestExtractMemoryExplicitDecisionAndReason(t *testing.T) {
	env := RooTaskEnvelope{
		TaskID: "task-decision-1",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Refactor session store"},
			{Ts: 105, Type: "say", Say: "text", Text: "We decided to separate OAuth callback from session creation because token validation requires independent retry logic."},
			{Ts: 110, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"src/auth/session.ts"}`},
		},
	}

	memory := ExtractMemory(env, "task-decision-1", "cp-101")
	if len(memory.Decisions) == 0 {
		t.Fatal("expected at least 1 decision extracted")
	}

	d := memory.Decisions[0]
	if !strings.Contains(d.Statement, "separate OAuth callback from session creation") {
		t.Errorf("decision statement = %q, want substring 'separate OAuth callback from session creation'", d.Statement)
	}
	if !strings.Contains(d.Reason, "token validation requires independent retry logic") {
		t.Errorf("decision reason = %q, want substring 'token validation requires independent retry logic'", d.Reason)
	}
}

func TestExtractMemoryMissingReasonNoHallucination(t *testing.T) {
	env := RooTaskEnvelope{
		TaskID: "task-no-reason",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Fix button styling"},
			{Ts: 105, Type: "say", Say: "text", Text: "Updated CSS rules. Everything looks good."},
			{Ts: 110, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"src/styles/button.css"}`},
		},
	}

	memory := ExtractMemory(env, "task-no-reason", "cp-102")
	if len(memory.Decisions) != 0 {
		t.Fatalf("expected 0 decisions when no explicit reasoning is stated, got %d decisions: %+v", len(memory.Decisions), memory.Decisions)
	}
}

func TestFileToMemoryAssociation(t *testing.T) {
	env := RooTaskEnvelope{
		TaskID: "task-assoc-1",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Add jwt token validator"},
			{Ts: 105, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"src/auth/jwt.ts"}`},
			{Ts: 110, Type: "say", Say: "completion_result", Text: "JWT validator implemented."},
		},
	}

	memory := ExtractMemory(env, "task-assoc-1", "cp-103")
	if len(memory.Changes) != 1 {
		t.Fatalf("len(memory.Changes) = %d, want 1", len(memory.Changes))
	}
	if memory.Changes[0].Path != "src/auth/jwt.ts" {
		t.Errorf("change path = %q, want 'src/auth/jwt.ts'", memory.Changes[0].Path)
	}
}

func TestMultipleCheckpointsSameFileQuery(t *testing.T) {
	var buf bytes.Buffer
	tempRepo := t.TempDir()

	env1 := RooTaskEnvelope{
		TaskID: "session-1",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Create database schema"},
			{Ts: 105, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"src/db/schema.sql"}`},
			{Ts: 110, Type: "say", Say: "completion_result", Text: "Created schema."},
		},
	}
	agent := New()
	sessionRef1 := agent.ResolveSessionFile(tempRepo+"/.entire/tmp/roo", "session-1")
	_ = osMkdirAllWrite(sessionRef1, env1)

	env2 := RooTaskEnvelope{
		TaskID: "session-2",
		UiMessages: []ClineMessage{
			{Ts: 200, Type: "say", Say: "task", Text: "Add indexes to schema"},
			{Ts: 205, Type: "say", Say: "text", Text: "We decided to add compound index on user_id and created_at because query profile showed high latency on range scans."},
			{Ts: 210, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"src/db/schema.sql"}`},
			{Ts: 215, Type: "say", Say: "completion_result", Text: "Added index."},
		},
	}
	sessionRef2 := agent.ResolveSessionFile(tempRepo+"/.entire/tmp/roo", "session-2")
	_ = osMkdirAllWrite(sessionRef2, env2)

	err := QueryWhy(tempRepo, "src/db/schema.sql", &buf)
	if err != nil {
		t.Fatalf("QueryWhy failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "WHY THIS FILE EXISTS: src/db/schema.sql") {
		t.Error("missing header in QueryWhy output")
	}
	if !strings.Contains(out, "session-1") || !strings.Contains(out, "session-2") {
		t.Error("expected both sessions mentioned in history")
	}
	if !strings.Contains(out, "compound index on user_id and created_at") {
		t.Error("expected extracted decision in output")
	}
}

func TestQueryWhyUnknownFile(t *testing.T) {
	var buf bytes.Buffer
	tempRepo := t.TempDir()

	err := QueryWhy(tempRepo, "nonexistent/file.go", &buf)
	if err != nil {
		t.Fatalf("QueryWhy failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No explicit development memory captured for: nonexistent/file.go") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestQueryHistory(t *testing.T) {
	var buf bytes.Buffer
	tempRepo := t.TempDir()

	env := RooTaskEnvelope{
		TaskID: "hist-task-1",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Setup telemetry pipeline"},
			{Ts: 105, Type: "say", Say: "tool", Text: `{"tool":"write_to_file","path":"pkg/telemetry/tracer.go"}`},
			{Ts: 110, Type: "say", Say: "completion_result", Text: "Tracer initialized."},
		},
	}
	agent := New()
	sessionRef := agent.ResolveSessionFile(tempRepo+"/.entire/tmp/roo", "hist-task-1")
	_ = osMkdirAllWrite(sessionRef, env)

	err := QueryHistory(tempRepo, "pkg/telemetry/tracer.go", &buf)
	if err != nil {
		t.Fatalf("QueryHistory failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "DEVELOPMENT HISTORY: pkg/telemetry/tracer.go") {
		t.Error("missing header in QueryHistory output")
	}
	if !strings.Contains(out, "Setup telemetry pipeline") {
		t.Error("missing intent in QueryHistory output")
	}
}

func TestReadOnlyTaskProducesNoFalseChanges(t *testing.T) {
	env := RooTaskEnvelope{
		TaskID: "readonly-task",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Explain what auth.py does"},
			{Ts: 105, Type: "say", Say: "text", Text: "auth.py validates user tokens and returns session payloads."},
		},
	}

	memory := ExtractMemory(env, "readonly-task", "cp-read")
	if len(memory.Changes) != 0 {
		t.Fatalf("expected 0 file changes for read-only task, got %d", len(memory.Changes))
	}
}

func TestMalformedPartialTranscript(t *testing.T) {
	env := RooTaskEnvelope{
		TaskID: "partial-task",
		UiMessages: []ClineMessage{
			{Ts: 100, Type: "say", Say: "task", Text: "Generate parser"},
			{Ts: 105, Type: "say", Say: "text", Text: "Generating parser...", Partial: true},
		},
	}

	memory := ExtractMemory(env, "partial-task", "")
	if memory.Intent != "Generate parser" {
		t.Errorf("memory.Intent = %q, want 'Generate parser'", memory.Intent)
	}
	if len(memory.Changes) != 0 {
		t.Errorf("expected 0 file changes during partial streaming")
	}
}

func osMkdirAllWrite(path string, env RooTaskEnvelope) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0o644)
}
