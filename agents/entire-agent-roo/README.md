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
| `info` | Returns agent metadata and declared capabilities (`hooks: false`) |
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
| `format-resume-command`| Formats task resume shell command |
| `parse-hook` | No-op (returns nil) |
| `install-hooks` | No-op (returns 0, nil) |
| `uninstall-hooks` | No-op (returns nil) |
| `are-hooks-installed` | No-op (returns false) |
| `get-transcript-position`| Returns message count |
| `extract-modified-files` | Parses mutating tool calls |
| `extract-prompts` | Extracts user prompts |
| `extract-summary` | Extracts assistant response summary |
| `calculate-tokens` | Aggregates token metrics |
| `watch` | Runs the storage-driven lifecycle watcher sidecar |
