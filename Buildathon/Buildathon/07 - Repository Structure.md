---
title: 07 - Repository Structure
tags:
  - buildathon
  - repo-structure
  - golang
  - monorepo
  - mise
date: 2026-09-06
status: active
---

# 07 - Repository Structure

> [!IMPORTANT]
> **Key Correction Made During Planning:** We initially considered creating an independent repository named `ContextForge`. We corrected this after discovering that Entire mandates all external agent integrations reside directly within the `entireio/external-agents` monorepo.

---

## 1. Monorepo Overview (`entireio/external-agents`)

The official `entireio/external-agents` repository is structured as a Go-based monorepo where each external agent adapter lives in its own subdirectory under `agents/`.

```
entireio/external-agents/
├── .claude/
├── .codex/
├── .cursor/
├── .opencode/
├── agents/
│   ├── entire-agent-amp/
│   ├── entire-agent-goose/
│   ├── entire-agent-grok/
│   ├── entire-agent-kilo/
│   ├── entire-agent-kiro/
│   ├── entire-agent-omp/
│   ├── entire-agent-qwen/
│   └── entire-agent-<target>/      <-- OUR HACKATHON DELIVERABLE
├── scripts/
├── README.md
├── go.work / go.mod
└── mise.toml
```

---

## 2. Target Adapter Directory Tree (`agents/entire-agent-<target>/`)

Our specific adapter project follows the canonical layout required by Entire:

```
agents/entire-agent-<target>/
├── AGENT.md                          # Metadata describing agent features & protocol support
├── README.md                         # Quickstart, installation, and architecture doc
├── go.mod                            # Go module definition for this adapter
├── go.sum                            # Dependency checksums
├── mise.toml                         # Mise task definitions (build, test, lint, run)
│
├── cmd/
│   └── entire-agent-<target>/
│       └── main.go                   # CLI entrypoint; called by Entire CLI
│
├── internal/
│   ├── <target>/                     # Target agent translation & interceptor logic
│   │   ├── agent.go                  # Agent lifecycle manager and state machine
│   │   ├── hooks.go                  # Interceptors for prompts, tool calls, and diffs
│   │   ├── transcript.go             # Parser converting agent logs to Entire transcripts
│   │   └── types.go                  # Internal models for target agent data structures
│   │
│   └── protocol/                     # Entire wire protocol encoder/decoder
│       ├── protocol.go               # Protocol serialization & IPC communication
│       └── types.go                  # Standard Entire event structures (Session, Turn, etc.)
│
├── tests/
│   ├── unit/                         # Unit tests for parsers, types, and normalizers
│   │   ├── hooks_test.go
│   │   └── transcript_test.go
│   └── e2e/                          # End-to-end integration tests
│       └── lifecycle_test.go
│
└── scripts/
    ├── setup.sh                      # Environment setup & dependency installer
    └── demo.sh                       # Automated scripted demo run
```

---

## 3. Detailed Component & File Responsibilities

### 1. Root & Configuration Files
* **`AGENT.md`**: Machine-readable and human-readable metadata declaring what capabilities the agent supports (e.g., transcripts: true, checkpoints: true, tool_calls: true).
* **`README.md`**: Project documentation, installation instructions, usage guide, and demo walkthrough.
* **`mise.toml`**: Standardized task runner configuration used by `mise` (or `make`) to build and test the agent:
  ```toml
  [tasks.build]
  description = "Build the adapter binary"
  run = "go build -o bin/entire-agent-<target> ./cmd/entire-agent-<target>"

  [tasks.test]
  description = "Run all unit and integration tests"
  run = "go test -v ./..."

  [tasks.lint]
  description = "Run linter"
  run = "golangci-lint run"
  ```

### 2. Binary Entrypoint (`cmd/entire-agent-<target>/main.go`)
* Serves as the binary executed by the user or Entire CLI.
* Parses CLI flags (e.g. `--session-id`, `--working-dir`, `--entire-socket`).
* Initializes the internal agent manager and starts event loops.

### 3. Agent Translation Package (`internal/<target>/`)
* **`agent.go`**: High-level coordinator managing the agent lifecycle state.
* **`hooks.go`**: Connects to the target agent runtime (sub-process pipes, filesystem watchers, or socket listeners) to intercept activities.
* **`transcript.go`**: Parses messy agent outputs/logs into structured Entire transcript models.
* **`types.go`**: Go structs mirroring target agent configuration and event payloads.

### 4. Protocol Package (`internal/protocol/`)
* **`protocol.go`**: Low-level encoder/decoder that serializes normalized events into Entire's wire format.
* **`types.go`**: Entire-standard event types (`SessionStartEvent`, `TurnEvent`, `CheckpointEvent`, `ToolCallEvent`).

---

## 4. Fact vs Decision vs Assumption

### Confirmed Facts
* `entireio/external-agents` uses Go with separate directories per agent under `agents/`.
* `mise.toml` is used for task orchestration and CI discovery.
* Standalone binary compilation is required.

### Decisions Made
* Strictly adhere to this Go package layout.
* Keep `internal/protocol/` clean and isolated so protocol upgrades do not affect agent parsing logic.

### Assumptions
* The Go module in each agent directory can either share a `go.work` workspace or declare its own `go.mod`.

### Things to Verify
* Check if common protocol types can be imported from a shared `pkg/` directory in the monorepo root or if each agent maintains a local copy.

---

## Related Notes
* [[03 - What We Have To Build]] — Functional boundaries.
* [[06 - External Agent Integration]] — External agent protocol specifics.
* [[08 - Development Workflow]] — Development plan.
* [[09 - Team Responsibilities]] — Work division across the repository.
