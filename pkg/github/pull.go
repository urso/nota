package github

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urso/nota/pkg/github/ghapi"
	"github.com/urso/nota/pkg/thread"
)

// PullResult holds the results of a pull operation.
type PullResult struct {
	InlineThreads      int
	ConversationThread bool
	NewThreads         int
	UpdatedThreads     int
	NewComments        int
	ResolvedCount      int
	OpenCount          int
	Group              string
}

// PullOptions configures a pull operation.
type PullOptions struct {
	Group        string
	ResolvedRepo ghapi.Repo
}

// matchIndex indexes existing threads by their sync identifiers.
type matchIndex struct {
	byThreadID     map[string]matchEntry // PRRT_... -> entry
	byConversation map[string]matchEntry // "provider:owner/repo:pr" -> entry
}

type matchEntry struct {
	thread *thread.Thread
	path   string
}

func buildMatchIndex(dir string, resolvedRepo ghapi.Repo) (*matchIndex, error) {
	idx := &matchIndex{
		byThreadID:     make(map[string]matchEntry),
		byConversation: make(map[string]matchEntry),
	}

	resolvedRepoStr := fmt.Sprintf("%s/%s", resolvedRepo.Owner, resolvedRepo.Name)

	for info, err := range thread.AllThreads(dir) {
		if err != nil {
			return nil, err
		}

		if info.Thread.Sync == nil {
			continue
		}

		s := info.Thread.Sync
		if s.Provider != "github" {
			continue
		}

		entry := matchEntry{thread: info.Thread, path: info.Path}

		if s.EffectiveKind() == thread.SyncKindReviewThread && s.ThreadID != "" {
			idx.byThreadID[s.ThreadID] = entry
		} else if s.EffectiveKind() == thread.SyncKindPR {
			repo := s.Repo
			if repo == "" {
				repo = resolvedRepoStr
			}
			key := conversationKey(s.Provider, repo, s.PR)
			idx.byConversation[key] = entry
		}
	}

	return idx, nil
}

func conversationKey(provider, repo, pr string) string {
	return fmt.Sprintf("%s:%s:%s", provider, strings.ToLower(repo), pr)
}

// Pull imports threads from GitHub into the local thread directory.
func Pull(dir string, threads []ghapi.ReviewThread, reviews []ghapi.Review, issueComments []ghapi.IssueComment, prInfo ghapi.PRInfo, opts PullOptions) (*PullResult, error) {
	idx, err := buildMatchIndex(dir, opts.ResolvedRepo)
	if err != nil {
		return nil, fmt.Errorf("building match index: %w", err)
	}

	result := &PullResult{
		Group: opts.Group,
	}

	var toWrite []writeEntry

	for _, rt := range threads {
		result.InlineThreads++
		mapped := mapReviewThread(rt, prInfo, opts.ResolvedRepo)
		mapped.Group = opts.Group

		if existing, ok := idx.byThreadID[rt.ID]; ok {
			mr := mergeThread(existing.thread, mapped)
			if mr.changed {
				result.UpdatedThreads++
				result.NewComments += mr.newComments
			}
			toWrite = append(toWrite, writeEntry{thread: mr.thread, path: existing.path})
		} else {
			result.NewThreads++
			result.NewComments += len(mapped.Comments)
			toWrite = append(toWrite, writeEntry{thread: mapped, path: ""})
		}

		if mapped.Status == "resolved" {
			result.ResolvedCount++
		} else {
			result.OpenCount++
		}
	}

	convThread := mapConversationThread(reviews, issueComments, prInfo, opts.ResolvedRepo)
	if convThread != nil {
		result.ConversationThread = true
		convThread.Group = opts.Group

		repoStr := fmt.Sprintf("%s/%s", opts.ResolvedRepo.Owner, opts.ResolvedRepo.Name)
		convKey := conversationKey("github", repoStr, convThread.Sync.PR)

		if existing, ok := idx.byConversation[convKey]; ok {
			mr := mergeThread(existing.thread, convThread)
			if mr.changed {
				result.UpdatedThreads++
				result.NewComments += mr.newComments
			}
			toWrite = append(toWrite, writeEntry{thread: mr.thread, path: existing.path})
		} else {
			result.NewThreads++
			result.NewComments += len(convThread.Comments)
			toWrite = append(toWrite, writeEntry{thread: convThread, path: ""})
		}
	}

	if err := writeThreads(dir, toWrite, opts.Group); err != nil {
		return nil, err
	}

	return result, nil
}

type writeEntry struct {
	thread *thread.Thread
	path   string // empty for new threads
}

func writeThreads(dir string, entries []writeEntry, group string) error {
	if len(entries) == 0 {
		return nil
	}

	safeName, err := sanitizeGroup(group)
	if err != nil {
		return fmt.Errorf("invalid group name %q: %w", group, err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory: %w", err)
	}

	unlock, err := thread.LockDir(dir)
	if err != nil {
		return err
	}
	defer unlock()

	nextNum, err := thread.NextNumber(dir)
	if err != nil {
		return fmt.Errorf("getting next thread number: %w", err)
	}

	for _, e := range entries {
		if err := thread.ValidateThread(e.thread); err != nil {
			return fmt.Errorf("invalid thread: %w", err)
		}

		var path string
		if e.path != "" {
			path = e.path
		} else {
			e.thread.Number = nextNum
			e.thread.Group = safeName
			nextNum++
			path = filepath.Join(dir, thread.Filename(e.thread))
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", path, err)
		}
		if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
			return fmt.Errorf("path %q resolves outside target directory", path)
		}

		if err := thread.WriteThread(path, e.thread); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	return nil
}

func sanitizeGroup(name string) (string, error) {
	if name == "" {
		return "", nil
	}

	for _, c := range name {
		switch c {
		case '/', '\\':
			return "", fmt.Errorf("contains path separator")
		case '\x00':
			return "", fmt.Errorf("contains null byte")
		case ':', '<', '>', '|', '*', '?', '"':
			return "", fmt.Errorf("contains invalid filename character %q", c)
		}
	}

	if name == "." || name == ".." {
		return "", fmt.Errorf("invalid name %q", name)
	}

	return name, nil
}

type mergeResult struct {
	thread      *thread.Thread
	changed     bool
	newComments int
}

func mergeThread(existing, incoming *thread.Thread) mergeResult {
	commentIndex := make(map[string]int)
	for i, c := range existing.Comments {
		if c.ExternalID != "" {
			commentIndex[c.ExternalID] = i
		}
	}

	var newComments []thread.Comment
	changed := false

	for _, inc := range incoming.Comments {
		if idx, ok := commentIndex[inc.ExternalID]; ok {
			existingComment := &existing.Comments[idx]
			if existingComment.UpdatedAt != inc.UpdatedAt {
				existingComment.Bodies = append(existingComment.Bodies, inc.Bodies...)
				existingComment.UpdatedAt = inc.UpdatedAt
				changed = true
			}
		} else {
			newComments = append(newComments, inc)
			changed = true
		}
	}

	if len(newComments) > 0 {
		sort.Slice(newComments, func(i, j int) bool {
			if len(newComments[i].Bodies) == 0 || len(newComments[j].Bodies) == 0 {
				return false
			}
			return newComments[i].Bodies[0].Time < newComments[j].Bodies[0].Time
		})
		existing.Comments = append(existing.Comments, newComments...)
	}

	return mergeResult{thread: existing, changed: changed, newComments: len(newComments)}
}
