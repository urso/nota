package github

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/urso/nota/pkg/github/ghapi"
	"github.com/urso/nota/pkg/thread"
)

const (
	providerGitHub = "github"
	authorGhost    = "github:ghost"
)

func formatAuthor(login string) string {
	if login == "" {
		return authorGhost
	}
	return "github:" + login
}

func mapReviewThread(rt ghapi.ReviewThread, prInfo ghapi.PRInfo, resolvedRepo ghapi.Repo) *thread.Thread {
	t := &thread.Thread{
		ID:     thread.NewGitHubID(),
		Goal:   "review",
		Status: "open",
		Sync: &thread.SyncConfig{
			Provider: providerGitHub,
			PR:       strconv.Itoa(prInfo.Number),
			ThreadID: rt.ID,
			PRID:     prInfo.ID,
		},
	}

	if rt.IsResolved {
		t.Status = "resolved"
	}

	if rt.SubjectType == "FILE" {
		t.AppendFileAnchor(thread.FileAnchor{
			File:   rt.Path,
			Commit: rt.CommitID,
		})
	} else {
		anchor := thread.Anchor{
			File: rt.Path,
		}
		if rt.Line != nil {
			anchor.Line = *rt.Line
			anchor.Commit = rt.CommitID
		} else if rt.OriginalLine != nil {
			anchor.Line = *rt.OriginalLine
			anchor.Commit = rt.OriginalCommitID
			anchor.Outdated = true
		}
		t.AppendAnchor(anchor)
	}

	for _, rc := range rt.Comments {
		t.Comments = append(t.Comments, mapReviewComment(rc))
	}

	return t
}

func mapReviewComment(rc ghapi.ReviewComment) thread.Comment {
	c := thread.Comment{
		ID:         "gh:" + rc.ID,
		Author:     formatAuthor(rc.Author),
		SyncStatus: "pulled",
		ExternalID: rc.FullDatabaseID,
		UpdatedAt:  rc.UpdatedAt,
		Bodies: []thread.Body{{
			Time:    rc.CreatedAt,
			Content: rc.Body,
		}},
	}
	return c
}

type conversationEntry struct {
	time    time.Time
	comment thread.Comment
}

func mapConversationThread(reviews []ghapi.Review, issueComments []ghapi.IssueComment, prInfo ghapi.PRInfo, resolvedRepo ghapi.Repo) *thread.Thread {
	var entries []conversationEntry

	for _, r := range reviews {
		if r.Body == "" {
			continue
		}
		t, _ := time.Parse(time.RFC3339, r.SubmittedAt)
		entries = append(entries, conversationEntry{
			time:    t,
			comment: mapReview(r),
		})
	}

	for _, ic := range issueComments {
		t, _ := time.Parse(time.RFC3339, ic.CreatedAt)
		entries = append(entries, conversationEntry{
			time:    t,
			comment: mapIssueComment(ic),
		})
	}

	if len(entries) == 0 {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].time.Before(entries[j].time)
	})

	t := &thread.Thread{
		ID:     thread.NewGitHubID(),
		Goal:   "discuss",
		Status: "open",
		Sync: &thread.SyncConfig{
			Provider: providerGitHub,
			PR:       strconv.Itoa(prInfo.Number),
			Kind:     thread.SyncKindPR,
			PRID:     prInfo.ID,
		},
	}

	for _, e := range entries {
		t.Comments = append(t.Comments, e.comment)
	}

	return t
}

func mapReview(r ghapi.Review) thread.Comment {
	author := authorGhost
	if r.User != nil {
		author = formatAuthor(r.User.Login)
	}

	return thread.Comment{
		ID:         "gh:" + r.NodeID,
		Author:     author,
		SyncStatus: "pulled",
		ExternalID: strconv.FormatInt(r.ID, 10),
		UpdatedAt:  r.SubmittedAt,
		Bodies: []thread.Body{{
			Time:    r.SubmittedAt,
			Content: r.Body,
		}},
	}
}

func mapIssueComment(ic ghapi.IssueComment) thread.Comment {
	author := authorGhost
	if ic.User != nil {
		author = formatAuthor(ic.User.Login)
	}

	return thread.Comment{
		ID:         "gh:" + ic.NodeID,
		Author:     author,
		SyncStatus: "pulled",
		ExternalID: strconv.FormatInt(ic.ID, 10),
		UpdatedAt:  ic.UpdatedAt,
		Bodies: []thread.Body{{
			Time:    ic.CreatedAt,
			Content: ic.Body,
		}},
	}
}

func setRepoIfDifferent(t *thread.Thread, resolvedRepo, prRepo ghapi.Repo) {
	if !ghapi.RepoMatches(resolvedRepo, prRepo) {
		t.Sync.Repo = fmt.Sprintf("%s/%s", prRepo.Owner, prRepo.Name)
	}
}
