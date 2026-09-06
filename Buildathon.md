# ContextForge — Development Memory for AI Coding Agents

## 1. One-Line Summary

ContextForge bridges the gap between unstructured AI agent outputs and long-term project maintainability by turning raw session transcripts and unobserved developer commits into a queryable, permanent Development Memory.

## 2. The Problem

AI coding agents are incredible at generating useful reasoning, architectural decisions, and development context while they work. When integrated with Entire, this context is captured and directly connected to checkpoints in your repository. 

However, development history easily becomes fragmented:
- **Format Volatility:** Agents change their output formats (like switching from monolithic JSON to streaming JSONL).
- **The Context Gap:** Human developers or external AI tools frequently edit files directly and push commits *outside* of observed agent sessions. 

When changes happen outside an observed session, future developers and agents can see **WHAT** changed in the Git history, but they have no idea **WHY** it changed. Without reasoning, the project's historical context deteriorates.

## 3. What We Built

To solve this, we engineered an intelligence layer on top of the Entire external-agents base repository.

### Already provided by Entire
The base repository provided the core external-agent protocol, the Checkpoint metadata engine, and the Graphify dependency analysis tool. It laid the foundation for tracking when files changed and associating them with a session ID.

### Added by ContextForge
We engineered a robust adapter and memory engine:
- **Roo External-Agent Adapter:** A native integration that discovers and parses Roo tasks/sessions.
- **The Normalization Boundary:** A resilient layer that dynamically parses both legacy monolithic JSON and streaming JSONL formats into a standardized `NormalizedEvent` model.
- **Resilient Parsing:** Flawless handling of unknown events (which are safely ignored without crashing) and incomplete/truncated transcripts (which are salvaged into partial sessions).
- **Lifecycle Integration:** Seamlessly triggers `SessionStart`, `TurnEnd`, and `SessionEnd` Entire checkpoints regardless of the upstream data format.
- **Development Memory Engine:** A heuristic engine that extracts explicit architectural decisions, intents, problems, and outcomes from raw agent responses and links them to the modified files.
- **Unobserved Change Detector:** A newly implemented Git-integrated feature that detects manual commits lacking an Entire footprint, cross-references the changed files against historical Development Memory, and highlights the "context gap" so the developer can explain their reasoning.

## 4. Track 3: The Agent Changed Its Format

**OLD ASSUMPTION**
→ The agent's transcript format is a stable, monolithic JSON object.

**CURVEBALL**
→ The organizer provided a new, streaming JSONL event format that broke existing parsing logic.

**OUR RESPONSE**
→ We introduced a strict Normalization Boundary (`LoadSession()` in `transcript.go`).

**RESULT**
→ Both the legacy format and the new JSONL format successfully produce the exact same normalized development context. 

By mapping the streaming JSONL data to a standard `NormalizedSession`, we shielded the core `ProcessTask()` lifecycle logic from upstream format volatility. Unknown events are safely mapped to `EventUnknown`, and if a JSONL file is cut off mid-stream, the parser recovers all preceding valid lines and flags the session as `Partial`, perfectly preserving existing checkpoint behavior.

## 5. Architecture

```text
Roo Agent
    ↓
Task / Session Storage (.entire/tmp/roo)
    ↓
LoadSession() (The Normalization Boundary)
    ↓
NormalizedSession (Contains standard NormalizedEvents)
    ↓
Roo Lifecycle / ProcessTask()
    ↓
Entire Checkpoint (e.g., 0aa1e829c569)
    ↓
Development Memory (Extracts Intent, Decisions, Outcomes)
```

- **LoadSession():** Dynamically detects the format (JSON vs JSONL) and sanitizes the data.
- **NormalizedSession:** Provides a stable interface for the rest of the application.
- **ProcessTask():** Triggers the Entire lifecycle hooks.
- **Development Memory:** Analyzes the normalized events to extract the "Why".

## 6. Development Memory

A raw transcript is just a chronological log of text and tool calls. Development Memory is an organized graph of architectural intent.

**WHAT changed + WHY it changed + EVIDENCE + WHAT happened afterward**

Instead of forcing a developer to read a 10,000-line JSON file, ContextForge extracts:
- **Intent:** The overarching goal (e.g., "Add coupon validation to checkout").
- **Decisions & Reasons:** Explicit choices made by the agent (e.g., "I decided to make expiry take precedence over disabled state because the tests failed").
- **Evidence & Provenance:** Direct links to the tools used (e.g., `write_to_file`) and the Exact Roo Session ID.
- **Problems & Outcomes:** Errors encountered and the final resolution state.

**The "Why does this file exist?" Use Case:**
Using the `entire-agent-roo why <file>` command, a developer can instantly see a chronological history of every architectural decision an agent ever made regarding a specific file, completely replacing the need for digging through Git blame.

## 7. Graph-Driven Development

Before refactoring our parser for Track 3, we used Entire's **Graphify** tool to map the blast radius of the JSONL format change.

**Verified Dependency Path:**
```text
LoadSession()
    ↓
ScanOnce()
    ↓
ProcessTask()
```
Graph analysis proved that by updating `LoadSession()` to return a `NormalizedSession`, we could completely insulate `ProcessTask()` from the format change. This allowed us to build the Normalization Boundary precisely where it was needed instead of blindly editing lifecycle code.

## 8. Reliability

Safety is built into the core design:

- **Unknown events:** If the agent introduces a new event type (e.g., `EventUnknown`), the adapter safely ignores it and continues processing without crashing.
- **Incomplete transcripts:** If a session crashes mid-write, `parseJSONLSession` salvages all valid lines before the corruption and marks `session.Partial = true`.
- **No fabricated reasoning (Zero Hallucination Rule):** If an agent modifies a file but doesn't explicitly state *why*, the Development Memory engine strictly outputs: *"No explicit historical reason was captured."* It relies on evidence, never inventing a reason.

## 9. Entire Integration

ContextForge does not duplicate Entire; it amplifies it.

We strictly reuse the Entire adapter protocol, `entire status`, and the Entire checkpoint system. When ContextForge processes a normalized session, it triggers the standard Entire hooks to produce a real checkpoint (e.g., `0aa1e829c569` generated from the Track 3 JSONL fixture). 

Furthermore, our Unobserved Change Detector reuses existing local Git infrastructure (`git log`, `git diff-tree`) instead of reinventing version control parsing.

## 10. Testing

Our implementation is backed by a robust, 100% passing test suite:
- **Old format / New format:** Verifies `LoadSession()` correctly parses both monolithic JSON and streaming JSONL.
- **Unknown event / Incomplete transcript:** Proves the parser doesn't panic on bad data and successfully recovers partial sessions.
- **Modified-file extraction:** Ensures file paths are correctly normalized regardless of OS slashes.
- **Unobserved Commit logic:** Proves that the system accurately flags commits missing an Entire footprint and successfully links them to historical Development Memory.
- **Go tests, go vet, and go build:** All pass cleanly, confirming production-readiness.

## 11. Demo Flow

**To demonstrate the integration:**
1. **Show legacy format parsing:** Run the tests verifying old monolithic JSON.
2. **Show the new JSONL format:** Open `track-3-agent-session.jsonl` to show the organizer's fixture.
3. **Show normalization & resilience:** Run `go test -v ./internal/roo/...` to prove unknown events and incomplete files are handled safely.
4. **Show Graph impact:** Run `python -m graphify path "LoadSession" "ProcessTask"` to validate the architectural boundary.
5. **Show the Entire checkpoint:** Run `entire checkpoint explain 0aa1e829c569` to view the real checkpoint generated from the Track 3 fixture.
6. **Show Unobserved Change Detection:** Run `entire-agent-roo detect-context-gap 2d1481b` to see the engine catch a manual developer commit and expose the missing context.

## 12. Why This Helps Developers

- **Future AI Agents:** Agents don't have to re-read the entire codebase; they can query the Development Memory to understand past architectural decisions.
- **Developers & Code Review:** Instantly answers "Why was this written this way?" without relying on vague commit messages.
- **Debugging & Onboarding:** Drastically reduces the time required to understand legacy code or fragmented agent sessions.
- **Maintaining Continuity:** The Context Gap detector ensures that manual hotfixes don't silently erase the historical reasoning of the project.

## 13. What Makes This Different

We did not invent parsing or Git hooks. What makes ContextForge powerful is the specific combination of:

**Entire's Checkpoint Capture + Format-Resilient Agent Integration + Normalized Event Model + Development Memory Engine + Unobserved Context Gap Detection.**

We transformed a volatile, unstructured text log into a permanent, queryable database of architectural provenance.

## 14. Limitations

- **Fresh Live Roo E2E Validation:** The Track 3 logic was validated flawlessly against the provided JSONL fixture, but full live validation with a newly spawned Roo UI session is pending final E2E environment setup.
- **Causal Inference:** The Development Memory relies on static regex heuristics. While highly accurate for explicit statements, it can miss nuanced, conversational reasoning that an LLM would catch.
- **Production-Scale Memory Search:** Currently, Development Memory is parsed sequentially from local `.json` files. For massive enterprise repositories, this would need to be migrated to a proper vector or graph database.

## 15. Future Work

- **LLM-Powered Extraction:** Routing normalized `agent_response` events through a lightweight local LLM (instead of regex) to guarantee 100% accurate extraction of architectural decisions.
- **Automated Developer Prompts:** Wiring the `detect-context-gap` command into an interactive terminal UI (`charmbracelet/bubbletea`) or GitHub PR bot to force developers to fill in missing reasoning before a merge.
- **Richer Cross-Agent Memory:** Expanding the Normalization Boundary to support Cursor, Cline, and other agents so memory is shared seamlessly across tools.

## 16. Final Pitch

**15-Second Pitch**
"ContextForge is an intelligence layer for Entire that turns messy, fragmented AI coding sessions into a permanent, queryable database of architectural decisions, ensuring you always know exactly *why* your code was written."

**30-Second Pitch**
"AI coding agents generate incredible context, but it gets lost the moment a human pushes a manual commit or the agent changes its data format. ContextForge solves this. We built a resilient normalization engine that handles upstream format changes, a Development Memory extractor that permanently links architectural decisions to your files, and a Context Gap detector that catches unobserved manual edits—ensuring your project's reasoning is never lost."

**2-Minute Pitch**
"When AI agents write code, they leave behind massive, unstructured logs of their reasoning. The problem is twofold: First, agents constantly change their output formats, breaking integrations. Second, when a human developer manually edits those same files later, they create a 'Context Gap' where the code changes, but the reasoning is lost. 

We built ContextForge to fix this. We started by building a strict Normalization Boundary for the Roo agent. When Track 3 threw a curveball with a completely new streaming JSONL format, our architecture absorbed it flawlessly. We convert both legacy and new formats into a standardized event model, safely ignoring unknown events and salvaging broken files. 

But we didn't stop at parsing. We built a Development Memory Engine. It scans those normalized events, extracts the explicit architectural decisions, and links them directly to the modified files in an Entire Checkpoint. Finally, we built an Unobserved Change Detector. If a human pushes a commit without an agent session, ContextForge detects the changed files, cross-references our Development Memory, and flags the exact historical decisions that are missing context. We aren't just saving code; we are permanently preserving the 'Why'."

## 17. Judge Q&A

**Didn't Entire already have adapters?**
Entire provides the protocol and the Checkpoint system. We built the actual intelligence layer: the format-resilient normalizer, the memory extractor, and the unobserved change detector.

**What changed because of Track 3?**
The data format changed from a monolithic JSON object to a streaming JSONL file. We implemented a Normalization Boundary to absorb this without breaking the Entire lifecycle.

**How do you handle unknown events and incomplete transcripts?**
Unknown events are mapped to `EventUnknown` and safely ignored. If a transcript is truncated, our parser salvages all valid lines before the break and marks the session as `Partial`.

**Why is Development Memory useful? / How is it different from a transcript?**
A transcript is a 10,000-line unstructured log. Development Memory is an organized graph showing exactly what files changed, the explicit architectural decisions behind them, and the provenance. It answers "Why does this file exist?" instantly.

**How did you use Graph?**
We used Graphify to map the dependency path (`LoadSession` -> `ProcessTask`). This proved we could contain the Track 3 format change entirely within the parsing boundary without risking the lifecycle integration.

**What is genuinely novel?**
The combination of Format-Resilient parsing with Unobserved Change Detection. We can detect a manual developer commit and instantly link it back to an AI's architectural decision from two weeks ago.

## 18. Final Takeaway

The goal is not merely to remember that code changed. 

The goal is to preserve enough development context that the next human or AI can understand what changed, why it changed, and what came before it.
