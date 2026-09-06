package roo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleRooEnvelope() RooTaskEnvelope {
	return RooTaskEnvelope{
		TaskID:    "1712345678900",
		CreatedAt: 1712345678900,
		UpdatedAt: 1712345685000,
		UiMessages: []ClineMessage{
			{
				Ts:   1712345678900,
				Type: "say",
				Say:  "task",
				Text: "Refactor auth middleware to use JWT",
			},
			{
				Ts:   1712345680100,
				Type: "say",
				Say:  "tool",
				Text: `{"tool":"write_to_file","path":"src/auth/jwt.ts","content":"export function verify() {}"}`,
			},
			{
				Ts:   1712345681200,
				Type: "say",
				Say:  "tool",
				Text: `{"tool":"replace_in_file","path":"src/server.ts","diff":"-import auth\n+import jwt"}`,
			},
			{
				Ts:   1712345682000,
				Type: "say",
				Say:  "api_req_started",
				Text: `{"tokensIn":1250,"tokensOut":320,"cacheWrites":100,"cacheReads":500}`,
			},
			{
				Ts:   1712345685000,
				Type: "say",
				Say:  "completion_result",
				Text: "JWT auth middleware successfully implemented.",
			},
		},
		ApiConversationHistory: []ApiMessage{
			{
				Role: "user",
				Content: []ApiMessagePart{
					{Type: "text", Text: "Refactor auth middleware to use JWT"},
				},
			},
			{
				Role: "assistant",
				Content: []ApiMessagePart{
					{Type: "text", Text: "I will implement JWT auth."},
					{
						Type:  "tool_use",
						ID:    "tool_1",
						Name:  "write_to_file",
						Input: json.RawMessage(`{"path":"src/auth/jwt.ts","content":"..."}`),
					},
				},
			},
		},
	}
}

func writeTempEnvelope(t *testing.T, env RooTaskEnvelope) string {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, env.TaskID+".json")
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return filePath
}

func TestExtractModifiedFiles(t *testing.T) {
	agent := New()
	env := sampleRooEnvelope()
	filePath := writeTempEnvelope(t, env)

	files, pos, err := agent.ExtractModifiedFiles(filePath, 0)
	if err != nil {
		t.Fatalf("ExtractModifiedFiles failed: %v", err)
	}

	expectedFiles := []string{"src/auth/jwt.ts", "src/server.ts"}
	if !reflect.DeepEqual(files, expectedFiles) {
		t.Errorf("files = %v, want %v", files, expectedFiles)
	}
	if pos != len(env.UiMessages) {
		t.Errorf("pos = %d, want %d", pos, len(env.UiMessages))
	}
}

func TestExtractPrompts(t *testing.T) {
	agent := New()
	env := sampleRooEnvelope()
	filePath := writeTempEnvelope(t, env)

	prompts, err := agent.ExtractPrompts(filePath, 0)
	if err != nil {
		t.Fatalf("ExtractPrompts failed: %v", err)
	}

	expectedPrompts := []string{"Refactor auth middleware to use JWT"}
	if !reflect.DeepEqual(prompts, expectedPrompts) {
		t.Errorf("prompts = %v, want %v", prompts, expectedPrompts)
	}
}

func TestExtractSummary(t *testing.T) {
	agent := New()
	env := sampleRooEnvelope()
	filePath := writeTempEnvelope(t, env)

	summary, hasSummary, err := agent.ExtractSummary(filePath)
	if err != nil {
		t.Fatalf("ExtractSummary failed: %v", err)
	}
	if !hasSummary {
		t.Fatal("expected hasSummary = true")
	}

	expectedSummary := "JWT auth middleware successfully implemented."
	if summary != expectedSummary {
		t.Errorf("summary = %q, want %q", summary, expectedSummary)
	}
}

func TestCalculateTokens(t *testing.T) {
	agent := New()
	env := sampleRooEnvelope()
	data, _ := json.Marshal(env)

	usage, err := agent.CalculateTokens(data, 0)
	if err != nil {
		t.Fatalf("CalculateTokens failed: %v", err)
	}

	if usage.InputTokens != 1250 {
		t.Errorf("InputTokens = %d, want 1250", usage.InputTokens)
	}
	if usage.OutputTokens != 320 {
		t.Errorf("OutputTokens = %d, want 320", usage.OutputTokens)
	}
	if usage.CacheCreationTokens != 100 {
		t.Errorf("CacheCreationTokens = %d, want 100", usage.CacheCreationTokens)
	}
	if usage.CacheReadTokens != 500 {
		t.Errorf("CacheReadTokens = %d, want 500", usage.CacheReadTokens)
	}
	if usage.APICallCount != 1 {
		t.Errorf("APICallCount = %d, want 1", usage.APICallCount)
	}
}

func TestReadTranscriptAndChunking(t *testing.T) {
	agent := New()
	env := sampleRooEnvelope()
	filePath := writeTempEnvelope(t, env)

	data, err := agent.ReadTranscript(filePath)
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	chunks, err := agent.ChunkTranscript(data, 128)
	if err != nil {
		t.Fatalf("ChunkTranscript failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected non-empty chunks")
	}

	reassembled, err := agent.ReassembleTranscript(chunks)
	if err != nil {
		t.Fatalf("ReassembleTranscript failed: %v", err)
	}
	if string(reassembled) != string(data) {
		t.Fatal("reassembled bytes do not match original")
	}
}
