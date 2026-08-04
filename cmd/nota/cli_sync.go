package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urso/nota/pkg/github"
	"github.com/urso/nota/pkg/github/ghapi"
)

// SyncCmd groups GitHub sync commands.
type SyncCmd struct {
	Pull SyncPullCmd `cmd:"" help:"Pull PR comments from GitHub"`
}

// SyncPullCmd imports comments from a GitHub PR.
type SyncPullCmd struct {
	PR    int    `arg:"" optional:"" help:"PR number (auto-detects if omitted)"`
	Repo  string `help:"Repository (owner/name, default: from git remotes)" short:"R"`
	Group string `help:"Group name for imported threads (default: pr-<number>)" short:"g"`
}

func (c *SyncPullCmd) Run() error {
	return c.run(os.Stdout, os.Stderr, projectRoot())
}

func (c *SyncPullCmd) run(stdout, stderr io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")

	repo, err := ghapi.ResolveRepo(c.Repo)
	if err != nil {
		return fmt.Errorf("resolving repository: %w", err)
	}

	if err := ghapi.CheckAuth(repo.Host); err != nil {
		return fmt.Errorf("authentication required: run 'gh auth login' or set GH_TOKEN")
	}

	pr := c.PR
	if pr == 0 {
		detector := ghapi.NewPRDetector(repo)
		detected, err := detector.DetectPR()
		if err != nil {
			if ghapi.IsNoPR(err) {
				return fmt.Errorf("no PR associated with current branch; specify a PR number: nota sync pull <pr-number>")
			}
			return fmt.Errorf("detecting PR: %w", err)
		}
		pr = detected
		fmt.Fprintf(stdout, "Detected PR #%d\n", pr)
	}

	group := c.Group
	if group == "" {
		group = fmt.Sprintf("pr-%d", pr)
	}

	client, err := ghapi.NewClient(ghapi.ClientOptions{Host: repo.Host})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	ctx := context.Background()

	fmt.Fprintf(stdout, "Fetching PR #%d from %s...\n", pr, repo.String())

	threads, prInfo, err := client.FetchReviewThreads(ctx, repo, pr)
	if err != nil {
		return fmt.Errorf("fetching review threads: %w", err)
	}

	reviews, err := client.FetchReviews(ctx, repo, pr)
	if err != nil {
		return fmt.Errorf("fetching reviews: %w", err)
	}

	issueComments, err := client.FetchIssueComments(ctx, repo, pr)
	if err != nil {
		return fmt.Errorf("fetching issue comments: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating .nota directory: %w", err)
	}

	result, err := github.Pull(dir, threads, reviews, issueComments, prInfo, github.PullOptions{
		Group:        group,
		ResolvedRepo: repo,
	})
	if err != nil {
		return fmt.Errorf("writing threads: %w", err)
	}

	printPullSummary(stdout, result, pr)
	return nil
}

func printPullSummary(w io.Writer, r *github.PullResult, pr int) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Pulled PR #%d:\n", pr)
	fmt.Fprintf(w, "  Inline threads: %d (%d resolved, %d open)\n", r.InlineThreads, r.ResolvedCount, r.OpenCount)

	if r.ConversationThread {
		fmt.Fprintln(w, "  Conversation:   1 thread")
	}

	if r.NewThreads > 0 || r.UpdatedThreads > 0 {
		fmt.Fprintf(w, "  Changes:        %d new, %d updated", r.NewThreads, r.UpdatedThreads)
		if r.NewComments > 0 {
			fmt.Fprintf(w, " (%d new comments)", r.NewComments)
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "  Changes:        none (already up to date)")
	}

	fmt.Fprintf(w, "  Group:          %s\n", r.Group)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "View with: nota thread list --group=%s\n", r.Group)
}
