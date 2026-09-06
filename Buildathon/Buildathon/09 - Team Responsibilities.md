---
title: 09 - Team Responsibilities
tags:
  - buildathon
  - team
  - roles
  - ownership
date: 2026-09-06
status: active
---

# 09 - Team Responsibilities

> [!IMPORTANT]
> **Zero Code Conflicts Rule:** To move fast without stepping on each other's toes, the 3 team members have clearly divided domain ownership across repository directories and technical layers.

---

## 1. Team Domain Ownership Matrix

```mermaid
graph TD
    subgraph Person 1: Integration Lead
        P1["Person 1: Agent Adapter Lead"]
        P1 --> F1["internal/<agent>/agent.go\n(Lifecycle State Machine)"]
        P1 --> F2["internal/<agent>/hooks.go\n(Event Interceptors)"]
        P1 --> F3["internal/<agent>/transcript.go\n(Turn & Log Parser)"]
    end

    subgraph Person 2: Protocol & Context Lead
        P2["Person 2: Protocol & Memory Lead"]
        P2 --> F4["internal/protocol/protocol.go\n(Entire Wire Protocol)"]
        P2 --> F5["internal/protocol/types.go\n(Protocol Schema)"]
        P2 --> F6["tests/ & e2e/\n(Protocol Tests & Context Recall Validation)"]
    end

    subgraph Person 3: Product, Demo & Docs Lead
        P3["Person 3: Demo & Docs Lead"]
        P3 --> F7["mise.toml & go.mod\n(Build & Task Config)"]
        P3 --> F8["AGENT.md & README.md\n(Specs & Documentation)"]
        P3 --> F9["scripts/demo.sh & sample-repo/\n(Live Demo Workflow)"]
    end

    P1 -->|Hands normalized turns| P2
    P2 -->|Provides stable protocol & test harness| P1
    P3 -->|Packages, tests, and presents| P1
    P3 -->|Packages, tests, and presents| P2
```

---

## 2. Granular Role Breakdown

### Person 1 — Entire Integration & Adapter Lead
* **Primary Focus**: The interface between the target agent and the adapter.
* **Owned Files**:
  * `agents/entire-agent-<target>/cmd/entire-agent-<target>/main.go`
  * `agents/entire-agent-<target>/internal/<target>/agent.go`
  * `agents/entire-agent-<target>/internal/<target>/hooks.go`
  * `agents/entire-agent-<target>/internal/<target>/transcript.go`
  * `agents/entire-agent-<target>/internal/<target>/types.go`
* **Core Responsibilities**:
  * Intercept target agent processes, stdio streams, or hook events.
  * Capture `SESSION_START`, `USER_PROMPT`, `AGENT_RESPONSE`, `TOOL_CALL`, `FILE_CHANGES`, `GIT_COMMIT`.
  * Convert messy raw agent output into clean, structured turns.
* **Deliverable**: The agent's real-time events are reliably captured and exposed to the protocol layer.

---

### Person 2 — Protocol, Context & Memory Lead
* **Primary Focus**: The interface between the adapter and the Entire engine, plus cross-session context recovery.
* **Owned Files**:
  * `agents/entire-agent-<target>/internal/protocol/protocol.go`
  * `agents/entire-agent-<target>/internal/protocol/types.go`
  * `agents/entire-agent-<target>/tests/unit/`
  * `agents/entire-agent-<target>/tests/e2e/`
* **Core Responsibilities**:
  * Ensure full conformance with Entire's external agent protocol wire format.
  * Verify that checkpoints and Git commit links are created properly.
  * Build and validate the **"Agent Remembers"** feature: retrieving previous session context and feeding it back to the agent for Day 2 continuation.
  * Prevent token blowups by designing concise context summaries.
  * Author automated unit and E2E integration test suites.
* **Deliverable**: Validated protocol compliance, rock-solid checkpoint creation, and functioning cross-session context recall.

---

### Person 3 — Product, Demo, Build & Documentation Lead
* **Primary Focus**: Developer experience, reproducibility, documentation, and the presentation story.
* **Owned Files**:
  * `agents/entire-agent-<target>/AGENT.md`
  * `agents/entire-agent-<target>/README.md`
  * `agents/entire-agent-<target>/mise.toml`
  * `agents/entire-agent-<target>/scripts/setup.sh`
  * `agents/entire-agent-<target>/scripts/demo.sh`
  * Demo repository & presentation deck.
* **Core Responsibilities**:
  * Configure `mise.toml` for automated building, linting, and testing across platforms.
  * Write clear, professional `README.md` and `AGENT.md` specifications.
  * Construct a realistic, highly visual demo scenario (e.g. implementing auth in Session 1, continuing with password reset in Session 2).
  * Continuously stress-test Person 1 and Person 2's code to find edge cases and bugs.
* **Deliverable**: A flawless setup and demo script that judges can clone, run, and understand within 3 minutes.

---

## 3. Fact vs Decision vs Assumption

### Confirmed Facts
* The team has 3 members.
* Splitting work by directory/layer prevents merge conflicts in Go monorepos.

### Decisions Made
* Person 1 handles incoming agent events $\rightarrow$ adapter.
* Person 2 handles adapter $\rightarrow$ Entire protocol & context recall.
* Person 3 handles packaging, build tooling, demo automation, and documentation.

### Assumptions
* All 3 teammates will coordinate PR reviews and integrate changes on a regular cadence.

### Things to Verify
* Ensure all team members have access to the forked repository with appropriate push/PR permissions.

---

## Related Notes
* [[07 - Repository Structure]] — Monorepo layout.
* [[08 - Development Workflow]] — Phased development workflow.
* [[10 - MVP]] — Milestone goals.
* [[11 - Demo Flow]] — Scripted demo workflow.
