# ContextForge — Development Memory for AI Coding Agents

## 1. Executive Summary

ContextForge provides a resilient normalization layer and architectural memory engine for Entire-integrated AI coding agents. It bridges the gap between raw, volatile agent transcript streams and long-term repository maintainability by standardizing multi-format agent events, extracting structured development intent and decisions, and detecting unobserved repository modifications that occur outside monitored agent sessions.

---

## 2. Problem Statement & Context Gap

While AI coding agents generate rich technical reasoning and implementation context during task execution, maintaining long-term repository provenance encounters two structural failure modes:

1. **Transcript Format Volatility (Track 3):** Upstream coding agents frequently evolve their storage models—such as transitioning from monolithic JSON structures to streaming JSONL logs—breaking existing parser pipelines and decoupling agent history from repository checkpoints.
2. **The Unobserved Context Gap:** Developers and external tools routinely perform direct edits, hotfixes, or rebases outside monitored agent workflows. While version control detects *what* files changed, the architectural *rationale* behind those changes is absent from the agent's memory graph.

Without an abstraction layer to normalize event streams and detect out-of-band modifications, repository development history fragments over time.

---

## 3. Scope & System Capabilities

ContextForge builds upon the foundation of Entire's external-agent protocol, checkpointing system, and Graphify dependency tools to introduce an end-to-end provenance architecture:

### Base Entire Framework
- External agent RPC protocol definition and process lifecycle hooks.
- Checkpoint commit management and shadow branch provenance tracking.
- Static dependency graph analysis (`graphify`).

### ContextForge Implementation
- **Roo Adapter:** Native integration discovering, monitoring, and parsing Roo agent sessions.
- **Resilient Normalization Boundary (`LoadSession`):** Dual-mode streaming parser handling both legacy monolithic JSON and streaming JSONL event feeds into a canonical `NormalizedEvent` schema.
- **Fault-Tolerant Event Processing:** Graceful classification of unknown schema types (`EventUnknown`) without pipeline termination, and partial session recovery for interrupted writes.
- **Development Memory Engine:** Structured extraction of high-level intent, explicit architectural decisions, tool provenance, and error outcomes mapped directly to touched files.
- **Unobserved Change Detection (`detect-context-gap`):** Git-integrated analysis engine identifying unobserved commits, mapping modified paths against historical development memory, and generating context-recovery prompts.

---

## 4. Track 3 Challenge: Format Normalization

### Challenge Description
Upstream agent formats may change without warning, replacing expected object schemas with line-delimited event streams.

### Engineering Solution
ContextForge introduces a strict **Normalization Boundary** implemented in `internal/roo/transcript.go`. The boundary isolates all lifecycle logic from raw file representations:

- **Format Autodetection:** Analyzes the file header and structure to distinguish between legacy JSON and streaming JSONL.
- **Schema Mapping:** Maps divergent event representations into a unified `NormalizedSession` consisting of standard `NormalizedEvent` records (`SessionStart`, `UserPrompt`, `AgentResponse`, `ToolCall`, `ToolResult`, `Checkpoint`, `SessionEnd`).
- **Partial Stream Recovery:** In cases of abruptly terminated agent processes, recovers all valid JSONL lines up to the EOF, flags `Partial: true`, and allows checkpoint generation to proceed without data loss.

---

## 5. System Architecture

```text
               +----------------------------------------+
               |        Roo Agent / External Tool        |
               +----------------------------------------+
                                    |
                            (File Persistence)
                                    v
               +----------------------------------------+
               |   Raw Transcripts (.entire/tmp/roo)    |
               |       (Monolithic JSON / JSONL)        |
               +----------------------------------------+
                                    |
                                    v
               +----------------------------------------+
               |    LoadSession() Normalization Layer   |
               |  - Stream parser & format autodetect   |
               |  - Fault-tolerant schema mapping       |
               +----------------------------------------+
                                    |
                                    v
               +----------------------------------------+
               |           NormalizedSession            |
               +----------------------------------------+
                        /                       \
                       v                         v
        +----------------------------+   +----------------------------+
        |  Entire Lifecycle Adapter  |   |  Development Memory Engine |
        |  - ProcessTask()           |   |  - Intent & decisions      |
        |  - Checkpoint generation   |   |  - File provenance links   |
        +----------------------------+   +----------------------------+
                       |                               |
                       v                               v
        +----------------------------+   +----------------------------+
        | Git / Entire Checkpoint    |   | Queryable Memory Database  |
        | (e.g. 0aa1e829c569)        |   | (entire-agent-roo why)     |
        +----------------------------+   +----------------------------+
```

---

## 6. Development Memory Extraction Model

ContextForge transforms raw conversational streams into structured, queryable provenance records:

- **Goal & Intent:** Captures root user directives and task requirements.
- **Architectural Decisions:** Identifies explicit justifications, trade-offs, and design rationales expressed in agent responses.
- **File & Tool Provenance:** Links decisions directly to modified repository paths and tool execution events.
- **Resolution Outcomes:** Records task completion states and error recovery actions.

### Zero-Hallucination Policy
If an agent modifies a file without documenting an explicit technical reason, the engine records:
`"No explicit historical reason was captured."`
The system strictly surfaces verifiable session evidence and avoids synthetic rationalizations.

---

## 7. Graph-Driven Dependency Isolation

Prior to implementing the Track 3 JSONL parser, Graphify was utilized to map module dependencies and guarantee architectural isolation.

**Dependency Path:**
```text
internal/roo/transcript.go (LoadSession)
          ↓
internal/roo/scanner.go (ScanOnce)
          ↓
internal/roo/task.go (ProcessTask)
```

By enforcing that `LoadSession()` returns a canonical `NormalizedSession`, `ProcessTask()` and downstream lifecycle handlers remained untouched during the Track 3 format migration.

---

## 8. Fault Tolerance & Edge Cases

| Scenario | System Behavior | Verification Status |
| :--- | :--- | :--- |
| **New / Undefined Event Types** | Safely parsed as `EventUnknown`; processing continues without error. | Verified (`TestParseJSONLSession_UnknownEvents`) |
| **Truncated / Corrupted Transcripts** | Recovers valid events prior to corruption; flags session as `Partial`. | Verified (`TestParseJSONLSession_PartialStream`) |
| **Monolithic Legacy JSON** | Autodetected and normalized into identical event schema. | Verified (`TestLoadSession_LegacyFormat`) |
| **Streaming Multi-Event JSONL** | Parsed per-line; reconstructs complete tool execution timeline. | Verified (`TestLoadSession_StreamingJSONL`) |
| **Cross-Platform File Paths** | Normalized to forward slashes across Unix and Windows environments. | Verified (`TestCleanFilePath_CrossPlatform`) |

---

## 9. Integration with Entire Core

ContextForge operates as a standards-compliant external agent binary within the Entire ecosystem:
- Implements Entire external agent CLI command protocol (`scan`, `process`, `why`).
- Emits standard JSON-RPC events consumed by the Entire supervisor daemon.
- Generates native Entire checkpoints referencing repository commit trees.
- Extends the protocol with `detect-context-gap` for Git-level unobserved change analysis.

---

## 10. Automated Test Suite & Verification

The test suite covers normalization, memory extraction, lifecycle coordination, and context gap detection across 30 automated tests:

```bash
# Execute full internal test suite
go test -v ./internal/roo/...

# Verify code formatting and linting
go vet ./...

# Build standalone agent binary
go build ./cmd/entire-agent-roo
```

**Key Test Coverage:**
- `internal/roo/transcript_test.go`: Format detection, JSONL streaming, corrupted line recovery.
- `internal/roo/memory_test.go`: Intent capture, decision parsing, zero-hallucination guarantees.
- `internal/roo/context_gap_test.go`: Git commit inspection, unobserved change classification, file correlation.

---

## 11. Verification & Demonstration Steps

Judges can verify the implementation directly using the following sequence:

1. **Verify Unit & Integration Tests:**
   ```bash
   cd agents/entire-agent-roo
   go test -v ./internal/roo/...
   ```
2. **Inspect Track 3 JSONL Normalization:**
   Review `track-3-agent-session.jsonl` and execute the parser verification test in `transcript_test.go`.
3. **Inspect Checkpoint Provenance:**
   Review generated checkpoint artifacts linking normalized session metadata to commit `0aa1e829c569`.
4. **Execute Unobserved Change Detection:**
   ```bash
   ./entire-agent-roo detect-context-gap <commit-sha>
   ```
   Inspect the resulting context gap analysis highlighting changed files without Entire session metadata.

---

## 12. Technical Differentiation

- **Canonical Normalization Boundary:** Completely insulates downstream lifecycle handling from agent log format changes.
- **Structured Architectural Provenance:** Replaces unstructured text searching with deterministic decision-to-file mapping.
- **Bi-Directional Context Tracking:** Bridges both observed AI agent sessions and manual out-of-band developer commits.

---

## 13. System Boundaries & Known Constraints

- **Live Roo UI Verification:** Track 3 compliance has been verified against the organizer JSONL fixture and synthetic test streams; live UI validation is constrained by the local headless test environment.
- **Decision Extraction Engine:** Architectural decision capture currently utilizes deterministic regex and keyword heuristics; complex natural language reasoning will benefit from future LLM-assisted classification.
- **Memory Storage Backend:** Development Memory is currently indexed via local structured files suitable for single-repository scale; enterprise deployment will require graph/vector storage integration.

---

## 14. Roadmap & Future Work

- **Interactive Context Recovery:** Integrating `detect-context-gap` into interactive terminal prompts (`bubbletea`) and GitHub PR validation checks to capture developer intent before merge.
- **LLM-Assisted Reasoning Extraction:** Incorporating small, local LLM inference models to capture nuanced technical justifications from freeform agent conversation.
- **Multi-Agent Protocol Expansion:** Extending the normalization schema to provide unified development memory across Cline, Claude Code, Cursor, and Roo.
