package roo

import (
	"os"
	"os/exec"
	"strings"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

type Agent struct{}

func New() *Agent {
	return &Agent{}
}

func (a *Agent) Info() protocol.InfoResponse {
	return protocol.InfoResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		Name:            "roo",
		Type:            "Roo Code",
		Description:     "Roo Code AI coding assistant integration for Entire",
		IsPreview:       true,
		ProtectedDirs:   []string{".roo"},
		ProtectedFiles: []string{
			".roo/hooks.json",
			".roo/mcp.json",
			".roomodes",
			".roorules",
		},
		HookNames: []string{
			HookNameSessionStart,
			HookNameTurnStart,
			HookNameTurnEnd,
			HookNameCompaction,
			HookNameSessionEnd,
		},
		Capabilities: protocol.DeclaredCapabilities{
			Hooks:              true,
			TranscriptAnalyzer: true,
			TranscriptPreparer: true,
			TokenCalculator:    true,
			CompactTranscript:  true,
			UsesTerminal:       true,
		},
	}
}

func (a *Agent) Detect() protocol.DetectResponse {
	if _, err := exec.LookPath("roo"); err == nil {
		return protocol.DetectResponse{Present: true}
	}
	if _, err := exec.LookPath("roo-code"); err == nil {
		return protocol.DetectResponse{Present: true}
	}
	if tasksDir := GetGlobalStorageTasksDir(); tasksDir != "" {
		if fi, err := os.Stat(tasksDir); err == nil && fi.IsDir() {
			return protocol.DetectResponse{Present: true}
		}
	}
	return protocol.DetectResponse{Present: false}
}

func (a *Agent) GetSessionID(input *protocol.HookInputJSON) string {
	if input != nil && input.SessionID != "" {
		return input.SessionID
	}
	return ""
}

func (a *Agent) FormatResumeCommand(sessionID string) string {
	if sessionID == "" {
		return "roo --resume"
	}
	return "roo --resume " + shellQuote(sessionID)
}

func shellQuote(value string) string {
	for _, r := range value {
		if !isSafeResumeRune(r) {
			return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
		}
	}
	return value
}

func isSafeResumeRune(r rune) bool {
	return r == '-' || r == '_' || r == '.' || r == ':' || r == '/' ||
		(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
