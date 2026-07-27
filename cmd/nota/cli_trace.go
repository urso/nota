package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/thread"
	"github.com/urso/nota/pkg/trace"
)

// TraceCmd updates anchor positions to HEAD.
type TraceCmd struct {
	ID string `arg:"" optional:"" help:"Thread ID (omit to trace all threads)"`
}

func (c *TraceCmd) Run() error {
	return c.run(os.Stdout, projectRoot())
}

func (c *TraceCmd) run(w io.Writer, root string) error {
	if root == "" {
		return fmt.Errorf("not in a git repository")
	}

	head, err := git.HeadCommit(root)
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	dir := filepath.Join(root, ".nota")

	if c.ID != "" {
		return c.traceOne(w, root, dir, head)
	}
	return c.traceAll(w, root, dir, head)
}

func (c *TraceCmd) traceOne(w io.Writer, root, dir, head string) error {
	info, err := thread.FindThread(dir, c.ID)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("thread not found: %s", c.ID)
	}

	gitOps := git.NewCachedOps(git.NewOps(root))
	traced, outdated := traceThreadAnchorsToCommit(gitOps, info.Thread, head)
	if traced > 0 || outdated > 0 {
		if err := thread.WriteThread(info.Path, info.Thread); err != nil {
			return fmt.Errorf("writing thread: %w", err)
		}
	}

	fmt.Fprintf(w, "%s: %d traced, %d outdated\n", c.ID, traced, outdated)
	return nil
}

func (c *TraceCmd) traceAll(w io.Writer, root, dir, head string) error {
	totalTraced := 0
	totalOutdated := 0
	threadCount := 0

	// Use cached GitOps to share git results across all threads
	gitOps := git.NewCachedOps(git.NewOps(root))

	for info, err := range thread.AllThreads(dir) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}

		traced, outdated := traceThreadAnchorsToCommit(gitOps, info.Thread, head)
		if traced > 0 || outdated > 0 {
			if err := thread.WriteThread(info.Path, info.Thread); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write %s: %v\n", info.Path, err)
				continue
			}
		}

		totalTraced += traced
		totalOutdated += outdated
		threadCount++
	}

	fmt.Fprintf(w, "%d threads: %d anchors traced, %d marked outdated\n",
		threadCount, totalTraced, totalOutdated)
	return nil
}

// traceThreadAnchorsToCommit traces all anchors in a thread to the target commit.
// Uses batch tracing to share git log output for anchors in the same file.
// Returns counts of traced and outdated anchors.
func traceThreadAnchorsToCommit(gitOps git.Ops, t *thread.Thread, targetCommit string) (traced, outdated int) {
	// Collect all anchors that need tracing
	var anchorsToTrace []thread.Anchor
	var anchorSources []anchorSource // tracks where each anchor came from

	// Thread-level anchors (use backtracking)
	if len(t.Anchors) > 0 {
		current := t.CurrentAnchor()
		if current.Commit != "" && current.Commit != targetCommit && !current.Outdated {
			anchorsToTrace = append(anchorsToTrace, *current)
			anchorSources = append(anchorSources, anchorSource{isThread: true, threadAnchors: t.Anchors})
		}
	}

	// Comment-level anchors
	for i := range t.Comments {
		c := &t.Comments[i]
		if c.Anchor != nil && c.Anchor.Commit != "" && c.Anchor.Commit != targetCommit && !c.Anchor.Outdated {
			anchorsToTrace = append(anchorsToTrace, *c.Anchor)
			anchorSources = append(anchorSources, anchorSource{commentIdx: i})
		}
	}

	if len(anchorsToTrace) == 0 {
		return 0, 0
	}

	// Batch trace all anchors using the shared GitOps
	results := trace.TraceAnchorsWithGit(gitOps, anchorsToTrace, targetCommit)

	// Apply results
	for i, br := range results {
		if br.Error != nil {
			continue
		}

		src := anchorSources[i]
		if src.isThread {
			// Thread-level anchor: if batch trace marked outdated, try backtracking
			// through anchor history to find a valid starting point
			if br.Result.Outdated {
				result, err := trace.TraceWithBacktrack(gitOps.RepoDir(), src.threadAnchors, targetCommit)
				if err == nil {
					t.AppendAnchor(result.Anchor)
					traced++
					if result.Outdated {
						outdated++
					}
					continue
				}
			}
			// Use batch result directly if not outdated or backtrack failed
			t.AppendAnchor(br.Result.Anchor)
			traced++
			if br.Result.Outdated {
				outdated++
			}
		} else {
			// Comment-level anchor
			t.Comments[src.commentIdx].Anchor = &br.Result.Anchor
			traced++
			if br.Result.Outdated {
				outdated++
			}
		}
	}

	return
}

type anchorSource struct {
	isThread      bool
	threadAnchors []thread.Anchor // for backtracking
	commentIdx    int
}
