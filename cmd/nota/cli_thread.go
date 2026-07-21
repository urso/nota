package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/thread"
)

// ThreadCmd groups thread management commands.
type ThreadCmd struct {
	List    ThreadListCmd    `cmd:"" help:"List threads"`
	Show    ThreadShowCmd    `cmd:"" help:"Show a thread"`
	Create  ThreadCreateCmd  `cmd:"" help:"Create a new thread"`
	Resolve ThreadResolveCmd `cmd:"" help:"Mark thread as resolved"`
	Wontfix ThreadWontfixCmd `cmd:"" help:"Mark thread as wontfix"`
	Reopen  ThreadReopenCmd  `cmd:"" help:"Reopen a thread"`
	Goal    ThreadGoalCmd    `cmd:"" help:"Update thread goal"`
}

// ThreadListCmd lists threads with optional filters.
type ThreadListCmd struct {
	Status string `help:"Filter by status (open, resolved, wontfix)"`
	Goal   string `help:"Filter by goal"`
	Group  string `help:"Filter by group"`
	Tag    string `help:"Filter by tag"`
}

func (c *ThreadListCmd) Run() error {
	dir := filepath.Join(projectRoot(), ".nota")
	filter := thread.ThreadFilter{
		Status: c.Status,
		Goal:   c.Goal,
		Group:  c.Group,
		Tag:    c.Tag,
	}

	threads, err := thread.ListThreads(dir, filter)
	if err != nil {
		return err
	}

	for _, info := range threads {
		t := info.Thread
		title := thread.ThreadTitle(t)
		fmt.Printf("%s\t%s\t%s\t%s\n", t.ID, t.Status, t.Goal, title)
	}

	return nil
}

// ThreadShowCmd displays a thread.
type ThreadShowCmd struct {
	ID   string `arg:"" required:"" help:"Thread ID"`
	Raw  bool   `help:"Output XML source"`
	JSON bool   `name:"json" help:"Output JSON"`
}

func (c *ThreadShowCmd) Run() error {
	dir := filepath.Join(projectRoot(), ".nota")

	info, err := thread.FindThread(dir, c.ID)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("thread not found: %s", c.ID)
	}

	if c.Raw {
		data, err := os.ReadFile(info.Path)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}

	if c.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info.Thread)
	}

	return renderThread(info.Thread)
}

func renderThread(t *thread.Thread) error {
	fmt.Printf("# Thread %s\n\n", t.ID)
	fmt.Printf("Status: %s", t.Status)
	if t.Goal != "" {
		fmt.Printf("  Goal: %s", t.Goal)
	}
	if t.Group != "" {
		fmt.Printf("  Group: %s", t.Group)
	}
	fmt.Println()

	if t.Anchor != nil {
		fmt.Printf("Anchor: %s:%d", t.Anchor.File, t.Anchor.Line)
		if t.Anchor.Commit != "" {
			fmt.Printf(" @ %s", t.Anchor.Commit[:min(7, len(t.Anchor.Commit))])
		}
		fmt.Println()
	}

	fmt.Println()

	for i, c := range t.Comments {
		if i > 0 {
			fmt.Println("---")
		}
		fmt.Printf("## %s (%s)\n\n", c.Author, c.ID)
		if len(c.Bodies) > 0 {
			body := c.Bodies[len(c.Bodies)-1]
			fmt.Printf("%s\n\n", strings.TrimSpace(body.Content))
		}
	}

	return nil
}

// ThreadCreateCmd creates a new thread.
type ThreadCreateCmd struct {
	Message string `arg:"" required:"" help:"Initial comment message"`
	Goal    string `help:"Thread goal (review, discuss, impl, etc.)"`
	Anchor  string `help:"Code anchor in file:line format"`
	Group   string `help:"Thread group name"`
	Parent  string `help:"Parent thread ID"`
}

func (c *ThreadCreateCmd) Run() error {
	if strings.TrimSpace(c.Message) == "" {
		return fmt.Errorf("message cannot be empty")
	}

	if c.Goal != "" && !thread.ValidGoal(c.Goal) {
		return fmt.Errorf("invalid goal %q: must be one of %s", c.Goal, strings.Join(thread.GoalValues, ", "))
	}

	t := thread.NewThread("open", c.Goal)

	if c.Group != "" {
		t.Group = c.Group
	}

	if c.Parent != "" {
		if err := validateThreadID(c.Parent); err != nil {
			return fmt.Errorf("invalid parent ID: %w", err)
		}
		t.Parent = &thread.Parent{Ref: c.Parent}
	}

	if c.Anchor != "" {
		anchor, err := parseAnchor(c.Anchor)
		if err != nil {
			return err
		}
		t.Anchor = anchor
	}

	author := git.UserName()
	comment := thread.NewComment(author, c.Message)
	t.Comments = append(t.Comments, comment)

	dir := filepath.Join(projectRoot(), ".nota")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating .nota/: %w", err)
	}

	filename := threadFilename(t)
	path := filepath.Join(dir, filename)

	if err := thread.WriteThread(path, t); err != nil {
		return err
	}

	fmt.Printf("Created thread %s\n", t.ID)
	return nil
}

func parseAnchor(s string) (*thread.Anchor, error) {
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

	anchor := &thread.Anchor{
		File: file,
		Line: line,
	}

	if commit, err := git.HeadCommit(""); err == nil {
		anchor.Commit = commit
	}

	return anchor, nil
}

func threadFilename(t *thread.Thread) string {
	suffix := strings.TrimPrefix(t.ID, "l:")
	if t.Group != "" {
		return t.Group + "-" + suffix + ".xml"
	}
	return suffix + ".xml"
}

var localIDPattern = regexp.MustCompile(`^l:[0-9a-f]{16}$`)
var githubIDPattern = regexp.MustCompile(`^gh:\d+$`)

func validateThreadID(id string) error {
	if localIDPattern.MatchString(id) || githubIDPattern.MatchString(id) {
		return nil
	}
	return fmt.Errorf("invalid thread ID format, expected l:uuid or gh:id")
}

// ThreadResolveCmd marks a thread as resolved.
type ThreadResolveCmd struct {
	ID string `arg:"" required:"" help:"Thread ID"`
}

func (c *ThreadResolveCmd) Run() error {
	return updateThreadStatus(c.ID, "resolved")
}

// ThreadWontfixCmd marks a thread as wontfix.
type ThreadWontfixCmd struct {
	ID string `arg:"" required:"" help:"Thread ID"`
}

func (c *ThreadWontfixCmd) Run() error {
	return updateThreadStatus(c.ID, "wontfix")
}

// ThreadReopenCmd reopens a thread.
type ThreadReopenCmd struct {
	ID string `arg:"" required:"" help:"Thread ID"`
}

func (c *ThreadReopenCmd) Run() error {
	return updateThreadStatus(c.ID, "open")
}

func updateThreadStatus(id, status string) error {
	if err := validateThreadID(id); err != nil {
		return err
	}

	dir := filepath.Join(projectRoot(), ".nota")
	info, err := thread.FindThread(dir, id)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("thread not found: %s", id)
	}

	info.Thread.Status = status
	if err := thread.WriteThread(info.Path, info.Thread); err != nil {
		return err
	}

	fmt.Printf("Thread %s marked as %s\n", id, status)
	return nil
}

// ThreadGoalCmd updates a thread's goal.
type ThreadGoalCmd struct {
	ID   string `arg:"" required:"" help:"Thread ID"`
	Goal string `arg:"" required:"" help:"New goal value"`
}

func (c *ThreadGoalCmd) Run() error {
	if err := validateThreadID(c.ID); err != nil {
		return err
	}

	if !thread.ValidGoal(c.Goal) {
		return fmt.Errorf("invalid goal %q: must be one of %s", c.Goal, strings.Join(thread.GoalValues, ", "))
	}

	dir := filepath.Join(projectRoot(), ".nota")
	info, err := thread.FindThread(dir, c.ID)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("thread not found: %s", c.ID)
	}

	info.Thread.Goal = c.Goal
	if err := thread.WriteThread(info.Path, info.Thread); err != nil {
		return err
	}

	fmt.Printf("Thread %s goal set to %s\n", c.ID, c.Goal)
	return nil
}
