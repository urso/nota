// Package tracking parses .nota/ tracking files (frontmatter + section headings).
package tracking

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// File represents a parsed .nota/ tracking file.
type File struct {
	Path       string
	Status     string
	Group      string
	DependsOn  []string
	References []string
	Tags       []string
	Sections   []Section
}

// Section represents a ## heading in a tracking file.
type Section struct {
	Tag        string
	Resolution string // "resolved", "wontfix", or "" for open
	Location   string // "file:line"
}

// IsOpen returns true if the section has no resolution.
func (s Section) IsOpen() bool { return s.Resolution == "" }

var reHeading = regexp.MustCompile(`^## (?:\[(resolved|wontfix)\] )?([a-z][a-z0-9_-]*) — (.+)$`)

// ParseFile reads and parses a tracking file.
func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, string(data))
}

// Parse parses tracking file content.
func Parse(path, content string) (*File, error) {
	f := &File{Path: path}

	lines := strings.Split(content, "\n")
	i := 0

	// Parse frontmatter.
	if i < len(lines) && strings.TrimSpace(lines[i]) == "---" {
		i++
		var fmLines []string
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "---" {
				i++
				break
			}
			fmLines = append(fmLines, lines[i])
			i++
		}
		parseFrontmatter(f, fmLines)
	}

	// Parse section headings.
	for ; i < len(lines); i++ {
		if m := reHeading.FindStringSubmatch(lines[i]); m != nil {
			f.Sections = append(f.Sections, Section{
				Resolution: m[1],
				Tag:        m[2],
				Location:   m[3],
			})
		}
	}

	return f, nil
}

// parseFrontmatter extracts fields from frontmatter lines.
func parseFrontmatter(f *File, lines []string) {
	i := 0
	for i < len(lines) {
		line := lines[i]
		if k, v, ok := scalarField(line); ok {
			switch k {
			case "status":
				f.Status = v
			case "group":
				f.Group = v
			}
			i++
			continue
		}

		if k, ok := listFieldKey(line); ok {
			// Check inline form: key: [a, b]
			if vals, ok := inlineList(line); ok {
				setList(f, k, vals)
				i++
				continue
			}
			// Multi-line form.
			i++
			var vals []string
			for i < len(lines) {
				if item, ok := listItem(lines[i]); ok {
					vals = append(vals, item)
					i++
				} else {
					break
				}
			}
			setList(f, k, vals)
			continue
		}

		i++
	}
}

func scalarField(line string) (key, value string, ok bool) {
	k, v, found := strings.Cut(line, ":")
	if !found {
		return "", "", false
	}
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "[") {
		return "", "", false
	}
	// Skip if next char after colon looks like a list start.
	return k, v, true
}

func listFieldKey(line string) (string, bool) {
	k, v, found := strings.Cut(line, ":")
	if !found {
		return "", false
	}
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	switch k {
	case "depends-on", "references", "tags":
		if v == "" || strings.HasPrefix(v, "[") {
			return k, true
		}
	}
	return "", false
}

func inlineList(line string) ([]string, bool) {
	_, v, _ := strings.Cut(line, ":")
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, false
	}
	inner := v[1 : len(v)-1]
	if inner == "" {
		return nil, true
	}
	parts := strings.Split(inner, ",")
	vals := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			vals = append(vals, p)
		}
	}
	return vals, true
}

func listItem(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "- ") {
		return "", false
	}
	return strings.TrimSpace(trimmed[2:]), true
}

func setList(f *File, key string, vals []string) {
	switch key {
	case "depends-on":
		f.DependsOn = vals
	case "references":
		f.References = vals
	case "tags":
		f.Tags = vals
	}
}

// Validate checks a tracking file for errors. Returns a list of error messages.
func Validate(f *File) []string {
	var errs []string

	if f.Status == "" {
		errs = append(errs, "Missing 'status' field in frontmatter")
	} else if f.Status != "open" && f.Status != "resolved" {
		errs = append(errs, fmt.Sprintf("Invalid status '%s' — must be 'open' or 'resolved'", f.Status))
	}

	// Validate dependency/reference targets exist.
	if f.Path != "" {
		dir := filepath.Dir(f.Path)
		for _, target := range f.DependsOn {
			p := filepath.Join(dir, target+".md")
			if _, err := os.Stat(p); err != nil {
				errs = append(errs, fmt.Sprintf("depends-on target '%s' not found (%s)", target, p))
			}
		}
		for _, target := range f.References {
			p := filepath.Join(dir, target+".md")
			if _, err := os.Stat(p); err != nil {
				errs = append(errs, fmt.Sprintf("references target '%s' not found (%s)", target, p))
			}
		}
	}

	// Status consistency.
	total := len(f.Sections)
	resolved := 0
	for _, s := range f.Sections {
		if s.Resolution != "" {
			resolved++
		}
	}
	if total > 0 && total == resolved && f.Status == "open" {
		errs = append(errs, "All sections are resolved/wontfix but status is still 'open' — update frontmatter to 'status: resolved'")
	}
	if total > 0 && total != resolved && f.Status == "resolved" {
		errs = append(errs, "Status is 'resolved' but there are open sections — status should be 'open'")
	}

	return errs
}

// ValidateHeadings checks section headings in raw content. Returns bad heading lines.
func ValidateHeadings(content string) []string {
	var bad []string
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") && !reHeading.MatchString(line) {
			bad = append(bad, line)
		}
	}
	return bad
}

// ValidateFile reads a tracking file and runs all validations:
// frontmatter presence, structure, headings, status consistency, and dependency targets.
// Returns a list of error messages (empty if valid).
func ValidateFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	var errs []string

	if !strings.HasPrefix(content, "---\n") {
		errs = append(errs, "Missing frontmatter (file must start with ---)")
	}

	f, err := Parse(path, content)
	if err != nil {
		return nil, err
	}

	errs = append(errs, Validate(f)...)

	badHeadings := ValidateHeadings(content)
	if len(badHeadings) > 0 {
		var buf strings.Builder
		buf.WriteString("Invalid section headings:\n")
		for _, h := range badHeadings {
			buf.WriteString(h)
			buf.WriteByte('\n')
		}
		buf.WriteString("Expected format: ## [resolved|wontfix]? <tag> — file:line")
		errs = append(errs, buf.String())
	}

	return errs, nil
}

// ReadOpen reads a tracking file and returns content with resolved/wontfix sections stripped.
func ReadOpen(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return FilterOpen(string(data)), nil
}

// FilterOpen strips resolved/wontfix sections from tracking file content.
func FilterOpen(content string) string {
	var buf strings.Builder
	skip := false
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "## [resolved]") || strings.HasPrefix(line, "## [wontfix]") {
			skip = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			skip = false
		}
		if !skip {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	// Trim trailing extra newline from split.
	s := buf.String()
	if strings.HasSuffix(s, "\n\n") && !strings.HasSuffix(content, "\n\n") {
		s = s[:len(s)-1]
	}
	return s
}

// FindOptions specifies filters for finding tracking files.
type FindOptions struct {
	Dir          string // .nota/ directory path
	Name         string // filename stem lookup
	Status       string // filter by status
	Tag          string // filter by tag
	RefsOf       string // list files that <name> references
	DepsOf       string // list files that <name> depends on
	ReferencedBy string // list files that reference <name>
	BlockedBy    string // list files that depend on <name>
}

// Find queries .nota/ tracking files matching the given options.
// Returns absolute paths of matching files.
func Find(opts FindOptions) ([]string, error) {
	dir := opts.Dir
	if dir == "" {
		return nil, nil
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	// Resolve forward relations: refs-of, deps-of.
	if opts.RefsOf != "" {
		return resolveList(dir, opts.RefsOf, "references")
	}
	if opts.DepsOf != "" {
		return resolveList(dir, opts.DepsOf, "depends-on")
	}

	// Resolve reverse relations.
	if opts.ReferencedBy != "" {
		return resolveReverse(dir, opts.ReferencedBy, "references")
	}
	if opts.BlockedBy != "" {
		return resolveReverse(dir, opts.BlockedBy, "depends-on")
	}

	// Direct name lookup.
	if opts.Name != "" {
		p := filepath.Join(dir, opts.Name+".md")
		if _, err := os.Stat(p); err != nil {
			return nil, nil
		}
		f, err := ParseFile(p)
		if err != nil {
			return nil, err
		}
		if matchesFilters(f, opts.Status, opts.Tag) {
			abs, _ := filepath.Abs(p)
			return []string{abs}, nil
		}
		return nil, nil
	}

	// Filter all files.
	return filterAll(dir, opts.Status, opts.Tag)
}

func resolveList(dir, name, field string) ([]string, error) {
	p := filepath.Join(dir, name+".md")
	f, err := ParseFile(p)
	if err != nil {
		return nil, nil
	}

	var list []string
	switch field {
	case "references":
		list = f.References
	case "depends-on":
		list = f.DependsOn
	}

	var result []string
	for _, target := range list {
		tp := filepath.Join(dir, target+".md")
		if _, err := os.Stat(tp); err == nil {
			abs, _ := filepath.Abs(tp)
			result = append(result, abs)
		}
	}
	return result, nil
}

func resolveReverse(dir, name, field string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var result []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		f, err := ParseFile(p)
		if err != nil {
			continue
		}
		var list []string
		switch field {
		case "references":
			list = f.References
		case "depends-on":
			list = f.DependsOn
		}
		if slices.Contains(list, name) {
			abs, _ := filepath.Abs(p)
			result = append(result, abs)
		}
	}
	return result, nil
}

func filterAll(dir, status, tag string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var result []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		f, err := ParseFile(p)
		if err != nil {
			continue
		}
		if matchesFilters(f, status, tag) {
			abs, _ := filepath.Abs(p)
			result = append(result, abs)
		}
	}
	return result, nil
}

func matchesFilters(f *File, status, tag string) bool {
	if status != "" && f.Status != status {
		return false
	}
	if tag != "" && !slices.Contains(f.Tags, tag) {
		return false
	}
	return true
}
