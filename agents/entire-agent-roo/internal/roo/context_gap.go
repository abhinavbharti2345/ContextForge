package roo

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

type ContextGap struct {
	CommitSHA          string
	CommitMessage      string
	Author             string
	Timestamp          string
	ChangedFiles       []string
	RelatedCheckpoints []string
	RelatedDecisions   []MemoryDecision
	RelatedSessions    []string
	MissingReason      string
	SuggestedQuestion  string
}

// DetectContextGap analyzes a commit to determine if it occurred outside an observed session,
// and cross-references it with existing Development Memory to build a ContextGap report.
func DetectContextGap(repoRoot, commitSHA string) (*ContextGap, error) {
	if repoRoot == "" {
		repoRoot = protocol.RepoRoot()
	}

	// 1. Extract commit metadata via git
	logCmd := exec.Command("git", "log", "-1", "--format=%H%n%an%n%aI%n%B", commitSHA)
	logCmd.Dir = repoRoot
	logOut, err := logCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch commit info: %w", err)
	}

	lines := strings.SplitN(string(logOut), "\n", 4)
	if len(lines) < 4 {
		return nil, fmt.Errorf("unexpected git log format")
	}

	sha := strings.TrimSpace(lines[0])
	author := strings.TrimSpace(lines[1])
	timestamp := strings.TrimSpace(lines[2])
	message := strings.TrimSpace(lines[3])

	// 2. Classify: Observed vs Unobserved
	// For this MVP, we consider a commit "observed" if it contains Entire checkpoint footprints in the message
	// Checkpoints often have titles or metadata. If it matches, we return early.
	// We'll also check if the commit was authored by an automated agent if there's a specific pattern.
	if strings.Contains(strings.ToLower(message), "entire checkpoint") || strings.Contains(strings.ToLower(message), "entire-agent") || strings.Contains(message, "## Intent") {
		return nil, nil // No context gap; it was an observed session
	}

	// 3. Extract changed files
	diffCmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", commitSHA)
	diffCmd.Dir = repoRoot
	diffOut, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch changed files: %w", err)
	}

	var changedFiles []string
	for _, f := range strings.Split(string(diffOut), "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			changedFiles = append(changedFiles, cleanFile(f))
		}
	}
	if len(changedFiles) == 0 {
		return nil, fmt.Errorf("no files changed in commit")
	}

	gap := &ContextGap{
		CommitSHA:     sha,
		CommitMessage: strings.Split(message, "\n")[0], // Just the subject line for the report
		Author:        author,
		Timestamp:     timestamp,
		ChangedFiles:  changedFiles,
	}

	// 4. Look up related history from Development Memory
	allMemories := loadAllMemories(repoRoot)
	
	// Sort chronologically so we refer to the latest decisions primarily
	sort.Slice(allMemories, func(i, j int) bool {
		return allMemories[i].Timestamp < allMemories[j].Timestamp
	})

	seenCheckpoints := map[string]bool{}
	seenSessions := map[string]bool{}
	seenDecisions := map[string]bool{}

	for _, m := range allMemories {
		// Does this memory intersect with our changed files?
		intersects := false
		for _, memChange := range m.Changes {
			for _, gitFile := range changedFiles {
				if memChange.Path == gitFile || strings.HasSuffix(memChange.Path, gitFile) || strings.HasSuffix(gitFile, memChange.Path) {
					intersects = true
					break
				}
			}
			if intersects {
				break
			}
		}

		if intersects {
			if m.CheckpointID != "" && !seenCheckpoints[m.CheckpointID] {
				gap.RelatedCheckpoints = append(gap.RelatedCheckpoints, m.CheckpointID)
				seenCheckpoints[m.CheckpointID] = true
			}
			if m.SessionID != "" && !seenSessions[m.SessionID] {
				gap.RelatedSessions = append(gap.RelatedSessions, m.SessionID)
				seenSessions[m.SessionID] = true
			}
			for _, dec := range m.Decisions {
				if !seenDecisions[dec.Statement] {
					gap.RelatedDecisions = append(gap.RelatedDecisions, dec)
					seenDecisions[dec.Statement] = true
				}
			}
		}
	}

	// 5. Generate prompt/Missing Reason
	if len(gap.RelatedCheckpoints) == 0 && len(gap.RelatedSessions) == 0 {
		gap.MissingReason = "No explicit historical reason was captured."
	} else {
		gap.MissingReason = "This change occurred outside an observed agent session.\nThe reason for the modification was not captured."
		
		ref := ""
		if len(gap.RelatedCheckpoints) > 0 {
			ref = "checkpoint " + gap.RelatedCheckpoints[0][:7]
		} else {
			ref = "session " + gap.RelatedSessions[0]
		}

		// Create the interactive prompt
		gap.SuggestedQuestion = fmt.Sprintf("I detected changes in commit %s that occurred outside an observed agent session.\n\nI found related history in %s.\n\nBefore continuing, can you explain why these changes were made?", 
			gap.CommitSHA[:7], ref)
	}

	return gap, nil
}

// FormatContextGap formats a ContextGap object into a human-readable text report.
func FormatContextGap(gap *ContextGap) string {
	var sb strings.Builder

	sb.WriteString("CONTEXT GAP DETECTED\n\n")
	sb.WriteString(fmt.Sprintf("Commit: %s\n", gap.CommitSHA[:7]))
	sb.WriteString(fmt.Sprintf("Message: %s\n\n", gap.CommitMessage))

	sb.WriteString("Changed files:\n")
	for _, f := range gap.ChangedFiles {
		sb.WriteString(fmt.Sprintf("  - %s\n", f))
	}
	sb.WriteString("\n")

	sb.WriteString("Related history:\n")
	if len(gap.RelatedCheckpoints) == 0 && len(gap.RelatedSessions) == 0 {
		sb.WriteString("  - None\n")
	} else {
		for _, cp := range gap.RelatedCheckpoints {
			sb.WriteString(fmt.Sprintf("  - Checkpoint %s\n", cp))
		}
		for _, dec := range gap.RelatedDecisions {
			sb.WriteString(fmt.Sprintf("  - Previous decision: %s\n", dec.Statement))
		}
		for _, ses := range gap.RelatedSessions {
			sb.WriteString(fmt.Sprintf("  - Related Roo session: %s\n", ses))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Missing context:\n")
	for _, line := range strings.Split(gap.MissingReason, "\n") {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
	sb.WriteString("\n")

	if gap.SuggestedQuestion != "" {
		sb.WriteString("Suggested question:\n")
		for _, line := range strings.Split(gap.SuggestedQuestion, "\n") {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
