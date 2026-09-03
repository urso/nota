package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/urso/nota/pkg/thread"
	"github.com/urso/nota/pkg/trace"
)

// List returns threads matching the filter with resolved anchors.
func (s *Service) List(f Filter) ([]ThreadView, error) {
	threadFilter := thread.ThreadFilter{
		Status: f.Status,
		Goal:   f.Goal,
		Group:  f.Group,
		Tag:    f.Tag,
	}

	infos, err := thread.ListThreads(s.notaDir, threadFilter)
	if err != nil {
		return nil, fmt.Errorf("listing threads: %w", err)
	}

	var results []ThreadView
	for _, info := range infos {
		view := s.buildView(info.Thread)

		if f.File != "" && !s.matchesFile(view, f.File) {
			continue
		}

		results = append(results, view)
	}

	return results, nil
}

// Get returns a single thread by ID or number.
func (s *Service) Get(query string) (*ThreadView, error) {
	info, err := thread.FindThread(s.notaDir, query)
	if err != nil {
		return nil, fmt.Errorf("finding thread: %w", err)
	}
	if info == nil {
		return nil, nil
	}

	view := s.buildView(info.Thread)
	return &view, nil
}

// buildView creates a ThreadView with resolved anchor and full title.
func (s *Service) buildView(t *thread.Thread) ThreadView {
	view := ThreadView{
		Thread: t,
		Title:  fullTitle(t),
	}

	if anchor := t.CurrentAnchor(); anchor != nil {
		result, err := trace.TraceToWorkingTree(s.repoRoot, *anchor)
		if err == nil {
			view.ResolvedAnchor = &ResolvedAnchor{
				File:     result.Anchor.File,
				Line:     result.Anchor.Line,
				Outdated: result.Outdated,
			}
		}
	}

	return view
}

// fullTitle extracts the full first line of the opening comment.
func fullTitle(t *thread.Thread) string {
	if len(t.Comments) == 0 || len(t.Comments[0].Bodies) == 0 {
		return ""
	}
	content := strings.TrimSpace(t.Comments[0].Bodies[0].Content)
	if idx := strings.IndexByte(content, '\n'); idx != -1 {
		content = content[:idx]
	}
	return strings.TrimSpace(content)
}

// matchesFile checks if the thread's resolved anchor matches the given file path.
func (s *Service) matchesFile(view ThreadView, file string) bool {
	if view.ResolvedAnchor == nil {
		return false
	}

	normalized, err := s.normalizePath(file)
	if err != nil {
		return false
	}

	return view.ResolvedAnchor.File == normalized
}

// normalizePath converts an absolute path to repo-relative.
// Returns empty string and error if path is outside repo root.
func (s *Service) normalizePath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return path, nil
	}

	rel, err := filepath.Rel(s.repoRoot, path)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %s is outside repository root", path)
	}

	return rel, nil
}
