package roo

import (
	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

// Roo Code does not provide a native command hook dispatcher in its extension codebase.
// Lifecycle events are tracked via task storage ingestion (globalStorage tasks) rather than native hooks.

func (a *Agent) ParseHook(hookName string, input []byte) (*protocol.EventJSON, error) {
	// Roo Code has no native command hooks. Return nil gracefully.
	return nil, nil
}

func (a *Agent) InstallHooks(localDev bool, force bool) (int, error) {
	// No-op: Roo Code does not support native command hooks.
	return 0, nil
}

func (a *Agent) UninstallHooks() error {
	// No-op: Roo Code does not support native command hooks.
	return nil
}

func (a *Agent) AreHooksInstalled() bool {
	// Roo Code does not support native command hooks.
	return false
}
