package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/thread"
	"github.com/urso/nota/pkg/trace"
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

// Author selectors for thread show --authors.
const (
	authorsAll    = "all"
	authorsHumans = "humans"
	authorsBots   = "bots"
)

// ThreadShowCmd displays a thread.
type ThreadShowCmd struct {
	ID      string `arg:"" required:"" help:"Thread ID"`
	Raw     bool   `help:"Output XML source"`
	JSON    bool   `name:"json" help:"Output JSON"`
	Authors string `default:"all" enum:"all,humans,bots" help:"Which comment authors to show (all, humans, bots)"`
	Limit   int    `help:"Show only the last N comments (0 = unbounded); the first comment is always shown"`
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

	// Trace anchors to HEAD before display (unless --raw)
	if !c.Raw {
		if updated := traceThreadAnchors(root, info.Thread); updated {
			if err := thread.WriteThread(info.Path, info.Thread); err != nil {
				// Non-fatal: log but continue showing
				fmt.Fprintf(os.Stderr, "warning: failed to save traced anchors: %v\n", err)
			}
		}
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

	return renderThreadTo(w, info.Thread, renderOpts{Authors: c.Authors, Limit: c.Limit})
}

// traceThreadAnchors traces all anchors in a thread to HEAD, using backtracking
// when the current anchor's commit is not an ancestor of HEAD (revert case).
// Returns true if any anchors were updated.
func traceThreadAnchors(root string, t *thread.Thread) bool {
	head, err := git.HeadCommit(root)
	if err != nil {
		return false
	}

	updated := false

	// Trace thread-level anchors with backtracking support
	if len(t.Anchors) > 0 {
		current := t.CurrentAnchor()
		if current.Commit != "" && current.Commit != head && !current.Outdated {
			result, err := trace.TraceWithBacktrack(root, t.Anchors, head)
			if err == nil {
				t.AppendAnchor(result.Anchor)
				updated = true
			}
		}
	}

	// Trace comment-level anchors (no history, so use direct trace)
	for i := range t.Comments {
		c := &t.Comments[i]
		if c.Anchor != nil && c.Anchor.Commit != "" && c.Anchor.Commit != head && !c.Anchor.Outdated {
			result, err := trace.TraceAnchor(root, *c.Anchor, head)
			if err == nil {
				c.Anchor = &result.Anchor
				updated = true
			}
		}
	}

	return updated
}

// renderOpts bounds what a thread render includes. The zero value renders
// every comment, matching an unfiltered read.
type renderOpts struct {
	Authors string // all (default), humans, or bots
	Limit   int    // last N comments; 0 means unbounded
}

func renderThreadTo(w io.Writer, t *thread.Thread, opts renderOpts) error {
	fmt.Fprintf(w, "# Thread %s\n\n", t.ID)
	fmt.Fprintf(w, "Status: %s", t.Status)
	if t.Goal != "" {
		fmt.Fprintf(w, "  Goal: %s", t.Goal)
	}
	if t.Group != "" {
		fmt.Fprintf(w, "  Group: %s", t.Group)
	}
	fmt.Fprintln(w)

	if loc := t.CurrentLocation(); loc != nil {
		// File anchors have Line==0; the absent line number is what
		// distinguishes them on screen.
		if loc.Line == 0 {
			fmt.Fprintf(w, "Anchor: %s", loc.File)
		} else {
			fmt.Fprintf(w, "Anchor: %s:%d", loc.File, loc.Line)
		}
		if loc.Commit != "" {
			fmt.Fprintf(w, " @ %s", loc.Commit[:min(7, len(loc.Commit))])
		}
		if loc.Outdated {
			fmt.Fprintf(w, " [outdated]")
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w)

	selected, omitted := selectComments(t.Comments, opts)

	for i, c := range selected {
		if i > 0 {
			fmt.Fprintln(w, "---")
		}
		// The elision marker sits after the opening comment, which is always
		// rendered: silent truncation is indistinguishable from a short thread.
		if omitted > 0 && i == 1 {
			fmt.Fprintf(w, "... %d comments omitted ...\n\n---\n", omitted)
		}
		fmt.Fprintf(w, "## %s (%s)\n\n", c.Author, c.ID)
		if len(c.Bodies) > 0 {
			body := c.Bodies[len(c.Bodies)-1]
			fmt.Fprintf(w, "%s\n\n", strings.TrimSpace(body.Content))
		}
	}

	return nil
}

// selectComments applies the author filter, then the limit. The first comment
// of the thread is always retained: on an inline review thread it *is* the
// review concern, and everything after it responds to that.
// Returns the comments to render and how many were dropped by the limit.
func selectComments(comments []thread.Comment, opts renderOpts) (selected []thread.Comment, omitted int) {
	filtered := comments
	if opts.Authors != "" && opts.Authors != authorsAll {
		filtered = filtered[:0:0]
		for _, c := range comments {
			if matchesAuthorFilter(c.Author, opts.Authors) {
				filtered = append(filtered, c)
			}
		}
	}

	if opts.Limit <= 0 || len(filtered) <= opts.Limit {
		return filtered, 0
	}

	tail := filtered[len(filtered)-opts.Limit:]

	// The opening comment survives the limit regardless of author filter; it
	// sets context for the thread even when viewing only bot or human replies.
	first := comments[0]
	if len(tail) > 0 && tail[0].ID == first.ID {
		return tail, len(filtered) - len(tail)
	}

	head := []thread.Comment{first}
	// Anything dropped between the opening comment and the tail, counted over
	// the filtered set so the number matches what a wider limit would reveal.
	omitted = len(filtered) - len(tail)
	if matchesAuthorFilter(first.Author, opts.Authors) || opts.Authors == "" || opts.Authors == authorsAll {
		omitted--
	}

	return append(head, tail...), omitted
}

// matchesAuthorFilter reports whether an author passes the given selector.
// Bots are identified by GitHub's own "[bot]" login suffix.
func matchesAuthorFilter(author, selector string) bool {
	switch selector {
	case authorsBots:
		return isBotAuthor(author)
	case authorsHumans:
		return !isBotAuthor(author)
	default:
		return true
	}
}

func isBotAuthor(author string) bool {
	return strings.HasSuffix(author, "[bot]")
}

// AgentAuthor is the author recorded for comments written by an AI agent.
// Human comments fall back to git config user.name.
const AgentAuthor = "agent"

// authorFor returns the author to record for a comment. An empty string lets
// the thread package fall back to git config user.name.
func authorFor(agent bool) string {
	if agent {
		return AgentAuthor
	}
	return ""
}

// ThreadCreateCmd creates a new thread.
type ThreadCreateCmd struct {
	Message string `arg:"" required:"" help:"Initial comment message (title)"`
	Goal    string `help:"Thread goal (review, discuss, impl, etc.)"`
	Anchor  string `help:"Code anchor in file:line format"`
	Group   string `help:"Thread group name"`
	Tags    string `help:"Comma-separated tags (e.g. severity:critical,category:correctness)"`
	Parent  string `help:"Parent thread ID"`
	Body    string `help:"Extended description (- for stdin)"`
	Agent   bool   `help:"Record the comment as authored by the agent instead of the git user"`
}

func (c *ThreadCreateCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *ThreadCreateCmd) run(w io.Writer, root string) error {
	body, err := c.resolveBody()
	if err != nil {
		return err
	}

	dir := filepath.Join(root, ".nota")
	t, err := thread.Create(dir, thread.CreateOpts{
		Message: c.Message,
		Body:    body,
		Goal:    c.Goal,
		Group:   c.Group,
		Tags:    c.Tags,
		Parent:  c.Parent,
		Anchor:  c.Anchor,
		Author:  authorFor(c.Agent),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Created thread %s\n", t.ID)
	return nil
}

func (c *ThreadCreateCmd) resolveBody() (string, error) {
	if c.Body == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	return c.Body, nil
}

// ThreadAddCmd adds a comment to an existing thread.
type ThreadAddCmd struct {
	ID      string `arg:"" required:"" help:"Thread ID"`
	Message string `arg:"" optional:"" help:"Comment message"`
	File    string `help:"Read message from file (- for stdin)"`
	Local   bool   `help:"Set visibility to local"`
	ReplyTo string `help:"Comment ID to reply to" name:"reply-to"`
	Anchor  string `help:"Code anchor in file:line format"`
	Agent   bool   `help:"Record the comment as authored by the agent instead of the git user"`
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
		Author:  authorFor(c.Agent),
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
	Agent    bool   `help:"Record the comment as authored by the agent instead of the git user"`
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
		Author:  authorFor(c.Agent),
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
