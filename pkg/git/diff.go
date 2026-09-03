package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Diff represents changes to a file in one commit.
type Diff struct {
	OldName string
	NewName string
	Deleted bool
	Created bool // File was created (didn't exist in old commit)
	Hunks   []Hunk
}

// Hunk represents a single diff hunk with its line changes.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type    byte // ' ' for context, '-' for removed, '+' for added
	Content string
}

// GetDiffs returns the sequence of diffs for a file from fromCommit to toCommit.
func GetDiffs(repoDir, fromCommit, toCommit, filePath string) ([]Diff, error) {
	cmd := exec.Command("git", "log", "--follow", "-p", "--reverse",
		"--pretty=format:%H", fmt.Sprintf("%s..%s", fromCommit, toCommit), "--", filePath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return nil, nil
		}
		return nil, fmt.Errorf("git log: %w", err)
	}

	return ParseDiffs(out)
}

// GetWorkingTreeDiffs returns diffs from a commit to the working tree.
func GetWorkingTreeDiffs(repoDir, fromCommit, filePath string) ([]Diff, error) {
	cmd := exec.Command("git", "diff", fromCommit, "--", filePath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return nil, nil
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}

	return ParseDiffs(out)
}

// ParseDiffs parses git log -p or git diff output into a sequence of Diff structs.
func ParseDiffs(data []byte) ([]Diff, error) {
	var diffs []Diff
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var current *Diff
	currentHunkIdx := -1
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git") {
			if current != nil {
				diffs = append(diffs, *current)
			}
			current = &Diff{}
			currentHunkIdx = -1
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "--- ") {
			if line == "--- /dev/null" {
				current.Created = true
			} else {
				// Handle various git diff prefixes: a/, c/ (worktree), or none
				name := line[4:] // Strip "--- "
				if len(name) >= 2 && name[1] == '/' {
					name = name[2:] // Strip single-char prefix like "a/" or "c/"
				}
				current.OldName = name
			}
			continue
		}

		if strings.HasPrefix(line, "+++ ") {
			if line == "+++ /dev/null" {
				current.Deleted = true
			} else {
				// Handle various git diff prefixes: b/, w/ (worktree), or none
				name := line[4:] // Strip "+++ "
				if len(name) >= 2 && name[1] == '/' {
					name = name[2:] // Strip single-char prefix like "b/" or "w/"
				}
				current.NewName = name
			}
			continue
		}

		if strings.HasPrefix(line, "@@") {
			hunk, err := ParseHunkHeader(line)
			if err != nil {
				continue
			}
			current.Hunks = append(current.Hunks, hunk)
			currentHunkIdx = len(current.Hunks) - 1
			continue
		}

		if currentHunkIdx >= 0 && len(line) > 0 {
			lineType := line[0]
			if lineType == ' ' || lineType == '-' || lineType == '+' {
				content := ""
				if len(line) > 1 {
					content = line[1:]
				}
				current.Hunks[currentHunkIdx].Lines = append(
					current.Hunks[currentHunkIdx].Lines,
					DiffLine{Type: lineType, Content: content},
				)
			}
		}
	}

	if current != nil {
		diffs = append(diffs, *current)
	}

	return diffs, scanner.Err()
}

// ParseHunkHeader parses a unified diff hunk header like "@@ -10,5 +12,7 @@".
func ParseHunkHeader(line string) (Hunk, error) {
	parts := strings.Split(line, " ")
	if len(parts) < 4 {
		return Hunk{}, fmt.Errorf("invalid hunk header: %s", line)
	}

	oldPart := strings.TrimPrefix(parts[1], "-")
	newPart := strings.TrimPrefix(parts[2], "+")

	oldStart, oldCount := parseRange(oldPart)
	newStart, newCount := parseRange(newPart)

	return Hunk{
		OldStart: oldStart,
		OldCount: oldCount,
		NewStart: newStart,
		NewCount: newCount,
	}, nil
}

func parseRange(s string) (start, count int) {
	parts := strings.SplitN(s, ",", 2)
	start, _ = strconv.Atoi(parts[0])
	if len(parts) == 2 {
		count, _ = strconv.Atoi(parts[1])
	} else {
		count = 1
	}
	return
}

// IsAncestorOf checks if ancestorCommit is an ancestor of descendantCommit.
func IsAncestorOf(repoDir, ancestorCommit, descendantCommit string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestorCommit, descendantCommit)
	cmd.Dir = repoDir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
	}
	return false, err
}

// MergeBase returns the best common ancestor of two commits.
func MergeBase(repoDir, commit1, commit2 string) (string, error) {
	cmd := exec.Command("git", "merge-base", commit1, commit2)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("git merge-base: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
