# Hackathon Judge Walkthrough Guide

This document is your tactical guide for presenting the codebase to the judges. It explicitly separates what the base repository provided from what we engineered, highlighting the specific files and code blocks that prove our technical contribution.

---

## 1. What We Inherited vs. What We Built

**What we started with (The Base Repo):**
- The Entire CLI (`entire status`, `entire checkpoint list`).
- The basic external agent protocol schemas.
- The Entire Graph (`graphify`) dependency mapping tool.

**What we engineered (Our Improvements):**
1. **The Roo Adapter Integration:** A native bridge connecting the Roo AI agent to the Entire platform.
2. **Development Memory Engine:** An analytical parser that reads unstructured agent logs to extract architectural decisions, intent, and file-level evidence (replacing blind file-watching).
3. **The Normalization Boundary (Track 3 Solution):** A resilient streaming parser that dynamically handles multiple data formats (JSON and JSONL), ignores unknown events, and salvages corrupted transcripts without crashing.

---

## 2. The Core Files to Show Judges

During the code walkthrough, open these exact files. They represent our biggest technical achievements.

### 📍 File 1: `agents/entire-agent-roo/internal/roo/transcript.go`
**What this is:** The Normalization Boundary (The Track 3 Solution).
**What to show:** Show the `LoadSession()` function.
**How to explain it:** 
> *"When Track 3 changed the agent's output format from monolithic JSON to streaming JSONL, we didn't hack our lifecycle logic. We built this strict normalization boundary. Look at `LoadSession`. It dynamically detects the format, safely ignores unknown events, and processes streaming JSONL line-by-line. If a session crashes mid-stream, this code salvages all previous lines into a partial `NormalizedSession` instead of discarding the file. This decoupled our integration from upstream format volatility."*

### 📍 File 2: `agents/entire-agent-roo/internal/roo/memory.go`
**What this is:** The Development Memory Engine.
**What to show:** Show the `ExtractMemoryFromSession()` function and the `DevelopmentMemory` struct.
**How to explain it:** 
> *"A normal adapter just watches for file changes and triggers a checkpoint. We built an intelligence layer. Look at this extraction logic. It scans the agent's raw 'thought' blocks and pulls out explicit architectural decisions, the reasoning behind them, and links them directly to the modified files. We turn a 10,000-line messy transcript into a clean, queryable Development Memory that tells future developers exactly WHY a file was changed."*

### 📍 File 3: `agents/entire-agent-roo/internal/roo/watcher.go`
**What this is:** The Lifecycle Integration.
**What to show:** Show `ProcessTask()` where it uses `NormalizedSession`.
**How to explain it:**
> *"This is where we hook into the Entire protocol. Notice how clean this logic is. Because of our normalization layer, `ProcessTask` doesn't know if the data came from legacy JSON or the new Track 3 JSONL stream. It just iterates over standardized events to reliably trigger `SessionStart`, `TurnEnd`, and `SessionEnd` hooks. It's perfectly decoupled."*

### 📍 File 4: `agents/entire-agent-roo/internal/roo/memory_test.go` (and `transcript_test.go`)
**What this is:** Our Testing and Validation.
**What to show:** The test suite, specifically `TestLoadSession_IncompleteTranscript` and `TestExtractMemoryExplicitDecisionAndReason`.
**How to explain it:**
> *"We didn't just build this on a happy path. We wrote 30 unit tests covering edge cases. This test proves that if a transcript is cut in half, we still generate a safe partial session. This other test proves we can accurately extract an architectural decision out of free-form natural language."*

---

## 3. The Live Demo Flow (How to drive the presentation)

When it's time to show the integration in action, follow this script:

**Step 1: Run the tests**
```bash
go test -v ./agents/entire-agent-roo/internal/roo
```
*Point out:* "All 30 tests pass, including the Track 3 compatibility and resilience tests."

**Step 2: Show the Graphify Blast Radius**
```bash
python -m graphify path "LoadSession" "ProcessTask"
```
*Point out:* "When the format changed, we used the Entire Graph to prove our architecture was decoupled. The graph shows exactly 2 hops between parsing and processing. We contained the blast radius perfectly."

**Step 3: Show the Real Checkpoint**
```bash
entire checkpoint explain 0aa1e829c569
```
*Point out:* "Here is the final result running against the Track 3 JSONL fixture. Look at the 'Intent' section. That isn't a git commit message written by a human. That is the Development Memory engine extracting the exact architectural intent from a messy JSONL stream and permanently attaching it to this Entire checkpoint."

---

## 4. How to Handle "Why does this matter?"

If a judge asks why this is better than what the base repo had, say:

> *"The base repo gave us a protocol to save files. We built an engine to save context. If production goes down at 3 AM and a developer sees an AI agent changed an auth middleware three weeks ago, raw files don't help them. They need to know WHY the agent made that change. Our adapter guarantees that the reasoning, the decisions, and the intent are permanently attached to the code, completely immune to upstream data format changes."*
