package git

// Ops abstracts git operations to enable caching across multiple calls.
type Ops interface {
	// GetDiffs returns the sequence of diffs for a file from fromCommit to toCommit.
	GetDiffs(fromCommit, toCommit, filePath string) ([]Diff, error)

	// IsAncestorOf checks if ancestorCommit is an ancestor of descendantCommit.
	IsAncestorOf(ancestorCommit, descendantCommit string) (bool, error)

	// MergeBase returns the best common ancestor of two commits.
	MergeBase(commit1, commit2 string) (string, error)

	// RepoDir returns the repository root directory.
	RepoDir() string
}

// directOps implements Ops by calling git directly without caching.
type directOps struct {
	repoDir string
}

// NewOps creates an Ops that calls git directly without caching.
func NewOps(repoDir string) Ops {
	return &directOps{repoDir: repoDir}
}

func (g *directOps) RepoDir() string {
	return g.repoDir
}

func (g *directOps) GetDiffs(fromCommit, toCommit, filePath string) ([]Diff, error) {
	return GetDiffs(g.repoDir, fromCommit, toCommit, filePath)
}

func (g *directOps) IsAncestorOf(ancestorCommit, descendantCommit string) (bool, error) {
	return IsAncestorOf(g.repoDir, ancestorCommit, descendantCommit)
}

func (g *directOps) MergeBase(commit1, commit2 string) (string, error) {
	return MergeBase(g.repoDir, commit1, commit2)
}

// cachedOps wraps an Ops and caches results.
type cachedOps struct {
	inner          Ops
	diffCache      map[string]diffCacheEntry
	ancestorCache  map[string]bool
	mergeBaseCache map[string]string
}

type diffCacheEntry struct {
	diffs []Diff
	err   error
}

// NewCachedOps creates an Ops that caches results from the underlying Ops.
func NewCachedOps(inner Ops) Ops {
	return &cachedOps{
		inner:          inner,
		diffCache:      make(map[string]diffCacheEntry),
		ancestorCache:  make(map[string]bool),
		mergeBaseCache: make(map[string]string),
	}
}

func (g *cachedOps) RepoDir() string {
	return g.inner.RepoDir()
}

func (g *cachedOps) GetDiffs(fromCommit, toCommit, filePath string) ([]Diff, error) {
	key := fromCommit + ":" + toCommit + ":" + filePath
	if entry, ok := g.diffCache[key]; ok {
		return entry.diffs, entry.err
	}
	diffs, err := g.inner.GetDiffs(fromCommit, toCommit, filePath)
	g.diffCache[key] = diffCacheEntry{diffs: diffs, err: err}
	return diffs, err
}

func (g *cachedOps) IsAncestorOf(ancestorCommit, descendantCommit string) (bool, error) {
	key := ancestorCommit + ":" + descendantCommit
	if result, ok := g.ancestorCache[key]; ok {
		return result, nil
	}
	result, err := g.inner.IsAncestorOf(ancestorCommit, descendantCommit)
	if err == nil {
		g.ancestorCache[key] = result
	}
	return result, err
}

func (g *cachedOps) MergeBase(commit1, commit2 string) (string, error) {
	key := commit1 + ":" + commit2
	if commit2 < commit1 {
		key = commit2 + ":" + commit1
	}
	if result, ok := g.mergeBaseCache[key]; ok {
		return result, nil
	}
	result, err := g.inner.MergeBase(commit1, commit2)
	if err == nil {
		g.mergeBaseCache[key] = result
	}
	return result, err
}
