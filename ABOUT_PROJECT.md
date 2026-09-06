# ContextForge: The Intelligence Layer for AI Agents

---

## 1. What Entire is & How We Use It Currently
**Entire** is an advanced AI observability and telemetry daemon designed for the modern AI-assisted development workflow. In an ecosystem where developers use various AI coding assistants (like Roo Code, Claude Code, Cursor, and Copilot), Entire acts as the unified observation layer. It passively monitors these agents, standardizes their disparate logging formats into a single, cohesive protocol, and provides deep telemetry so engineering teams can audit exactly what their AI agents are doing.

Currently, we leverage Entire's `entire-agent-roo` adapter. This adapter is specifically built to watch Roo Code's background storage (specifically its global storage where it saves session transcripts, UI messages, and API conversation histories). By tailing these JSONL and JSON files, the adapter intercepts the raw, unstructured data of the agent's thought processes, tool usage, and terminal outputs.

## 2. What is Our Project About & Why Does It Exist?
Our project is called **ContextForge**. It fundamentally transforms Entire from a *passive logging and observability tool* into an *active, intelligent Development Memory Engine*.

**Why is this necessary if Entire already exists?** 
Current telemetry tools like Entire are excellent at telling you *what* happened (e.g., "Agent X modified `auth.go` at 2:00 PM"). However, they completely fail to capture *why* it happened (the architectural intent) and they fail to *stop* future mistakes. 
When humans code, they leave behind architectural decisions in pull requests, design docs, or team meetings. When AI agents code, they make hundreds of micro-decisions rapidly, apply the code, and then the context is lost forever. We call this **Multi-Agent Amnesia**. If a future agent is asked to optimize a caching layer, it might delete a critical rate-limiting function because it didn't know the rate-limiter depended on that cache. ContextForge exists to capture that lost reasoning, build a memory graph, and proactively prevent agents from destroying established architecture.

## 3. Cons & Missing Features of the Current Entire Adapter Solution
Before our ContextForge overhaul, the `entire-agent-roo` adapter suffered from three critical blind spots that severely limited its usefulness for enterprise scale:
1. **Brittle, Regex-Based Memory Parsing**: The adapter relied on hard-coded Regular Expressions (e.g., `(?i)(?:decided to|we choose to)...`) to extract agent reasoning from unstructured text. This was wildly inaccurate. If an agent phrased a decision naturally (e.g., *"Given the race condition, let's just stick with a mutex here"*), the regex would fail to capture it, resulting in massive context loss.
2. **Total Structural Blindness**: The adapter had zero awareness of the actual codebase. It treated files simply as isolated strings. It could not calculate dependencies or understand the structural "blast radius" of modifying a file.
3. **Passive, Read-Only Nature**: The adapter only logged actions *after* the damage was done. If an agent deleted a vital security file, the adapter would merely log the deletion. It lacked any mechanism to proactively intercept the agent and warn it about historical constraints *before* the code was overwritten.

## 4. How We Tackled Those Constraints & Our Own Features
We completely rewrote the core engine in three major phases, addressing each limitation with a robust, intelligent feature:

### Phase 1: The LLM Decision Extractor
- **The Problem Fixed**: Brittle regex failing to capture complex architectural intent.
- **Why we did it**: Natural language is too complex for regex. We needed a semantic engine capable of reading an agent's entire thought process and extracting the exact structural decision and its underlying reason.
- **What it is**: We built a dynamic AI client (`extractDecisionsWithLLM` in `memory.go`) directly into the adapter. When the adapter reads a transcript, it intercepts the agent's unstructured text and prompts a background LLM (via an OpenAI/Anthropic compatible endpoint) to parse it into structured JSON objects (Statement + Reason).
- **Solution Example**: 
  - *Agent thought*: "The user wants to speed up the database. I noticed we are opening a new connection every time. I'll implement a singleton connection pool to fix the overhead."
  - *Old Regex*: Fails.
  - *ContextForge LLM*: Extracts `{"statement": "Implement a singleton connection pool", "reason": "To fix database connection overhead"}`.

### Phase 2: Graphify Blast Radius Integration
- **The Problem Fixed**: Structural blindness and agents breaking undocumented dependencies.
- **Why we did it**: Agents modify files in isolation. We needed to give them a multi-dimensional view of the codebase so they know exactly what will break if they change a function.
- **What it is**: We integrated a native Abstract Syntax Tree (AST) parser that reads `graphify-out/graph.json`—a structural map of the user's project. We wrote a `calculateConsequences` algorithm that traverses this graph. When an agent touches a file, the algorithm instantly identifies every other file that structurally imports, calls, or depends on the target file.
- **Solution Example**: When an agent modifies `session_store.go`, ContextForge instantly appends `Impacts: auth_middleware.go, rate_limiter.go` to the memory logs, calculating the exact blast radius of the change.

### Phase 3: The Active Interceptor Hook
- **The Problem Fixed**: Passive logging failing to prevent catastrophic AI code modifications.
- **Why we did it**: Telemetry is useless if production is already down. We needed a tripwire to catch the agent *before* it executes a destructive tool.
- **What it is**: We implemented a "Soft Interceptor" pattern. First, we built the `entire-agent-roo impact` CLI command which queries the memory and blast radius. Second, we upgraded the watcher (`watcher.go`) to detect the exact millisecond an agent attempts to use a mutating tool and emit a `pre-tool-use` event. Third, we injected a strict workspace rule (`.agents/rules/interceptor.md`) that forces the agent to run our CLI command before making edits.
- **Solution Example**: The agent decides to rewrite `auth.go`. Before writing, our rule forces it to run `entire-agent-roo impact auth.go`. The CLI screams back: `🚨 SYSTEM WARNING: Modifying this will impact api_gateway.go`. The agent pauses, reads the warning, and asks the human developer for explicit permission instead of blindly breaking the API gateway.

## 5. Why Our Solution Solves a "Larger Than Life" Problem
We are not just building a log viewer—we are building **Consequence Awareness** and **Long-Term Memory** for AI. 
As companies scale from single developers using Copilot to fleets of autonomous AI agents managing enterprise mono-repos, the most terrifying risk is "Agentic Technical Debt." Autonomous agents are essentially junior developers working at 1000x speed. Without context, they will blindly overwrite hard-fought, undocumented architectural decisions (such as security workarounds, compliance regulations, or scalability hacks). 

By bridging the gap between historical memory and active execution, ContextForge elevates AI agents from "blind code monkeys" to "senior architects." We are solving the fundamental safety bottleneck preventing the mass adoption of autonomous enterprise AI.

## 6. Technical Dependencies & Tools Used
- **Go (Golang)**: Used for writing the core memory engine, the adapter logic, and the high-performance JSON log parsers. Chosen for its concurrency and speed when parsing massive session logs.
- **Entire Protocol**: The base telemetry standard we built on top of, providing the lifecycle hooks (`session-start`, `turn-start`, etc.).
- **Graphify CLI**: An external AST-generation tool we use to convert codebases into the `graph.json` structural map for our blast-radius link analysis.
- **LLM APIs**: We integrated HTTP clients that route to `OPENAI_BASE_URL` or `ANTHROPIC_MODEL` to perform our dynamic semantic extractions.
- **Git Hooks**: We utilize `post-commit` and `post-checkout` git hooks to automate the regeneration of the Graphify AST, ensuring our dependency graph is never stale.

## 7. Main Folders & Files Responsible for the Magic
- **`agents/entire-agent-roo/internal/roo/memory.go`**: **The Brain.** This file is the crown jewel of our project. It houses the `extractDecisionsWithLLM` logic, the `calculateConsequences` graph parser, and the `QueryImpact` CLI system. It is responsible for bridging raw logs into intelligent memory.
- **`agents/entire-agent-roo/internal/roo/watcher.go`**: **The Nervous System.** A highly optimized polling loop that watches Roo Code's global storage. We modified this to track tool indices (`DetectNewToolCall`) so we can emit `pre-tool-use` hooks the moment an agent tries to modify a file.
- **`agents/entire-agent-roo/internal/roo/context_gap.go`**: **The Auditor.** This file compares the git diffs against the LLM memories to find "Context Gaps" (code that was written but lacks architectural reasoning). We fixed major substring matching bugs here.
- **`.agents/rules/interceptor.md`**: **The Guardrail.** The workspace instruction document that enforces the Soft Interceptor pattern on the AI agent, dictating that it must run our `impact` command before acting.

## 8. Brief Tutorial: How to Show This to the Judges
To effectively demonstrate ContextForge to the judges, follow this sequence:

### Step A: Initialize the System
1. **Enable the adapter**: Show how easy it is to attach our memory engine to a workspace.
   ```bash
   entire enable --agent roo
   ```
2. **Show the baseline memory**: Query the historical context we've already generated for our own engine.
   ```bash
   entire-agent-roo why internal/roo/memory.go
   ```
   *(Point out how the output lists explicit LLM-extracted reasons, not just raw text).*

### Step B: The Interceptor Demo (The "Wow" Moment)
Explain to the judges: *"We are going to simulate a rogue AI agent trying to delete a critical file."*
1. **Trigger the Tripwire**: Run the new Impact command manually to show what the agent sees right before it edits a file:
   ```bash
   entire-agent-roo impact internal/roo/memory.go
   ```
2. **Explain the Output**: The CLI will aggressively print a `🚨 SYSTEM WARNING`. Point out two things to the judges:
   - **The Historical Decisions**: It lists exactly *why* the file exists based on past agent sessions.
   - **The Blast Radius**: It lists the exact files that will break if the edit goes through.
3. **The Payoff**: Explain that because of our `.agents/rules/interceptor.md` rule, the AI agent is forced to read this warning and pause execution to ask the human for permission, effectively saving production from an AI hallucination.

## 9. Other Important Factors to Highlight
- **The Zero-Hallucination Rule**: Emphasize that our memory engine is strictly designed to *never* hallucinate context. We heavily prompted the LLM extractor to leave reasons blank if an explicit architectural choice isn't found in the transcript. We prioritize accuracy over volume.
- **Graceful Degradation / Fallbacks**: Mention that if the user's LLM API key expires or the endpoint goes down, ContextForge does not crash. It seamlessly falls back to the legacy regex parser to guarantee stability during live enterprise environments.

## 10. Edge Cases Addressed (Preparedness for Expert Judges)
Expert judges will look for holes in our architecture. Here is how we solved the three biggest edge cases:
- **The "Path Substring" Edge Case**: A major bug in the original adapter was that a query for `session.go` would use `strings.HasSuffix` and falsely match `auth/session.go`, injecting hallucinated history into the agent's context. 
  - *Our Fix:* We implemented strict path normalization and exact equality matching (`==`) across the entire engine to prevent context pollution.
- **The "Stale Graph" Edge Case**: If a developer refactors the codebase but the AST graph isn't updated, the blast radius calculation becomes dangerously stale. 
  - *Our Fix:* We implemented automated git hooks (`post-commit`, `post-checkout`) that rebuild the Graphify AST silently in the background after every code change.
- **The "Storage-Driven Interceptor" Limitation**: Roo Code uses a passive global storage log. Because we are an external Go daemon, we cannot natively freeze the internal Node.js process of the Roo Code extension. 
  - *Our Fix:* We engineered a "Soft Interceptor." By injecting the strict `.agents/rules/interceptor.md` instruction file into the workspace, we force the LLM itself to respect our CLI output and voluntarily pause its own execution loop.
