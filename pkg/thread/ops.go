package thread

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gofrs/flock"
	"github.com/urso/nota/pkg/git"
)

// lockThread acquires an exclusive lock on a thread file.
// It creates a .lock file alongside the thread and returns an unlock function.
// The unlock function should be deferred to release the lock.
func lockThread(dir, threadID string) (unlock func() error, err error) {
	// Create a lock file path based on thread ID
	lockPath := filepath.Join(dir, threadID+".lock")

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	fl := flock.New(lockPath)
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("acquiring lock for thread %s: %w", threadID, err)
	}

	return func() error {
		defer os.Remove(lockPath) // Clean up lock file
		return fl.Unlock()
	}, nil
}

var localIDPattern = regexp.MustCompile(`^l:[0-9a-f]{16}$`)
var githubIDPattern = regexp.MustCompile(`^gh:\d+$`)

// ValidateID checks if a thread or comment ID has a valid format.
func ValidateID(id string) error {
	if localIDPattern.MatchString(id) || githubIDPattern.MatchString(id) {
		return nil
	}
	return fmt.Errorf("invalid thread ID format, expected l:uuid or gh:id")
}

// Filename returns the filename for a thread based on its ID and group.
func Filename(t *Thread) string {
	suffix := strings.TrimPrefix(t.ID, "l:")
	if t.Group != "" {
		return t.Group + "-" + suffix + ".xml"
	}
	return suffix + ".xml"
}

// ParseAnchor parses a "file:line" string into an Anchor.
func ParseAnchor(s string) (*Anchor, error) {
	idx := strings.LastIndexByte(s, ':')
	if idx == -1 {
		return nil, fmt.Errorf("invalid anchor format %q: expected file:line", s)
	}

	file := s[:idx]
	lineStr := s[idx+1:]

	line, err := strconv.Atoi(lineStr)
	if err != nil || line < 1 {
		return nil, fmt.Errorf("invalid anchor format %q: line must be a positive integer", s)
	}

	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("anchor file not found: %s", file)
	}

	anchor := &Anchor{
		File: file,
		Line: line,
	}

	if commit, err := git.HeadCommit(""); err == nil {
		anchor.Commit = commit
	}

	return anchor, nil
}

// CreateOpts holds options for creating a thread.
type CreateOpts struct {
	Message string
	Goal    string
	Group   string
	Parent  string
	Anchor  string
	Author  string
}

// Create creates a new thread and writes it to the given directory.
func Create(dir string, opts CreateOpts) (*Thread, error) {
	if strings.TrimSpace(opts.Message) == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	if opts.Goal != "" && !ValidGoal(opts.Goal) {
		return nil, fmt.Errorf("invalid goal %q: must be one of %s", opts.Goal, strings.Join(GoalValues, ", "))
	}

	if opts.Parent != "" {
		if err := ValidateID(opts.Parent); err != nil {
			return nil, fmt.Errorf("invalid parent ID: %w", err)
		}
	}

	t := NewThread("open", opts.Goal)
	t.Group = opts.Group

	if opts.Parent != "" {
		t.Parent = &Parent{Ref: opts.Parent}
	}

	if opts.Anchor != "" {
		anchor, err := ParseAnchor(opts.Anchor)
		if err != nil {
			return nil, err
		}
		t.Anchor = anchor
	}

	author := opts.Author
	if author == "" {
		author = git.UserName()
	}
	comment := NewComment(author, opts.Message)
	t.Comments = append(t.Comments, comment)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	path := filepath.Join(dir, Filename(t))
	if err := WriteThread(path, t); err != nil {
		return nil, err
	}

	return t, nil
}

// AddCommentOpts holds options for adding a comment.
type AddCommentOpts struct {
	Message string
	Author  string
	Local   bool
	ReplyTo string
	Anchor  string
}

// AddComment adds a comment to an existing thread.
func AddComment(dir, threadID string, opts AddCommentOpts) (*Comment, error) {
	if err := ValidateID(threadID); err != nil {
		return nil, err
	}

	if strings.TrimSpace(opts.Message) == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	if opts.ReplyTo != "" {
		if err := ValidateID(opts.ReplyTo); err != nil {
			return nil, fmt.Errorf("invalid reply-to ID: %w", err)
		}
	}

	// Acquire lock before read-modify-write
	unlock, err := lockThread(dir, threadID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	info, err := FindThread(dir, threadID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}

	author := opts.Author
	if author == "" {
		author = git.UserName()
	}
	comment := NewComment(author, opts.Message)
	if opts.Local {
		comment.Visibility = "local"
	}
	if opts.ReplyTo != "" {
		comment.ReplyTo = &ReplyTo{Ref: opts.ReplyTo}
	}
	if opts.Anchor != "" {
		anchor, err := ParseAnchor(opts.Anchor)
		if err != nil {
			return nil, err
		}
		comment.Anchor = anchor
	}

	info.Thread.Comments = append(info.Thread.Comments, comment)

	if err := WriteThread(info.Path, info.Thread); err != nil {
		return nil, err
	}

	return &comment, nil
}

// UpdateStatus updates a thread's status.
func UpdateStatus(dir, threadID, status string) error {
	if err := ValidateID(threadID); err != nil {
		return err
	}

	// Acquire lock before read-modify-write
	unlock, err := lockThread(dir, threadID)
	if err != nil {
		return err
	}
	defer unlock()

	info, err := FindThread(dir, threadID)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("thread not found: %s", threadID)
	}

	info.Thread.Status = status
	return WriteThread(info.Path, info.Thread)
}

// UpdateGoal updates a thread's goal.
func UpdateGoal(dir, threadID, goal string) error {
	if err := ValidateID(threadID); err != nil {
		return err
	}

	if !ValidGoal(goal) {
		return fmt.Errorf("invalid goal %q: must be one of %s", goal, strings.Join(GoalValues, ", "))
	}

	// Acquire lock before read-modify-write
	unlock, err := lockThread(dir, threadID)
	if err != nil {
		return err
	}
	defer unlock()

	info, err := FindThread(dir, threadID)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("thread not found: %s", threadID)
	}

	info.Thread.Goal = goal
	return WriteThread(info.Path, info.Thread)
}

// SpawnOpts holds options for spawning a child thread.
type SpawnOpts struct {
	Message string
	Goal    string
	Group   string
	Author  string
}

// Spawn creates a child thread linked to a parent.
func Spawn(dir, parentID string, opts SpawnOpts) (*Thread, error) {
	if err := ValidateID(parentID); err != nil {
		return nil, fmt.Errorf("invalid parent ID: %w", err)
	}

	if strings.TrimSpace(opts.Message) == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	if opts.Goal != "" && !ValidGoal(opts.Goal) {
		return nil, fmt.Errorf("invalid goal %q: must be one of %s", opts.Goal, strings.Join(GoalValues, ", "))
	}

	// Acquire lock on parent before reading it
	unlock, err := lockThread(dir, parentID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	parentInfo, err := FindThread(dir, parentID)
	if err != nil {
		return nil, err
	}
	if parentInfo == nil {
		return nil, fmt.Errorf("thread not found: %s", parentID)
	}

	child := NewThread("open", opts.Goal)
	child.Parent = &Parent{Ref: parentID}

	if opts.Group != "" {
		child.Group = opts.Group
	} else if parentInfo.Thread.Group != "" {
		child.Group = parentInfo.Thread.Group
	}

	author := opts.Author
	if author == "" {
		author = git.UserName()
	}
	comment := NewComment(author, opts.Message)
	child.Comments = append(child.Comments, comment)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	path := filepath.Join(dir, Filename(child))
	if err := WriteThread(path, child); err != nil {
		return nil, err
	}

	return child, nil
}

// AddDependencies adds blocker IDs to a thread's depends-on attribute.
func AddDependencies(dir, threadID string, blockerIDs []string) (string, error) {
	if err := ValidateID(threadID); err != nil {
		return "", err
	}

	for _, blockerID := range blockerIDs {
		if err := ValidateID(blockerID); err != nil {
			return "", fmt.Errorf("invalid blocker ID %q: %w", blockerID, err)
		}
	}

	// Acquire lock before read-modify-write
	unlock, err := lockThread(dir, threadID)
	if err != nil {
		return "", err
	}
	defer unlock()

	info, err := FindThread(dir, threadID)
	if err != nil {
		return "", err
	}
	if info == nil {
		return "", fmt.Errorf("thread not found: %s", threadID)
	}

	existing := ParseDependsOn(info.Thread.DependsOn)
	for _, blockerID := range blockerIDs {
		if !slices.Contains(existing, blockerID) {
			existing = append(existing, blockerID)
		}
	}
	info.Thread.DependsOn = strings.Join(existing, ",")

	if err := WriteThread(info.Path, info.Thread); err != nil {
		return "", err
	}

	return info.Thread.DependsOn, nil
}

// RemoveDependencies removes blocker IDs from a thread's depends-on attribute.
func RemoveDependencies(dir, threadID string, blockerIDs []string) (string, error) {
	if err := ValidateID(threadID); err != nil {
		return "", err
	}

	for _, blockerID := range blockerIDs {
		if err := ValidateID(blockerID); err != nil {
			return "", fmt.Errorf("invalid blocker ID %q: %w", blockerID, err)
		}
	}

	// Acquire lock before read-modify-write
	unlock, err := lockThread(dir, threadID)
	if err != nil {
		return "", err
	}
	defer unlock()

	info, err := FindThread(dir, threadID)
	if err != nil {
		return "", err
	}
	if info == nil {
		return "", fmt.Errorf("thread not found: %s", threadID)
	}

	existing := ParseDependsOn(info.Thread.DependsOn)
	var filtered []string
	for _, id := range existing {
		if !slices.Contains(blockerIDs, id) {
			filtered = append(filtered, id)
		}
	}
	info.Thread.DependsOn = strings.Join(filtered, ",")

	if err := WriteThread(info.Path, info.Thread); err != nil {
		return "", err
	}

	return info.Thread.DependsOn, nil
}

// ParseDependsOn parses a comma-separated depends-on string.
func ParseDependsOn(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
