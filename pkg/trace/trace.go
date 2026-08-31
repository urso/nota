// Package trace implements diff-based line tracking through git commit history.
package trace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/thread"
)

// Re-export parsing functions for tests
var (
	parseDiffs      = git.ParseDiffs
	parseHunkHeader = git.ParseHunkHeader
)

// Result represents the outcome of tracing an anchor through git history.
type Result struct {
	Anchor   thread.Anchor // New anchor with updated position
	Outdated bool          // True if line was deleted or content changed
}

// TraceAnchor traces an anchor from its commit to targetCommit, returning a new anchor.
// If the line was deleted or the content changed, Outdated is set to true.
// repoDir is the git repository root directory.
func TraceAnchor(repoDir string, anchor thread.Anchor, targetCommit string) (Result, error) {
	if anchor.Commit == "" {
		return Result{Anchor: anchor}, nil
	}
	if anchor.Commit == targetCommit {
		return Result{Anchor: anchor}, nil
	}

	// Check if targetCommit is ancestor of anchor.Commit (revert case)
	isAncestor, err := isAncestorOf(repoDir, targetCommit, anchor.Commit)
	if err != nil {
		return Result{}, fmt.Errorf("checking ancestry: %w", err)
	}
	if isAncestor {
		// Target is before anchor — can't trace backwards in this call
		// Caller should handle backtracking through anchor history
		return Result{
			Anchor:   anchor,
			Outdated: true,
		}, nil
	}

	// Get file path relative to repo root
	filePath := anchor.File
	if filepath.IsAbs(filePath) {
		rel, err := filepath.Rel(repoDir, filePath)
		if err == nil {
			filePath = rel
		}
	}

	// Get diff output
	diffs, err := getDiffs(repoDir, anchor.Commit, targetCommit, filePath)
	if err != nil {
		return Result{}, fmt.Errorf("getting diffs: %w", err)
	}

	// Track line through diffs
	currentLine := anchor.Line
	currentFile := filePath

	for _, d := range diffs {
		if d.Deleted {
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       currentLine,
					Commit:     targetCommit,
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}, nil
		}

		if d.Created {
			// File was created after anchor commit — anchor is invalid
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       anchor.Line,
					Commit:     targetCommit,
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}, nil
		}

		if d.NewName != "" {
			currentFile = d.NewName
		}

		currentLine = applyHunks(currentLine, d.Hunks)
		if currentLine == 0 {
			// Line was deleted
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       anchor.Line, // Keep original for reference
					Commit:     targetCommit,
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}, nil
		}
	}

	// Verify content hash if available
	newAnchor := thread.Anchor{
		File:       currentFile,
		Line:       currentLine,
		Commit:     targetCommit,
		TracedFrom: anchor.Commit,
	}

	if anchor.ContentHash != "" {
		actualHash, err := computeContentHashAtCommit(repoDir, currentFile, currentLine, targetCommit)
		if err == nil && actualHash != anchor.ContentHash {
			newAnchor.Outdated = true
			return Result{Anchor: newAnchor, Outdated: true}, nil
		}
		newAnchor.ContentHash = actualHash
	}

	return Result{Anchor: newAnchor}, nil
}

// Type aliases for git package types used throughout trace.
type (
	Diff     = git.Diff
	Hunk     = git.Hunk
	DiffLine = git.DiffLine
	GitOps   = git.Ops
)

// NewGitOps creates a GitOps that calls git directly without caching.
func NewGitOps(repoDir string) GitOps {
	return git.NewOps(repoDir)
}

// NewCachedGitOps creates a GitOps that caches results from the underlying GitOps.
func NewCachedGitOps(inner GitOps) GitOps {
	return git.NewCachedOps(inner)
}

// getDiffs returns the sequence of diffs for a file from fromCommit to toCommit.
func getDiffs(repoDir, fromCommit, toCommit, filePath string) ([]Diff, error) {
	return git.GetDiffs(repoDir, fromCommit, toCommit, filePath)
}

// applyHunks traces a line number through a sequence of hunks.
// Returns the new line number, or 0 if the line was deleted.
//
// Uses the actual diff lines to track exactly where each old line maps to.
func applyHunks(line int, hunks []Hunk) int {
	offset := 0 // Cumulative offset from previous hunks

	for _, h := range hunks {
		hunkOldEnd := h.OldStart + h.OldCount

		if line < h.OldStart {
			// Line is before this hunk
			return line + offset
		}

		if line >= hunkOldEnd {
			// Line is after this hunk, accumulate offset
			offset += (h.NewCount - h.OldCount)
			continue
		}

		// Line is within this hunk - trace through actual diff lines
		oldLineNum := h.OldStart
		newLineNum := h.NewStart
		for _, dl := range h.Lines {
			switch dl.Type {
			case ' ': // Context line - both old and new advance
				if oldLineNum == line {
					return newLineNum + offset
				}
				oldLineNum++
				newLineNum++
			case '-': // Removed line - only old advances
				if oldLineNum == line {
					return 0 // Line was deleted
				}
				oldLineNum++
			case '+': // Added line - only new advances
				newLineNum++
			}
		}

		// Fallback if we didn't find the line in diff lines
		// (shouldn't happen for well-formed diffs)
		return line + offset + (h.NewCount - h.OldCount)
	}

	return line + offset
}

// isAncestorOf checks if ancestorCommit is an ancestor of descendantCommit.
func isAncestorOf(repoDir, ancestorCommit, descendantCommit string) (bool, error) {
	return git.IsAncestorOf(repoDir, ancestorCommit, descendantCommit)
}

// computeContentHashAtCommit computes the content hash for a line at a specific commit.
func computeContentHashAtCommit(repoDir, filePath string, line int, commit string) (string, error) {
	var cmd *exec.Cmd
	if commit == "" {
		// Working tree
		fullPath := filepath.Join(repoDir, filePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return "", err
		}
		return computeContentHash(content, line), nil
	}

	cmd = exec.Command("git", "show", fmt.Sprintf("%s:%s", commit, filePath))
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return computeContentHash(out, line), nil
}

// computeContentHash returns SHA256 hash of the line content, first 8 hex chars.
func computeContentHash(content []byte, line int) string {
	lines := bytes.Split(content, []byte("\n"))
	if line < 1 || line > len(lines) {
		return ""
	}
	lineContent := bytes.TrimSpace(lines[line-1])
	hash := sha256.Sum256(lineContent)
	return hex.EncodeToString(hash[:4])
}

// ContentHash computes the content hash for a line in a file.
func ContentHash(filePath string, line int) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return computeContentHash(content, line), nil
}

// TraceToWorkingTree traces an anchor from its commit to the current working tree state.
// Returns an anchor with Commit="" representing uncommitted changes.
func TraceToWorkingTree(repoDir string, anchor thread.Anchor) (Result, error) {
	if anchor.Commit == "" {
		// Normalize path to relative for consistency
		filePath := anchor.File
		if filepath.IsAbs(filePath) {
			if rel, err := filepath.Rel(repoDir, filePath); err == nil {
				anchor.File = rel
			}
		}
		return Result{Anchor: anchor}, nil
	}

	filePath := anchor.File
	if filepath.IsAbs(filePath) {
		rel, err := filepath.Rel(repoDir, filePath)
		if err == nil {
			filePath = rel
		}
	}

	// Get diff from anchor's commit to working tree
	diffs, err := getWorkingTreeDiffs(repoDir, anchor.Commit, filePath)
	if err != nil {
		return Result{}, fmt.Errorf("getting working tree diffs: %w", err)
	}

	currentLine := anchor.Line
	currentFile := filePath

	for _, d := range diffs {
		if d.Deleted {
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       currentLine,
					Commit:     "",
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}, nil
		}

		if d.Created {
			// File was created after anchor commit — anchor is invalid
			// Keep original line but mark outdated
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       anchor.Line,
					Commit:     "",
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}, nil
		}

		if d.NewName != "" {
			currentFile = d.NewName
		}

		currentLine = applyHunks(currentLine, d.Hunks)
		if currentLine == 0 {
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       anchor.Line,
					Commit:     "",
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}, nil
		}
	}

	newAnchor := thread.Anchor{
		File:       currentFile,
		Line:       currentLine,
		Commit:     "",
		TracedFrom: anchor.Commit,
	}

	if anchor.ContentHash != "" {
		actualHash, err := computeContentHashAtCommit(repoDir, currentFile, currentLine, "")
		if err == nil && actualHash != anchor.ContentHash {
			newAnchor.Outdated = true
			return Result{Anchor: newAnchor, Outdated: true}, nil
		}
		newAnchor.ContentHash = actualHash
	}

	return Result{Anchor: newAnchor}, nil
}

// getWorkingTreeDiffs returns diffs from a commit to the working tree.
func getWorkingTreeDiffs(repoDir, fromCommit, filePath string) ([]Diff, error) {
	return git.GetWorkingTreeDiffs(repoDir, fromCommit, filePath)
}

// TraceWithBacktrack traces anchors to a target commit, handling reverts by backtracking
// through anchor history to find a valid starting point.
// Returns the new anchor and whether it's outdated.
func TraceWithBacktrack(repoDir string, anchors []thread.Anchor, targetCommit string) (Result, error) {
	if len(anchors) == 0 {
		return Result{}, fmt.Errorf("no anchors to trace")
	}

	current := anchors[len(anchors)-1]

	// Fast path: already at target or no commit
	if current.Commit == "" || current.Commit == targetCommit {
		return Result{Anchor: current}, nil
	}

	// Check if we can trace forward directly
	isAncestor, err := isAncestorOf(repoDir, current.Commit, targetCommit)
	if err != nil {
		return Result{}, fmt.Errorf("checking ancestry: %w", err)
	}

	if isAncestor {
		return TraceAnchor(repoDir, current, targetCommit)
	}

	// Target is not a descendant of current anchor's commit (revert case).
	// Walk anchors in reverse to find one whose commit is an ancestor of target.
	for i := len(anchors) - 1; i >= 0; i-- {
		a := anchors[i]
		if a.Commit == "" {
			continue
		}

		if a.Commit == targetCommit {
			return Result{Anchor: a}, nil
		}

		isAnc, err := isAncestorOf(repoDir, a.Commit, targetCommit)
		if err != nil {
			continue
		}

		if isAnc {
			return TraceAnchor(repoDir, a, targetCommit)
		}
	}

	// No valid starting point found
	return Result{
		Anchor: thread.Anchor{
			File:       current.File,
			Line:       current.Line,
			Commit:     targetCommit,
			TracedFrom: current.Commit,
			Outdated:   true,
		},
		Outdated: true,
	}, nil
}

// BatchResult holds results for a single anchor in a batch trace operation.
type BatchResult struct {
	Index  int
	Result Result
	Error  error
}

// TraceAnchors traces multiple anchors to a target commit, sharing git log output
// for anchors in the same file. Returns results in the same order as input anchors.
func TraceAnchors(repoDir string, anchors []thread.Anchor, targetCommit string) []BatchResult {
	git := NewCachedGitOps(NewGitOps(repoDir))
	return TraceAnchorsWithGit(git, anchors, targetCommit)
}

// TraceAnchorsWithGit traces multiple anchors using the provided GitOps.
// Use NewCachedGitOps to share git results across multiple calls.
func TraceAnchorsWithGit(git GitOps, anchors []thread.Anchor, targetCommit string) []BatchResult {
	results := make([]BatchResult, len(anchors))

	// Group anchors by file
	byFile := make(map[string][]anchorRef)

	for i, a := range anchors {
		if a.Commit == "" || a.Commit == targetCommit || a.Outdated {
			results[i] = BatchResult{Index: i, Result: Result{Anchor: a}}
			continue
		}

		filePath := a.File
		if filepath.IsAbs(filePath) {
			if rel, err := filepath.Rel(git.RepoDir(), filePath); err == nil {
				filePath = rel
			}
		}
		byFile[filePath] = append(byFile[filePath], anchorRef{index: i, anchor: a})
	}

	// Process each file group
	for filePath, refs := range byFile {
		// Find the best starting commit for fetching diffs.
		// We need a commit that is an ancestor of all anchor commits AND targetCommit.
		earliestCommit, needsMergeBase := findEarliestCommit(git, refs, targetCommit)

		if needsMergeBase {
			// Anchors are on divergent branches (e.g., after rebase).
			// Find common ancestor with target to get complete diff history.
			base, err := git.MergeBase(earliestCommit, targetCommit)
			if err == nil && base != "" {
				earliestCommit = base
			}
		}

		// Get diffs once for all anchors in this file
		diffs, err := git.GetDiffs(earliestCommit, targetCommit, filePath)
		if err != nil {
			for _, ref := range refs {
				results[ref.index] = BatchResult{Index: ref.index, Error: err}
			}
			continue
		}

		// Apply diffs to each anchor
		for _, ref := range refs {
			result := traceWithDiffsUsingGit(git, ref.anchor, targetCommit, diffs, filePath)
			results[ref.index] = BatchResult{Index: ref.index, Result: result}
		}
	}

	return results
}

// findEarliestCommit finds the earliest commit among anchors that can serve as
// a starting point for diff fetching. Returns the commit and whether merge-base
// is needed (true if commits are on divergent branches).
func findEarliestCommit(git GitOps, refs []anchorRef, targetCommit string) (string, bool) {
	if len(refs) == 0 {
		return "", false
	}

	earliest := refs[0].anchor.Commit
	needsMergeBase := false

	// Check if earliest is ancestor of target (normal case)
	isAnc, err := git.IsAncestorOf(earliest, targetCommit)
	if err == nil && !isAnc {
		needsMergeBase = true
	}

	for _, ref := range refs[1:] {
		commit := ref.anchor.Commit

		// Check if this commit is earlier than current earliest
		isAnc, err := git.IsAncestorOf(commit, earliest)
		if err == nil && isAnc {
			earliest = commit
			continue
		}

		// Check if earliest is ancestor of this commit
		isAnc, err = git.IsAncestorOf(earliest, commit)
		if err == nil && isAnc {
			// earliest is already earlier, keep it
			continue
		}

		// Neither is ancestor of the other — divergent branches
		needsMergeBase = true
	}

	return earliest, needsMergeBase
}

type anchorRef struct {
	index  int
	anchor thread.Anchor
}

// traceWithDiffsUsingGit traces an anchor using pre-fetched diffs and GitOps.
func traceWithDiffsUsingGit(git GitOps, anchor thread.Anchor, targetCommit string, diffs []Diff, filePath string) Result {
	// Check ancestry (target before anchor = revert case)
	isAncestor, err := git.IsAncestorOf(targetCommit, anchor.Commit)
	if err == nil && isAncestor {
		return Result{Anchor: anchor, Outdated: true}
	}

	// Use all diffs — they were fetched from the common ancestor
	relevantDiffs := diffs

	currentLine := anchor.Line
	currentFile := filePath

	for _, d := range relevantDiffs {
		if d.Deleted {
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       currentLine,
					Commit:     targetCommit,
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}
		}

		if d.Created {
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       anchor.Line,
					Commit:     targetCommit,
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}
		}

		if d.NewName != "" {
			currentFile = d.NewName
		}

		currentLine = applyHunks(currentLine, d.Hunks)
		if currentLine == 0 {
			return Result{
				Anchor: thread.Anchor{
					File:       currentFile,
					Line:       anchor.Line,
					Commit:     targetCommit,
					TracedFrom: anchor.Commit,
					Outdated:   true,
				},
				Outdated: true,
			}
		}
	}

	newAnchor := thread.Anchor{
		File:       currentFile,
		Line:       currentLine,
		Commit:     targetCommit,
		TracedFrom: anchor.Commit,
	}

	if anchor.ContentHash != "" {
		actualHash, err := computeContentHashAtCommit(git.RepoDir(), currentFile, currentLine, targetCommit)
		if err == nil && actualHash != anchor.ContentHash {
			newAnchor.Outdated = true
			return Result{Anchor: newAnchor, Outdated: true}
		}
		newAnchor.ContentHash = actualHash
	}

	return Result{Anchor: newAnchor}
}
