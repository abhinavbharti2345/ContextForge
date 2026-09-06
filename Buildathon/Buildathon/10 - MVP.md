---
title: 10 - MVP
tags:
  - buildathon
  - mvp
  - milestones
  - requirements
date: 2026-09-06
status: active
---

# 10 - MVP

> [!NOTE]
> **MVP North Star:** A functional, end-to-end integration demonstrating:
> 1. Real-time capture of an unsupported AI agent's session into Entire checkpoints.
> 2. Seamless continuation across sessions where the agent recalls prior decisions without manual context re-entry.

---

## 1. MVP Milestone Breakdown

```mermaid
graph TD
    M1["Milestone 1: Single Session Capture (Base MVP)\n• Intercept agent prompt & tool calls\n• Transmit to Entire Protocol\n• Link Entire Checkpoint to Git commit"]
    
    M2["Milestone 2: Multi-Session Context Recall\n• Start Session 2 with minimal prompt\n• Adapter queries Entire checkpoint history\n• Injects prior architectural context to agent"]
    
    M3["Milestone 3: Explain & Rewind Showcase\n• 'entire explain' displays reasoning trace\n• 'entire rewind' resets codebase & agent state\n• Standalone reproducible demo.sh script"]
    
    M1 --> M2 --> M3
```

---

## 2. Granular Milestone Deliverables

### Milestone 1: Single Session Capture (Base MVP)
* **Goal**: Prove that the unsupported agent can run a coding task and have its full context recorded in Entire.
* **Flow**:
  1. User starts agent via our adapter.
  2. User issues a coding task (e.g., "Add User authentication model").
  3. Agent explores repository, edits files, and runs tests.
  4. Adapter captures:
     - Prompts and system instructions.
     - Files inspected and modified.
     - Tool invocations and terminal command logs.
  5. Entire CLI receives events and writes a **Checkpoint** linked to the resulting Git commit.

### Milestone 2: Multi-Session Context Recall ("Agent Remembers")
* **Goal**: Prove that Entire's stored context provides persistent memory for the agent.
* **Flow**:
  1. User closes Session 1.
  2. User starts Session 2 with a brief prompt: `"Continue auth work. Add JWT token validation."`
  3. Adapter retrieves prior checkpoint metadata from Entire.
  4. Agent starts Session 2 with full knowledge of Session 1's decisions, file paths, and test outputs without the user re-pasting anything.

### Milestone 3: Inspection & Rewind Polish
* **Goal**: Provide the killer developer experience that wows hackathon judges.
* **Flow**:
  1. Run `entire explain <commit-sha>`: outputs the complete prompt, rationale, and tool traces for that commit.
  2. Run `entire rewind`: cleanly rolls back both the working tree and the agent's context to a prior checkpoint.

---

## 3. Strict MVP Scope Boundaries

```mermaid
graph LR
    subgraph IN SCOPE [100% FOCUS]
        IN1["Go Adapter Binary"]
        IN2["Agent Hook / Event Interceptor"]
        IN3["Entire Protocol Normalization"]
        IN4["Checkpoint Creation & Commit Link"]
        IN5["Cross-Session Context Injection"]
        IN6["E2E Tests & demo.sh"]
    end

    subgraph OUT OF SCOPE [DO NOT BUILD]
        OUT1["Custom Web UI / Dashboard"]
        OUT2["New LLM Model / Agent Engine"]
        OUT3["Custom Git Server / Storage Engine"]
        OUT4["Multi-agent Orchestrator"]
    end
```

---

## 4. MVP Acceptance Checklist

- [ ] Adapter builds cleanly via `mise run build` / `go build`.
- [ ] Agent process launches successfully through the adapter.
- [ ] Prompts and turn responses are captured in real-time.
- [ ] Tool calls (e.g. bash commands, file reads) are recorded.
- [ ] Entire Checkpoints are created and linked to Git commit SHAs.
- [ ] `entire explain` returns formatted context for the agent's commits.
- [ ] Session 2 successfully recalls decisions made in Session 1.
- [ ] Automated tests in `tests/` pass with zero failures.
- [ ] `scripts/demo.sh` runs the full lifecycle from scratch reliably.

---

## 5. Fact vs Decision vs Assumption

### Confirmed Facts
* The MVP must show both capture (Day 1) and value delivery (Day 2 context retrieval / explain).
* Entire handles the underlying checkpoint storage mechanics.

### Decisions Made
* Prioritize Milestone 1 & 2 before polishing Milestone 3.
* Keep the demo scenario concrete and familiar (Authentication module creation and extension).

### Assumptions
* The target agent's CLI supports pre-loading or injecting context during initialization.

### Things to Verify
* The exact command arguments for `entire explain` and `entire rewind` in `entireio/cli`.

---

## Related Notes
* [[03 - What We Have To Build]] — Deliverables definition.
* [[08 - Development Workflow]] — Phased roadmap.
* [[11 - Demo Flow]] — Detailed step-by-step presentation script.
* [[Buildathon — What We Actually Need To Do]] — Execution checklist.
