# entire-agent-roo

Entire external agent integration binary for [Roo Code](https://github.com/RooCodeInc/Roo-Code).

## Overview

`entire-agent-roo` connects Roo Code (VS Code Extension & CLI) with the Entire Git-native checkpoint system. It implements all standard protocol subcommands for:

- Discovering Roo Code global task storage (`ui_messages.json` + `api_conversation_history.json`)
- Extracting modified files from mutating tool calls (`write_to_file`, `replace_in_file`, `edit_file`, etc.)
- Extracting prompts and assistant summaries
- Calculating token usages (`tokensIn`, `tokensOut`, `cacheReads`, `cacheWrites`)
- Formatting resume commands (`roo --resume <taskId>`)

## Protocol Commands

| Subcommand | Description |
| :--- | :--- |
| `info` | Returns agent metadata and declared capabilities |
| `detect` | Checks for presence of `roo` binary or task storage |
| `get-session-id` | Extracts session/task ID from hook input |
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
| `parse-hook` | Normalizes lifecycle hook payloads |
| `install-hooks` | Configures workspace `.roo/hooks.json` |
| `uninstall-hooks` | Cleans up `.roo/hooks.json` |
| `are-hooks-installed` | Checks hook configuration status |
| `get-transcript-position`| Returns message count |
| `extract-modified-files` | Parses mutating tool calls |
| `extract-prompts` | Extracts user prompts |
| `extract-summary` | Extracts assistant response summary |
| `calculate-tokens` | Aggregates token metrics |
