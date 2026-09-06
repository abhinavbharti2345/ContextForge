package roo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSessionDirAndResolveSessionFile(t *testing.T) {
	agent := New()
	repoPath := filepath.Join(os.TempDir(), "test-repo")

	dir, err := agent.GetSessionDir(repoPath)
	if err != nil {
		t.Fatalf("GetSessionDir failed: %v", err)
	}

	expectedDir := filepath.Join(repoPath, ".entire", "tmp", "roo")
	if dir != expectedDir {
		t.Errorf("GetSessionDir = %q, want %q", dir, expectedDir)
	}

	file := agent.ResolveSessionFile(dir, "task-123")
	expectedFile := filepath.Join(expectedDir, "task-123.json")
	if file != expectedFile {
		t.Errorf("ResolveSessionFile = %q, want %q", file, expectedFile)
	}
}

func TestGetGlobalStorageTasksDirCustom(t *testing.T) {
	customDir := filepath.Join(os.TempDir(), "custom-roo-tasks")
	t.Setenv("ROO_TASKS_DIR", customDir)

	resolved := GetGlobalStorageTasksDir()
	if resolved != customDir {
		t.Errorf("GetGlobalStorageTasksDir = %q, want %q", resolved, customDir)
	}
}

func TestSafeSessionID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1712345678900", "1712345678900"},
		{"task/123:abc", "task_123_abc"},
		{"", "unknown"},
		{"valid-task_id.1", "valid-task_id.1"},
	}

	for _, tt := range tests {
		got := safeSessionID(tt.input)
		if got != tt.want {
			t.Errorf("safeSessionID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
