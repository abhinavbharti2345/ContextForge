package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/roo"
)

func main() {
	data, err := os.ReadFile(`c:\CS\CS Projects\external-agents\track-3-agent-session.jsonl`)
	if err != nil {
		panic(err)
	}

	session, err := roo.LoadSession(data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Session ID: %s\n", session.SessionID)
	fmt.Printf("Events Count: %d\n", len(session.Events))

	mem := roo.ExtractMemoryFromSession(session, "btw-track3-demo-001", "cp-123")
	fmt.Printf("Intent: %s\n", mem.Intent)
	
	b, _ := json.MarshalIndent(mem, "", "  ")
	fmt.Printf("Memory:\n%s\n", string(b))
}
