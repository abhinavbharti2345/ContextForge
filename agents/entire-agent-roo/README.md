# entire-agent-roo

Entire external agent integration binary for [Roo Code](https://github.com/RooCodeInc/Roo-Code).

## Overview

`entire-agent-roo` connects Roo Code (VS Code Extension & CLI) with the Entire Git-native checkpoint system. Because Roo Code persists all interactions into its local `globalStorage` task directory without native command hooks, `entire-agent-roo` implements a **storage-driven external agent** architecture:

- `entire-agent-roo watch`: Background watcher monitoring active tasks in VS Code `globalStorage` and dispatching `turn-start` and `turn-end` events to the Entire CLI on turn completion.
- Discovering Roo Code global task storage (`ui_messages.json` + `api_conversation_history.json`).
- Extracting modified files from mutating tool calls (`write_to_file`, `replace_in_file`, `edit_file`, etc.).
- Extracting prompts and assistant summaries.
- Calculating token usages (`tokensIn`, `tokensOut`, `cacheReads`, `cacheWrites`).
- Formatting resume commands (`roo --resume <taskId>`).

## Usage

```bash
# Run storage watcher in background
entire-agent-roo watch

# Or with custom tasks storage directory
entire-agent-roo watch --tasks-dir /path/to/tasks --poll-interval 250 --debounce 500
```

## Protocol Commands

| Subcommand | Description |
| :--- | :--- |
| `info` | Returns agent metadata and declared capabilities (`hooks: true`) |
| `detect` | Checks for presence of `roo` binary or task storage |
| `get-session-id` | Extracts session/task ID |
| `get-session-dir` | Returns session directory (`.entire/tmp/roo`) |
| `resolve-session-file` | Resolves session JSON path |
| `read-session` | Reads session envelope and modified files |
| `write-session` | Persists session snapshot |
| `read-transcript` | Returns raw transcript bytes |
| `chunk-transcript` | Splits transcript into byte chunks |
| `reassemble-transcript`| Reassembles transcript chunks |
| `compact-transcript` | Produces Entire Compact Transcript JSONL |
| `prepare-transcript` | Ingests live task files from VS Code globalStorage |
| `format-resume-command`| Formats task resume shell command (`code`) |
| `parse-hook` | Parses watcher payloads for `session-start`, `turn-start`, `turn-end`, `session-end` |
| `install-hooks` | No-op (returns 0, nil) |
| `uninstall-hooks` | No-op (returns nil) |
| `are-hooks-installed` | Returns true (handled by watcher sidecar) |
| `get-transcript-position`| Returns message count |
| `extract-modified-files` | Parses mutating tool calls |
| `extract-prompts` | Extracts user prompts |
| `extract-summary` | Extracts assistant response summary |
| `calculate-tokens` | Aggregates token metrics |
| `watch` | Runs the storage-driven lifecycle watcher sidecar |
| `why` | Queries development memory: intent, decisions, and evidence for a file |
| `history` | Queries chronological development memory & turn history for a file |

## Development Memory

Entire stores the development context across Git checkpoints. The **Development Memory** layer organizes that context into historical intent, explicit architectural decisions, reasons, modifications, and verifiable provenance evidence without hallucinating causality.

### Memory Pipeline

```
Roo Code (VS Code)
       ↓
entire-agent-roo watch
       ↓
Entire Checkpoint (git branch entire/checkpoints/v1)
       ↓
Development Memory (.entire/tmp/roo/*.json & task storage)
 ├── Intent
 ├── Decisions
 ├── Reasons
 ├── Changes
 ├── Problems
 ├── Outcomes
 └── Evidence
```

### Querying File Provenance (`why`)

To understand why a specific file exists, what problem prompted its creation/modification, and what explicit decisions were made:

```bash
entire-agent-roo why src/auth/session.ts
```

Example Output:
```
================================================================================
WHY THIS FILE EXISTS: src/auth/session.ts
================================================================================

Intent:
  Implement persistent OAuth sessions

Related Decisions:
  - Separate OAuth callback from session creation
    Reason: Token validation requires independent retry logic.
    Source: Session 1741243542 (Checkpoint 0c873e5)

Evidence & Provenance:
  - Entire Checkpoint: 0c873e5
    Roo Session: 1741243542
    Tool Action: created/modified via write_to_file

Modification History:
  [2026-09-06T06:50:00Z] Modified in session 1741243542 (Intent: "Implement persistent OAuth sessions")
```

If no explicit rationale was articulated in the agent transcript, `entire-agent-roo why` reports:
```
Related Decisions:
  No explicit historical reason was captured.
```
*Zero-Hallucination Guarantee: Reasoning is only extracted when explicitly stated by the user or agent transcript.*

### Chronological File History (`history`)

To view the complete development progression of a file across sessions and turns:

```bash
entire-agent-roo history src/auth/session.ts
```

