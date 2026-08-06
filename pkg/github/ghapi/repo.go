package ghapi

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
)

// Repo identifies a GitHub repository.
type Repo struct {
	Owner string
	Name  string
	Host  string
}

// String returns "owner/name".
func (r Repo) String() string {
	return r.Owner + "/" + r.Name
}

// FullName returns "host/owner/name" for non-github.com hosts, otherwise "owner/name".
func (r Repo) FullName() string {
	if r.Host != "" && r.Host != defaultHost {
		return r.Host + "/" + r.Owner + "/" + r.Name
	}
	return r.String()
}

// ResolveRepo determines the repository from git remotes or an explicit override.
// If override is non-empty (format: "owner/name"), it is parsed and used.
// Otherwise, repository.Current() resolves from git remotes.
func ResolveRepo(override string) (Repo, error) {
	if override != "" {
		return ParseRepo(override)
	}

	r, err := repository.Current()
	if err != nil {
		return Repo{}, fmt.Errorf("resolving repository: %w", err)
	}

	return Repo{
		Owner: r.Owner,
		Name:  r.Name,
		Host:  r.Host,
	}, nil
}

// ParseRepo parses an "owner/name" string into a Repo.
func ParseRepo(s string) (Repo, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Repo{}, fmt.Errorf("empty repository string")
	}

	parts := strings.Split(s, "/")
	switch len(parts) {
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return Repo{}, fmt.Errorf("invalid repository format %q: expected owner/name", s)
		}
		return Repo{Owner: parts[0], Name: parts[1], Host: defaultHost}, nil
	case 3:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return Repo{}, fmt.Errorf("invalid repository format %q: expected host/owner/name", s)
		}
		return Repo{Host: parts[0], Owner: parts[1], Name: parts[2]}, nil
	default:
		return Repo{}, fmt.Errorf("invalid repository format %q: expected owner/name or host/owner/name", s)
	}
}

// RepoMatches compares two repos as bare "owner/name", ignoring host.
// This matches the design decision that GHES pulls would match github.com threads.
func RepoMatches(a, b Repo) bool {
	return strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Name, b.Name)
}
