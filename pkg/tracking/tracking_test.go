package tracking

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_basic(t *testing.T) {
	content := `---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix this

## [resolved] review — src/auth/login.go:78

> Fixed
`
	f, err := Parse("test.md", content)
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != "open" {
		t.Errorf("status = %q, want open", f.Status)
	}
	if f.Group != "auth" {
		t.Errorf("group = %q, want auth", f.Group)
	}
	if len(f.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(f.Sections))
	}
	if f.Sections[0].Tag != "review" || f.Sections[0].Resolution != "" {
		t.Errorf("section 0: tag=%q resolution=%q", f.Sections[0].Tag, f.Sections[0].Resolution)
	}
	if f.Sections[1].Tag != "review" || f.Sections[1].Resolution != "resolved" {
		t.Errorf("section 1: tag=%q resolution=%q", f.Sections[1].Tag, f.Sections[1].Resolution)
	}
}

func TestParse_dependsOnAndReferences(t *testing.T) {
	content := `---
status: open
depends-on:
  - session
  - config
references:
  - auth-legacy
tags:
  - security
  - auth
---

## review — src/foo.go:1

Fix
`
	f, err := Parse("test.md", content)
	if err != nil {
		t.Fatal(err)
	}
	assertSlice(t, "depends-on", f.DependsOn, []string{"session", "config"})
	assertSlice(t, "references", f.References, []string{"auth-legacy"})
	assertSlice(t, "tags", f.Tags, []string{"security", "auth"})
}

func TestParse_inlineLists(t *testing.T) {
	content := `---
status: open
depends-on: [session, config]
references: [auth-legacy]
tags: [security, auth]
---

## review — src/foo.go:1

Fix
`
	f, err := Parse("test.md", content)
	if err != nil {
		t.Fatal(err)
	}
	assertSlice(t, "depends-on", f.DependsOn, []string{"session", "config"})
	assertSlice(t, "references", f.References, []string{"auth-legacy"})
	assertSlice(t, "tags", f.Tags, []string{"security", "auth"})
}

func TestParse_extensionTags(t *testing.T) {
	content := `---
status: open
---

## impl — src/foo.go:1

Implement

## refactor — src/foo.go:10

Refactor

## critique — src/foo.go:20

Challenge
`
	f, err := Parse("test.md", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Sections) != 3 {
		t.Fatalf("sections = %d, want 3", len(f.Sections))
	}
	tags := []string{"impl", "refactor", "critique"}
	for i, want := range tags {
		if f.Sections[i].Tag != want {
			t.Errorf("section %d: tag=%q, want %q", i, f.Sections[i].Tag, want)
		}
	}
}

func TestValidate_missingStatus(t *testing.T) {
	f := &File{}
	errs := Validate(f)
	if len(errs) != 1 || errs[0] != "Missing 'status' field in frontmatter" {
		t.Errorf("errs = %v", errs)
	}
}

func TestValidate_invalidStatus(t *testing.T) {
	f := &File{Status: "pending"}
	errs := Validate(f)
	if len(errs) != 1 {
		t.Errorf("errs = %v", errs)
	}
}

func TestValidate_statusConsistency(t *testing.T) {
	f := &File{
		Status: "open",
		Sections: []Section{
			{Tag: "review", Resolution: "resolved"},
			{Tag: "review", Resolution: "wontfix"},
		},
	}
	errs := Validate(f)
	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}
	if errs[0] != "All sections are resolved/wontfix but status is still 'open' — update frontmatter to 'status: resolved'" {
		t.Errorf("err = %q", errs[0])
	}
}

func TestValidate_dependsOnTarget(t *testing.T) {
	dir := t.TempDir()
	// Create target file.
	os.WriteFile(filepath.Join(dir, "exists.md"), []byte("---\nstatus: open\n---\n"), 0o644)

	f := &File{
		Path:      filepath.Join(dir, "main.md"),
		Status:    "open",
		DependsOn: []string{"exists", "gone"},
	}
	errs := Validate(f)
	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}
	if !contains(errs[0], "depends-on target 'gone' not found") {
		t.Errorf("err = %q", errs[0])
	}
}

func TestValidateHeadings(t *testing.T) {
	content := `---
status: open
---

## review — src/foo.go:1

Fix

## Bad heading without tag format

## [resolved] discuss — src/bar.go:5

> Done
`
	bad := ValidateHeadings(content)
	if len(bad) != 1 || bad[0] != "## Bad heading without tag format" {
		t.Errorf("bad = %v", bad)
	}
}

func TestValidateFile_valid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "good.md")
	os.WriteFile(p, []byte("---\nstatus: open\n---\n\n## review — src/foo.go:1\n\nFix\n"), 0o644)

	errs, err := ValidateFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateFile_missingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.md")
	os.WriteFile(p, []byte("no frontmatter\n\n## review — src/foo.go:1\n\nFix\n"), 0o644)

	errs, err := ValidateFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) < 1 {
		t.Fatal("expected errors")
	}
	if !contains(errs[0], "Missing frontmatter") {
		t.Errorf("errs = %v", errs)
	}
}

func TestValidateFile_badHeadingAndStatus(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.md")
	os.WriteFile(p, []byte("---\nstatus: pending\n---\n\n## bad heading\n\nFix\n"), 0o644)

	errs, err := ValidateFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Should have both invalid status and invalid heading errors.
	hasStatus := false
	hasHeading := false
	for _, e := range errs {
		if contains(e, "Invalid status") {
			hasStatus = true
		}
		if contains(e, "Invalid section headings") {
			hasHeading = true
		}
	}
	if !hasStatus || !hasHeading {
		t.Errorf("expected status and heading errors, got %v", errs)
	}
}

func TestValidateFile_missingDep(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "main.md")
	os.WriteFile(p, []byte("---\nstatus: open\ndepends-on:\n  - gone\n---\n\n## review — src/foo.go:1\n\nFix\n"), 0o644)

	errs, err := ValidateFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 || !contains(errs[0], "depends-on target 'gone' not found") {
		t.Errorf("errs = %v", errs)
	}
}

func TestFilterOpen(t *testing.T) {
	content := `---
status: open
---

## review — src/foo.go:1

Fix this

## [resolved] review — src/foo.go:10

> Fixed

## discuss — src/bar.go:5

Talk about this
`
	got := FilterOpen(content)
	if contains(got, "[resolved]") {
		t.Errorf("resolved section not stripped: %s", got)
	}
	if !contains(got, "## review — src/foo.go:1") {
		t.Errorf("open section missing: %s", got)
	}
	if !contains(got, "## discuss — src/bar.go:5") {
		t.Errorf("discuss section missing: %s", got)
	}
}

func TestFind_byName(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "auth.md"), []byte("---\nstatus: open\n---\n\n## review — src/auth.go:1\n\nFix\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, Name: "auth"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "auth.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_byStatus(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "open1.md"), []byte("---\nstatus: open\n---\n\n## review — src/a.go:1\n\nFix\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "done1.md"), []byte("---\nstatus: resolved\n---\n\n## [resolved] review — src/b.go:1\n\n> Fixed\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "open1.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_byTag(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "sec.md"), []byte("---\nstatus: open\ntags:\n  - security\n---\n\n## review — src/a.go:1\n\nFix\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "other.md"), []byte("---\nstatus: open\ntags:\n  - perf\n---\n\n## review — src/b.go:1\n\nFix\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, Tag: "security"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "sec.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_depsOf(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "dep.md"), []byte("---\nstatus: open\n---\n\n## review — src/a.go:1\n\nFix\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "main.md"), []byte("---\nstatus: open\ndepends-on:\n  - dep\n---\n\n## review — src/b.go:1\n\nFix\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, DepsOf: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "dep.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_refsOf(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "target.md"), []byte("---\nstatus: resolved\n---\n\n## [resolved] review — src/a.go:1\n\n> Fixed\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "source.md"), []byte("---\nstatus: open\nreferences:\n  - target\n---\n\n## review — src/b.go:1\n\nFix\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, RefsOf: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "target.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_blockedBy(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "blocker.md"), []byte("---\nstatus: open\n---\n\n## review — src/a.go:1\n\nFix\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "blocked.md"), []byte("---\nstatus: open\ndepends-on:\n  - blocker\n---\n\n## review — src/b.go:1\n\nFix\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "other.md"), []byte("---\nstatus: open\n---\n\n## review — src/c.go:1\n\nFix\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, BlockedBy: "blocker"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "blocked.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_referencedBy(t *testing.T) {
	dir := t.TempDir()
	nota := filepath.Join(dir, ".nota")
	os.MkdirAll(nota, 0o755)
	os.WriteFile(filepath.Join(nota, "target.md"), []byte("---\nstatus: resolved\n---\n\n## [resolved] review — src/a.go:1\n\n> Fixed\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "a.md"), []byte("---\nstatus: open\nreferences:\n  - target\n---\n\n## review — src/b.go:1\n\nFix\n"), 0o644)
	os.WriteFile(filepath.Join(nota, "b.md"), []byte("---\nstatus: open\n---\n\n## review — src/c.go:1\n\nFix\n"), 0o644)

	results, err := Find(FindOptions{Dir: nota, ReferencedBy: "target"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %v", results)
	}
	if filepath.Base(results[0]) != "a.md" {
		t.Errorf("got %s", results[0])
	}
}

func TestFind_noDir(t *testing.T) {
	results, err := Find(FindOptions{Dir: "/nonexistent/path"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty, got %v", results)
	}
}

func assertSlice(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", name, i, got[i], want[i])
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
