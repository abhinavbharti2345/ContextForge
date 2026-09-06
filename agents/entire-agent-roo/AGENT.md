# Roo Code — External Agent Research & Architecture Specification

## Verdict: COMPATIBLE (Storage Ingestion Model)

Roo Code (formerly Roo-Cline) is an AI coding assistant supporting multi-mode operation, MCP servers, and durable task persistence. It stores comprehensive task history across `ui_messages.json` (UI trace and tool interactions) and `api_conversation_history.json` (raw model messages with token usage breakdowns).

Roo Code does not provide native lifecycle command hooks in its extension codebase. Instead, the adapter functions as a **storage-driven external agent**, ingesting and analyzing live task storage from the VS Code `globalStorage` directory.

## Static Checks
| Check | Result | Notes |
| :--- | :--- | :--- |
| Binary present | Optional | `roo` / `roo-code` via `@roocode/cli`; VS Code desktop extension supported via globalStorage discovery |
| Storage discovery | PASS | `%APPDATA%\Code\User\globalStorage\rooveterinaryinc.roo-cline\tasks\` (Windows), `~/Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline/tasks/` (macOS), `~/.config/Code/User/globalStorage/rooveterinaryinc.roo-cline/tasks/` (Linux) |
| Native Hooks | FAIL / N/A | No native command hook dispatcher in Roo Code codebase |
| Tool events | PASS | `write_to_file`, `replace_in_file`, `execute_command`, `read_file`, `use_mcp_tool` |

## Storage & Transcript Model
- **Task Structure**:
  - `ui_messages.json`: Array of `ClineMessage` objects containing timestamp, type (`say`/`ask`), message label (`task`, `tool`, `api_req_started`, `completion_result`), and text.
  - `api_conversation_history.json`: Array of Anthropic/OpenAI formatted messages tracking model reasoning, structured tool requests (`write_to_file`, `replace_in_file`), and tool execution results.
- **Session Reference**:
  - Materialized snapshot stored under `.entire/tmp/roo/<taskId>.json` containing `RooTaskEnvelope`.
- **Modified File Extraction**:
  - Parsed from `ui_messages` (`say: "tool"`) and `api_conversation_history` (`tool_use` parts: `write_to_file`, `replace_in_file`, `create_file`, `delete_file`).
- **Token Usage**:
  - Aggregated from `ui_messages` (`say: "api_req_started"` payloads containing `tokensIn`, `tokensOut`, `cacheWrites`, `cacheReads`).

## Protocol Mapping
| Subcommand | Native Roo Concept | Implementation |
| :--- | :--- | :--- |
| `info` | Static metadata | Returns name `roo`, type `Roo Code`, preview, `hooks: false` |
| `detect` | CLI / globalStorage | Checks `roo` on PATH or VS Code task storage directory |
| `get-session-id` | `taskId` | Returns task ID |
| `get-session-dir` | `.entire/tmp/roo` | Isolated temp session dir |
| `resolve-session-file` | `<taskId>.json` | `<session-dir>/<taskId>.json` |
| `read-session` | Task envelope | Returns native JSON bytes, start time, and modified files |
| `write-session` | Task envelope | Persists native data to `session_ref` |
| `read-transcript` | Task envelope | Raw bytes |
| `chunk-transcript` | Byte chunks | Base64 chunking |
| `reassemble-transcript`| Byte chunks | Concatenates chunks |
| `compact-transcript` | Compact JSONL | Emits Entire Compact Transcript format |
| `prepare-transcript` | Storage Ingestion | Ingests `ui_messages.json` + `api_conversation_history.json` from globalStorage |
| `format-resume-command`| Roo CLI resume | `roo --resume <taskId>` |
| `parse-hook` | No-op | Returns `nil` (Roo has no native command hooks) |
| `install-hooks` | No-op | Returns `0, nil` |
| `uninstall-hooks` | No-op | Returns `nil` |
| `are-hooks-installed` | No-op | Returns `false` |
| `get-transcript-position`| UI message count | Returns number of UI messages |
| `extract-modified-files` | Mutating tool calls | Extracts file paths from `write_to_file`, `replace_in_file`, etc. |
| `extract-prompts` | Task / User prompts | Reads initial task prompt and subsequent user turns |
| `extract-summary` | Completion / text | Extracts `completion_result` or last assistant text |
| `calculate-tokens` | `api_req_started` | Aggregates token and cache counts |

## Selected Capabilities
| Capability | Declared | Justification |
| :--- | :--- | :--- |
| `hooks` | **false** | Roo Code has no native command hooks |
| `transcript_analyzer` | **true** | `ui_messages.json` + `api_conversation_history.json` contain structured tools & prompts |
| `transcript_preparer` | **true** | Ingests live task data from VS Code `globalStorage` |
| `compact_transcript` | **true** | Formats to Entire Compact Transcript format |
| `token_calculator` | **true** | Accurate token counts in `api_req_started` |
| `uses_terminal` | **true** | Roo commands execute via terminal |
| `text_generator` | false | Defer |
| `hook_response_writer` | false | Defer |
| `subagent_aware_extractor` | false | Defer |
