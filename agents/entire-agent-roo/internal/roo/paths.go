package roo

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

const (
	transcriptSubdir = "roo"
	extensionDirName = "rooveterinaryinc.roo-cline"
)

func (a *Agent) GetSessionDir(repoPath string) (string, error) {
	return filepath.Join(protocol.DefaultSessionDir(repoPath), transcriptSubdir), nil
}

func (a *Agent) ResolveSessionFile(sessionDir, sessionID string) string {
	return filepath.Join(sessionDir, safeSessionID(sessionID)+".json")
}

func transcriptPath(sessionID string) string {
	return filepath.Join(protocol.DefaultSessionDir(protocol.RepoRoot()), transcriptSubdir, safeSessionID(sessionID)+".json")
}

func safeSessionID(sessionID string) string {
	if sessionID == "" {
		return "unknown"
	}
	var out []rune
	for _, r := range sessionID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

func GetGlobalStorageTasksDir() string {
	if custom := os.Getenv("ROO_TASKS_DIR"); custom != "" {
		return custom
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
				appData = filepath.Join(userProfile, "AppData", "Roaming")
			}
		}
		if appData != "" {
			return filepath.Join(appData, "Code", "User", "globalStorage", extensionDirName, "tasks")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", extensionDirName, "tasks")
		}
	default: // linux, bsd, etc.
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, ".config", "Code", "User", "globalStorage", extensionDirName, "tasks")
		}
	}
	return ""
}
