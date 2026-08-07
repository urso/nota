package ghapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// PRDetector detects the PR associated with the current branch.
type PRDetector interface {
	DetectPR() (int, error)
}

// NoPRError indicates no PR is associated with the current branch.
type NoPRError struct{}

func (e *NoPRError) Error() string {
	return "no pull request associated with current branch"
}

// IsNoPR returns true if err indicates no PR exists for the branch.
func IsNoPR(err error) bool {
	_, ok := err.(*NoPRError)
	return ok
}

// ghPRDetector detects PRs by shelling out to `gh pr view`.
type ghPRDetector struct{}

// NewPRDetector creates a PRDetector that uses `gh pr view` for detection.
// Detection relies on gh's own branch-to-PR resolution, which handles forks
// and tracking branches better than an explicit --repo flag.
func NewPRDetector() PRDetector {
	return &ghPRDetector{}
}

func (d *ghPRDetector) DetectPR() (int, error) {
	cmd := exec.Command("gh", "pr", "view", "--json", "number")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return 0, &NoPRError{}
			}
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "no pull requests") {
				return 0, &NoPRError{}
			}
			return 0, fmt.Errorf("gh pr view failed: %s", stderr)
		}
		return 0, fmt.Errorf("running gh: %w", err)
	}

	var result struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return 0, fmt.Errorf("parsing gh output: %w", err)
	}

	if result.Number == 0 {
		return 0, &NoPRError{}
	}

	return result.Number, nil
}
