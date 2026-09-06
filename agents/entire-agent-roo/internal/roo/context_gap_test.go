package roo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupMockGitRepo creates a temporary git repository for testing.
func setupMockGitRepo(t *testing.T) string {
	dir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")

	return dir
}

// createMockCommit creates a file, stages it, and commits it, returning the SHA.
func createMockCommit(t *testing.T, repoDir, fileName, fileContent, commitMsg string) string {
	filePath := filepath.Join(repoDir, fileName)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
		t.Fatal(err)
	}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("add", fileName)
	runGit("commit", "-m", commitMsg)
	return runGit("rev-parse", "HEAD")
}

func setupMockDevelopmentMemory(t *testing.T, repoDir string) {
	entireDir := filepath.Join(repoDir, ".entire", "tmp", "roo")
	if err := os.MkdirAll(entireDir, 0755); err != nil {
		t.Fatal(err)
	}

	// This is a minimal JSON representation of a NormalizedSession that loadAllMemories can parse
	// Wait, loadAllMemories parses AgentSessionJSON or NormalizedSession from .entire/tmp/roo/*.json
	// Actually, it uses LoadSession on the data. LoadSession expects either JSON or JSONL.
	// Let's create a minimal JSONL file simulating a session with a changed file.
	jsonl := `{"timestamp": "2026-09-06T09:00:00Z", "event": "session_started", "session_id": "test-session-1"}
{"timestamp": "2026-09-06T09:00:01Z", "event": "file_changed", "session_id": "test-session-1", "path": "src/checkout/apply_coupon.ts", "change": "modified", "summary": "Added validation"}
{"timestamp": "2026-09-06T09:00:02Z", "event": "agent_response", "session_id": "test-session-1", "text": "I decided to add validation because tests failed."}
{"timestamp": "2026-09-06T09:00:03Z", "event": "checkpoint_created", "session_id": "test-session-1", "checkpoint_id": "mock-cp-1", "git_commit": "abc", "summary": "done", "intent": "Add validation"}
{"timestamp": "2026-09-06T09:00:04Z", "event": "session_ended", "session_id": "test-session-1"}
`
	memFile := filepath.Join(entireDir, "test-session-1.json")
	if err := os.WriteFile(memFile, []byte(jsonl), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectContextGap_ObservedCommit(t *testing.T) {
	repoDir := setupMockGitRepo(t)
	sha := createMockCommit(t, repoDir, "src/checkout/apply_coupon.ts", "content", "Entire Checkpoint 123\n\n## Intent\nAdd validation")

	gap, err := DetectContextGap(repoDir, sha)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gap != nil {
		t.Fatalf("expected no context gap for an observed commit, got %+v", gap)
	}
}

func TestDetectContextGap_UnobservedCommit_WithHistory(t *testing.T) {
	repoDir := setupMockGitRepo(t)
	setupMockDevelopmentMemory(t, repoDir)
	
	// Create unobserved commit touching the exact same file as the mock memory
	sha := createMockCommit(t, repoDir, "src/checkout/apply_coupon.ts", "updated content", "Refactor coupon validation")

	gap, err := DetectContextGap(repoDir, sha)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gap == nil {
		t.Fatal("expected a context gap to be detected")
	}

	if gap.CommitMessage != "Refactor coupon validation" {
		t.Errorf("CommitMessage = %q, want 'Refactor coupon validation'", gap.CommitMessage)
	}
	if len(gap.ChangedFiles) != 1 || gap.ChangedFiles[0] != "src/checkout/apply_coupon.ts" {
		t.Errorf("ChangedFiles = %v", gap.ChangedFiles)
	}
	if len(gap.RelatedCheckpoints) != 1 || gap.RelatedCheckpoints[0] != "mock-cp-1" {
		t.Errorf("RelatedCheckpoints = %v", gap.RelatedCheckpoints)
	}
	if len(gap.RelatedDecisions) != 1 || gap.RelatedDecisions[0].Statement != "add validation" {
		t.Errorf("RelatedDecisions = %v", gap.RelatedDecisions)
	}
	if !strings.Contains(gap.MissingReason, "occurred outside an observed agent session") {
		t.Errorf("MissingReason = %q", gap.MissingReason)
	}
}

func TestDetectContextGap_UnobservedCommit_NoHistory(t *testing.T) {
	repoDir := setupMockGitRepo(t)
	setupMockDevelopmentMemory(t, repoDir)
	
	// Create unobserved commit touching a COMPLETELY DIFFERENT file
	sha := createMockCommit(t, repoDir, "src/unrelated/file.ts", "content", "Add unrelated file")

	gap, err := DetectContextGap(repoDir, sha)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gap == nil {
		t.Fatal("expected a context gap to be detected")
	}

	if len(gap.RelatedCheckpoints) != 0 {
		t.Errorf("Expected 0 related checkpoints, got %v", gap.RelatedCheckpoints)
	}
	if gap.MissingReason != "No explicit historical reason was captured." {
		t.Errorf("MissingReason = %q", gap.MissingReason)
	}
}
