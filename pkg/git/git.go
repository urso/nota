package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Scope defines which git files to include.
type Scope string

const (
	ScopeModified Scope = "modified" // staged + unstaged + untracked
	ScopeUnstaged Scope = "unstaged" // unstaged + untracked only
	ScopeStaged   Scope = "staged"   // staged only
	ScopeAll      Scope = "all"      // all tracked files
)

// ListFiles returns file paths based on the given git scope.
// dir is the working directory for git commands; if empty, uses os.Getwd().
// Returns paths relative to the git repo root.
func ListFiles(scope Scope, dir string) ([]string, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	root, err := RepoRoot(dir)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{})

	switch scope {
	case ScopeModified:
		// staged + unstaged + untracked
		if err := addGitFiles(set, root, "diff", "--name-only", "--cached"); err != nil {
			return nil, err
		}
		if err := addGitFiles(set, root, "diff", "--name-only"); err != nil {
			return nil, err
		}
		if err := addGitFiles(set, root, "ls-files", "--others", "--exclude-standard"); err != nil {
			return nil, err
		}
	case ScopeUnstaged:
		// unstaged + untracked
		if err := addGitFiles(set, root, "diff", "--name-only"); err != nil {
			return nil, err
		}
		if err := addGitFiles(set, root, "ls-files", "--others", "--exclude-standard"); err != nil {
			return nil, err
		}
	case ScopeStaged:
		// staged only
		if err := addGitFiles(set, root, "diff", "--name-only", "--cached"); err != nil {
			return nil, err
		}
	case ScopeAll:
		// all tracked + untracked files
		if err := addGitFiles(set, root, "ls-files"); err != nil {
			return nil, err
		}
		if err := addGitFiles(set, root, "ls-files", "--others", "--exclude-standard"); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown scope: %s", scope)
	}

	// Filter to existing files only, and validate paths stay under repo root.
	cleanRoot := filepath.Clean(root)
	var files []string
	for f := range set {
		fullPath := filepath.Join(root, f)
		// F5: Prevent path traversal — ensure resolved path is under repo root.
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(absPath, cleanRoot+string(filepath.Separator)) && absPath != cleanRoot {
			fmt.Fprintf(os.Stderr, "warning: skipping path outside repo root: %s\n", f)
			continue
		}
		if _, err := os.Stat(fullPath); err == nil {
			files = append(files, f)
		}
	}

	sort.Strings(files)
	return files, nil
}

// RepoRoot returns the git repo root directory.
func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// IsAvailable checks if git CLI is accessible.
func IsAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func addGitFiles(set map[string]struct{}, dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			set[line] = struct{}{}
		}
	}

	return nil
}

// HeadCommit returns the current HEAD commit SHA.
// dir is the working directory for git commands; if empty, uses os.Getwd().
func HeadCommit(dir string) (string, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}
