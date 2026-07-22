package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urso/nota/pkg/thread"
)

// ThreadCmd groups thread management commands.
type ThreadCmd struct {
	List     ThreadListCmd     `cmd:"" help:"List threads"`
	Show     ThreadShowCmd     `cmd:"" help:"Show a thread"`
	Create   ThreadCreateCmd   `cmd:"" help:"Create a new thread"`
	Add      ThreadAddCmd      `cmd:"" help:"Add a comment to a thread"`
	Spawn    ThreadSpawnCmd    `cmd:"" help:"Create a child thread"`
	Depend   ThreadDependCmd   `cmd:"" help:"Add dependency on other threads"`
	Undepend ThreadUndependCmd `cmd:"" help:"Remove dependency on other threads"`
	Resolve  ThreadResolveCmd  `cmd:"" help:"Mark thread as resolved"`
	Wontfix  ThreadWontfixCmd  `cmd:"" help:"Mark thread as wontfix"`
	Reopen   ThreadReopenCmd   `cmd:"" help:"Reopen a thread"`
	Goal     ThreadGoalCmd     `cmd:"" help:"Update thread goal"`
}

// ThreadListCmd lists threads with optional filters.
type ThreadListCmd struct {
	Status string `help:"Filter by status (open, resolved, wontfix)"`
	Goal   string `help:"Filter by goal"`
	Group  string `help:"Filter by group"`
	Tag    string `help:"Filter by tag"`

	// Relationship queries
	RefsOf       string `help:"List threads that this thread references" name:"refs-of"`
	DepsOf       string `help:"List threads that this thread depends on" name:"deps-of"`
	ReferencedBy string `help:"List threads that reference this thread" name:"referenced-by"`
	BlockedBy    string `help:"List threads that depend on this thread" name:"blocked-by"`
}

func (c *ThreadListCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadListCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	filter := thread.ThreadFilter{
		Status:       c.Status,
		Goal:         c.Goal,
		Group:        c.Group,
		Tag:          c.Tag,
		RefsOf:       c.RefsOf,
		DepsOf:       c.DepsOf,
		ReferencedBy: c.ReferencedBy,
		BlockedBy:    c.BlockedBy,
	}

	threads, err := thread.ListThreads(dir, filter)
	if err != nil {
		return err
	}

	for _, info := range threads {
		t := info.Thread
		title := thread.ThreadTitle(t)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, t.Status, t.Goal, title)
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
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadShowCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")

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
		fmt.Fprint(w, string(data))
		return nil
	}

	if c.JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info.Thread)
	}

	return renderThreadTo(w, info.Thread)
}

func renderThreadTo(w io.Writer, t *thread.Thread) error {
	fmt.Fprintf(w, "# Thread %s\n\n", t.ID)
	fmt.Fprintf(w, "Status: %s", t.Status)
	if t.Goal != "" {
		fmt.Fprintf(w, "  Goal: %s", t.Goal)
	}
	if t.Group != "" {
		fmt.Fprintf(w, "  Group: %s", t.Group)
	}
	fmt.Fprintln(w)

	if t.Anchor != nil {
		fmt.Fprintf(w, "Anchor: %s:%d", t.Anchor.File, t.Anchor.Line)
		if t.Anchor.Commit != "" {
			fmt.Fprintf(w, " @ %s", t.Anchor.Commit[:min(7, len(t.Anchor.Commit))])
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w)

	for i, c := range t.Comments {
		if i > 0 {
			fmt.Fprintln(w, "---")
		}
		fmt.Fprintf(w, "## %s (%s)\n\n", c.Author, c.ID)
		if len(c.Bodies) > 0 {
			body := c.Bodies[len(c.Bodies)-1]
			fmt.Fprintf(w, "%s\n\n", strings.TrimSpace(body.Content))
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
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadCreateCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	t, err := thread.Create(dir, thread.CreateOpts{
		Message: c.Message,
		Goal:    c.Goal,
		Group:   c.Group,
		Parent:  c.Parent,
		Anchor:  c.Anchor,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Created thread %s\n", t.ID)
	return nil
}

// ThreadAddCmd adds a comment to an existing thread.
type ThreadAddCmd struct {
	ID      string `arg:"" required:"" help:"Thread ID"`
	Message string `arg:"" optional:"" help:"Comment message"`
	File    string `help:"Read message from file (- for stdin)"`
	Local   bool   `help:"Set visibility to local"`
	ReplyTo string `help:"Comment ID to reply to" name:"reply-to"`
	Anchor  string `help:"Code anchor in file:line format"`
}

func (c *ThreadAddCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadAddCmd) run(w io.Writer, root string) error {
	message, err := c.resolveMessage()
	if err != nil {
		return err
	}

	dir := filepath.Join(root, ".nota")
	comment, err := thread.AddComment(dir, c.ID, thread.AddCommentOpts{
		Message: message,
		Local:   c.Local,
		ReplyTo: c.ReplyTo,
		Anchor:  c.Anchor,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Added comment %s to thread %s\n", comment.ID, c.ID)
	return nil
}

func (c *ThreadAddCmd) resolveMessage() (string, error) {
	switch {
	case c.Message != "" && c.File != "":
		return "", fmt.Errorf("cannot specify both message argument and --file")
	case c.Message != "":
		return c.Message, nil
	case c.File == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	case c.File != "":
		data, err := os.ReadFile(c.File)
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("message cannot be empty")
	}
}

// ThreadResolveCmd marks a thread as resolved.
type ThreadResolveCmd struct {
	ID string `arg:"" required:"" help:"Thread ID"`
}

func (c *ThreadResolveCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadResolveCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	if err := thread.UpdateStatus(dir, c.ID, "resolved"); err != nil {
		return err
	}
	fmt.Fprintf(w, "Thread %s marked as resolved\n", c.ID)
	return nil
}

// ThreadWontfixCmd marks a thread as wontfix.
type ThreadWontfixCmd struct {
	ID string `arg:"" required:"" help:"Thread ID"`
}

func (c *ThreadWontfixCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadWontfixCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	if err := thread.UpdateStatus(dir, c.ID, "wontfix"); err != nil {
		return err
	}
	fmt.Fprintf(w, "Thread %s marked as wontfix\n", c.ID)
	return nil
}

// ThreadReopenCmd reopens a thread.
type ThreadReopenCmd struct {
	ID string `arg:"" required:"" help:"Thread ID"`
}

func (c *ThreadReopenCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadReopenCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	if err := thread.UpdateStatus(dir, c.ID, "open"); err != nil {
		return err
	}
	fmt.Fprintf(w, "Thread %s marked as open\n", c.ID)
	return nil
}

// ThreadGoalCmd updates a thread's goal.
type ThreadGoalCmd struct {
	ID   string `arg:"" required:"" help:"Thread ID"`
	Goal string `arg:"" required:"" help:"New goal value"`
}

func (c *ThreadGoalCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadGoalCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	if err := thread.UpdateGoal(dir, c.ID, c.Goal); err != nil {
		return err
	}
	fmt.Fprintf(w, "Thread %s goal set to %s\n", c.ID, c.Goal)
	return nil
}

// ThreadSpawnCmd creates a child thread linked to a parent.
type ThreadSpawnCmd struct {
	ParentID string `arg:"" required:"" help:"Parent thread ID"`
	Message  string `arg:"" required:"" help:"Initial comment message"`
	Goal     string `help:"Thread goal (review, discuss, impl, etc.)"`
	Group    string `help:"Thread group name (inherits from parent if not specified)"`
}

func (c *ThreadSpawnCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadSpawnCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	child, err := thread.Spawn(dir, c.ParentID, thread.SpawnOpts{
		Message: c.Message,
		Goal:    c.Goal,
		Group:   c.Group,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Created child thread %s (parent: %s)\n", child.ID, c.ParentID)
	return nil
}

// ThreadDependCmd adds dependencies to a thread.
type ThreadDependCmd struct {
	ID         string   `arg:"" required:"" help:"Thread ID"`
	BlockerIDs []string `arg:"" required:"" help:"Blocker thread IDs"`
}

func (c *ThreadDependCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadDependCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	dependsOn, err := thread.AddDependencies(dir, c.ID, c.BlockerIDs)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Thread %s now depends on: %s\n", c.ID, dependsOn)
	return nil
}

// ThreadUndependCmd removes dependencies from a thread.
type ThreadUndependCmd struct {
	ID         string   `arg:"" required:"" help:"Thread ID"`
	BlockerIDs []string `arg:"" required:"" help:"Blocker thread IDs to remove"`
}

func (c *ThreadUndependCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadUndependCmd) run(w io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")
	dependsOn, err := thread.RemoveDependencies(dir, c.ID, c.BlockerIDs)
	if err != nil {
		return err
	}

	if dependsOn == "" {
		fmt.Fprintf(w, "Thread %s has no dependencies\n", c.ID)
	} else {
		fmt.Fprintf(w, "Thread %s now depends on: %s\n", c.ID, dependsOn)
	}
	return nil
}
