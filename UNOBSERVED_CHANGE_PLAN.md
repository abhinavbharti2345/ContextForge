# Unobserved Change Detection & Context Recovery

## Goal
Detect changes in Git commits that occurred outside observed agent sessions, and link them to historical context from Entire Checkpoints and Development Memory to highlight missing reasoning.

## User Review Required
> [!IMPORTANT]
> - Should this logic live as a new sub-command in `entire-agent-roo` (e.g., `entire-agent-roo detect-context-gap <commit-sha>`) for the MVP?
> - For the MVP, we will manually supply the unobserved `<commit-sha>` to the command rather than writing a full `pre-push` git hook. Does this align with the "smallest useful MVP" constraint?

## Existing Files & Functions to Reuse
- **`agents/entire-agent-roo/internal/roo/memory.go`**:
  - `loadAllMemories()`: Loads all `DevelopmentMemory` structures from the repository.
  - `DevelopmentMemory` struct: Holds `Intent`, `Decisions`, `Changes`, etc.
  - `cleanFile()`: Helps match file paths cleanly.
- **Git Integration**: We will use standard `git log` and `git diff-tree` shell commands to extract commit metadata (SHA, message, author) and changed files, avoiding the need to build a complex Git parsing library from scratch.

## Proposed Changes

### `agents/entire-agent-roo/internal/roo/context_gap.go`
#### [NEW] `context_gap.go`
This file will contain the core logic for detecting context gaps.
- `type ContextGap struct`: Matches the conceptual structure (CommitSHA, Message, ChangedFiles, RelatedCheckpoints, RelatedDecisions, MissingReason, SuggestedQuestion).
- `func DetectContextGap(commitSHA string) (ContextGap, error)`:
  1. Executes `git log` to get commit details (Message, Author, Timestamp).
  2. Executes `git diff-tree` to extract changed files.
  3. Checks if the commit is an observed agent checkpoint (e.g., by checking if the commit message or branch matches Entire's checkpoint pattern).
  4. If unobserved, loads all `DevelopmentMemory` via `loadAllMemories()`.
  5. Intersects the commit's changed files with the `Changes[].Path` in historical memories.
  6. Populates the `ContextGap` object with `RelatedDecisions` and a `SuggestedQuestion`.
- `func formatContextGap(gap ContextGap) string`: Produces the human-readable string required by the prompt.

### `agents/entire-agent-roo/cmd/entire-agent-roo/main.go`
#### [MODIFY] `main.go`
- Add a new subcommand `detect-context-gap` that accepts a commit SHA, calls `roo.DetectContextGap`, and prints the result to stdout.

## Data Flow
1. **Trigger**: Developer runs `entire-agent-roo detect-context-gap <sha>`.
2. **Git Parse**: Extract changed files and commit metadata.
3. **Classification**: Verify it lacks Entire checkpoint metadata (e.g. lack of Entire hook footprint).
4. **Graph/Memory Lookup**: Find matching files in the repo's Development Memory.
5. **Report**: Render the Context Gap and prompt the user for the missing reasoning.

## How we avoid duplicating Entire functionality
We do not build a new file-watcher or checkpoint creator. We rely entirely on the Git commit history that Entire already interfaces with, and we reuse the existing `DevelopmentMemory` extraction logic in `memory.go`. We use Git as the source of truth for the change, and the local `.entire/tmp` or global storage as the source of truth for history.

## Verification Plan

### Automated Tests
Add `context_gap_test.go` with mock scenarios:
- Observed commit -> no context gap.
- Unobserved commit with related history -> links to previous checkpoint.
- Unobserved commit with NO history -> explicitly outputs "No explicit historical reason was captured."

### Manual Verification
- Create `UNOBSERVED_CHANGE_DEMO.md` showing a step-by-step terminal execution of the MVP using a real commit in the repository.
