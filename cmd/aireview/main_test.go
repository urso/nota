package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/urso/aireview/pkg/deleter"
	"github.com/urso/aireview/pkg/formatter"
	"github.com/urso/aireview/pkg/grouper"
	"github.com/urso/aireview/pkg/parser"
	"gopkg.in/yaml.v3"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTestFileWithPerm(t *testing.T, dir, name, content string, perm os.FileMode) string {
	t.Helper()
	path := writeTestFile(t, dir, name, content)
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- Full Pipeline: scan → parse → group → format ---

func TestFullPipelineMarkdown(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "auth.go", `package auth

func validate() {
	// review(auth): check token expiry
	token := getToken()
	return token.Valid()
}

// review: standalone comment
func other() {}
`)

	pyFile := writeTestFile(t, dir, "helper.py", `# see: auth
def helper():
    pass
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile, pyFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	var buf bytes.Buffer
	if err := formatter.FormatMarkdown(&buf, groups); err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	// Named group "auth" should appear with entry and reference.
	if !strings.Contains(out, "## auth") {
		t.Error("missing 'auth' group heading")
	}
	if !strings.Contains(out, "check token expiry") {
		t.Error("missing comment message for auth entry")
	}
	if !strings.Contains(out, "see —") {
		t.Error("missing see reference in auth group")
	}

	// Standalone unnamed comment should appear.
	if !strings.Contains(out, "## (unnamed)") {
		t.Error("missing unnamed group heading")
	}
	if !strings.Contains(out, "standalone comment") {
		t.Error("missing standalone comment message")
	}

	// Context code should appear in fenced blocks.
	if !strings.Contains(out, "````go") {
		t.Error("missing go code fence")
	}
	if !strings.Contains(out, "````python") {
		t.Error("missing python code fence")
	}
	if !strings.Contains(out, ">>> comment <<<") {
		t.Error("missing comment marker in context")
	}
}

func TestFullPipelineYAMLStructure(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "auth.go", `package auth

// review(auth): check token
func validate() {}

// discuss(auth): is this safe?
func other() {}
`)

	pyFile := writeTestFile(t, dir, "ref.py", `# also: auth
def helper():
    pass
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile, pyFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	var buf bytes.Buffer
	if err := formatter.FormatYAML(&buf, groups); err != nil {
		t.Fatal(err)
	}

	// Unmarshal into typed structs to validate structure.
	type yamlContext struct {
		Before []string `yaml:"before"`
		After  []string `yaml:"after"`
	}
	type yamlEntry struct {
		Tag     string      `yaml:"tag"`
		File    string      `yaml:"file"`
		Line    int         `yaml:"line"`
		Comment string      `yaml:"comment"`
		Context yamlContext `yaml:"context"`
	}
	type yamlRef struct {
		File    string      `yaml:"file"`
		Line    int         `yaml:"line"`
		Context yamlContext `yaml:"context"`
	}
	type yamlGroup struct {
		Name       string     `yaml:"name"`
		Entries    []yamlEntry `yaml:"entries"`
		References []yamlRef   `yaml:"references"`
	}

	var parsed []yamlGroup
	if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, buf.String())
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 group, got %d", len(parsed))
	}

	g := parsed[0]
	if g.Name != "auth" {
		t.Errorf("expected group name 'auth', got %q", g.Name)
	}
	if len(g.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(g.Entries))
	}
	if len(g.References) != 1 {
		t.Errorf("expected 1 reference, got %d", len(g.References))
	}

	// Verify entry tags.
	tags := make(map[string]bool)
	for _, e := range g.Entries {
		tags[e.Tag] = true
	}
	if !tags["review"] {
		t.Error("missing review entry")
	}
	if !tags["discuss"] {
		t.Error("missing discuss entry")
	}

	// Verify reference has a file.
	if len(g.References) > 0 && !strings.HasSuffix(g.References[0].File, "ref.py") {
		t.Errorf("expected reference from ref.py, got %s", g.References[0].File)
	}

	// Verify context arrays are not null (F12 fix).
	for _, e := range g.Entries {
		if e.Context.Before == nil {
			t.Error("context.before is null, expected empty array")
		}
		if e.Context.After == nil {
			t.Error("context.after is null, expected empty array")
		}
	}
}

func TestFullPipelineContextZero(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", `package main

func hello() {
	// review: no context please
	return
}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 0)

	for _, g := range groups {
		for _, e := range g.Entries {
			if len(e.Context.Before) > 0 || len(e.Context.After) > 0 {
				t.Error("expected no context with context=0")
			}
		}
	}

	// Verify markdown output has no code fences when context=0.
	var buf bytes.Buffer
	if err := formatter.FormatMarkdown(&buf, groups); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "````") {
		t.Error("expected no code fences with context=0")
	}
}

// --- Full Pipeline: scan → parse → group → delete ---

func TestFullPipelineDelete(t *testing.T) {
	dir := t.TempDir()

	src := "package main\n\n// review(fix): remove this\nfunc hello() {\n\treturn\n}\n"
	goFile := writeTestFile(t, dir, "code.go", src)

	_, fileContents, fileRanges, filePerms, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(result), "review(fix)") {
		t.Error("comment was not deleted")
	}
	if !strings.Contains(string(result), "func hello()") {
		t.Error("code was incorrectly removed")
	}
}

func TestFullPipelineDeleteMultipleComments(t *testing.T) {
	dir := t.TempDir()

	src := `package main

// review(a): first comment
func first() {}

// review(b): second comment
func second() {}

// discuss(a): third comment
func third() {}
`
	goFile := writeTestFile(t, dir, "multi.go", src)

	comments, fileContents, fileRanges, filePerms, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(comments))
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	if strings.Contains(got, "review") || strings.Contains(got, "discuss") {
		t.Error("not all comments were deleted")
	}
	if !strings.Contains(got, "func first()") || !strings.Contains(got, "func second()") || !strings.Contains(got, "func third()") {
		t.Error("code was incorrectly removed")
	}
}

func TestFullPipelineDeletePreservesPermissions(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFileWithPerm(t, dir, "perm.go",
		"package main\n// review: delete me\nfunc hello() {}\n", 0o755)

	_, fileContents, fileRanges, filePerms, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(goFile)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Errorf("expected permissions 0755, got %o", info.Mode().Perm())
	}
}

func TestCommentOnlyLineFullyRemoved(t *testing.T) {
	dir := t.TempDir()
	goFile := writeTestFile(t, dir, "code.go", "package main\n\n    // review: msg\nfunc hello() {}\n")

	_, fileContents, fileRanges, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	modified, err := deleter.DeleteComments(fileContents[goFile], fileRanges[goFile])
	if err != nil {
		t.Fatal(err)
	}

	expected := "package main\n\nfunc hello() {}\n"
	if string(modified) != expected {
		t.Errorf("expected %q, got %q", expected, string(modified))
	}
}

func TestTrailingCommentPreservesCode(t *testing.T) {
	dir := t.TempDir()
	goFile := writeTestFile(t, dir, "code.go", "package main\n\nx := 1 // review: msg\nfunc hello() {}\n")

	_, fileContents, fileRanges, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	modified, err := deleter.DeleteComments(fileContents[goFile], fileRanges[goFile])
	if err != nil {
		t.Fatal(err)
	}

	result := string(modified)
	if !strings.Contains(result, "x := 1") {
		t.Error("code before trailing comment was removed")
	}
	if strings.Contains(result, "review") {
		t.Error("comment was not removed")
	}
}

// --- Multi-language tests ---

func TestPythonComments(t *testing.T) {
	dir := t.TempDir()

	pyFile := writeTestFile(t, dir, "app.py", `# review(auth): validate tokens
def validate():
    pass

# explain: why this pattern
def other():
    pass
`)

	comments, fileContents, _, _, err := processFiles([]string{pyFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	var buf bytes.Buffer
	if err := formatter.FormatMarkdown(&buf, groups); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "## auth") {
		t.Error("missing auth group for Python file")
	}
	if !strings.Contains(out, "validate tokens") {
		t.Error("missing Python comment message")
	}
	if !strings.Contains(out, "````python") {
		t.Error("missing python language tag on code fence")
	}
}

func TestPythonDeleteComments(t *testing.T) {
	dir := t.TempDir()

	pyFile := writeTestFile(t, dir, "app.py", "# review: delete me\ndef hello():\n    pass\n")

	_, fileContents, fileRanges, filePerms, err := processFiles([]string{pyFile})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(pyFile)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	if strings.Contains(got, "review") {
		t.Error("Python comment was not deleted")
	}
	if !strings.Contains(got, "def hello()") {
		t.Error("Python code was incorrectly removed")
	}
}

func TestCBlockComments(t *testing.T) {
	dir := t.TempDir()

	cFile := writeTestFile(t, dir, "main.c", `#include <stdio.h>

/* review(perf): check allocation
 * this might leak memory
 * in the hot path */
int main() {
    return 0;
}
`)

	comments, fileContents, _, _, err := processFiles([]string{cFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	c := comments[0]
	if c.Tag != parser.TagReview {
		t.Errorf("expected tag review, got %s", c.Tag)
	}
	if c.Name != "perf" {
		t.Errorf("expected name perf, got %s", c.Name)
	}
	if !strings.Contains(c.Message, "check allocation") {
		t.Errorf("expected message to contain 'check allocation', got %q", c.Message)
	}
	// Continuation lines should be joined (F10-style: * stripped).
	if !strings.Contains(c.Message, "this might leak memory") {
		t.Errorf("expected continuation 'this might leak memory' in message, got %q", c.Message)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	var buf bytes.Buffer
	if err := formatter.FormatMarkdown(&buf, groups); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "## perf") {
		t.Error("missing perf group for C block comment")
	}
}

func TestCBlockCommentDelete(t *testing.T) {
	dir := t.TempDir()

	cFile := writeTestFile(t, dir, "main.c", "#include <stdio.h>\n\n/* review: remove */\nint main() {\n    return 0;\n}\n")

	_, fileContents, fileRanges, filePerms, err := processFiles([]string{cFile})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(cFile)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	if strings.Contains(got, "review") {
		t.Error("C block comment was not deleted")
	}
	if !strings.Contains(got, "int main()") {
		t.Error("C code was incorrectly removed")
	}
}

func TestRubyComments(t *testing.T) {
	dir := t.TempDir()

	rbFile := writeTestFile(t, dir, "app.rb", `# review(auth): check session
class Auth
  def validate
    true
  end
end
`)

	comments, _, _, _, err := processFiles([]string{rbFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	if comments[0].Tag != parser.TagReview || comments[0].Name != "auth" {
		t.Errorf("unexpected comment: %+v", comments[0])
	}
}

// --- Cross-file grouping ---

func TestCrossFileGrouping(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTestFile(t, dir, "auth.go", `package auth

// review(security): validate input
func validateInput() {}
`)

	file2 := writeTestFile(t, dir, "token.go", `package auth

// discuss(security): rate limiting?
func rateLimit() {}
`)

	file3 := writeTestFile(t, dir, "ref.py", `# see: security
def audit():
    pass
`)

	comments, fileContents, _, _, err := processFiles([]string{file1, file2, file3})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(comments))
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	// Should produce 1 named group with 2 entries and 1 reference.
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.Name != "security" {
		t.Errorf("expected group name 'security', got %q", g.Name)
	}
	if len(g.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(g.Entries))
	}
	if len(g.References) != 1 {
		t.Errorf("expected 1 reference, got %d", len(g.References))
	}

	// Entries should have distinct tags.
	entryTags := map[parser.Tag]bool{}
	for _, e := range g.Entries {
		entryTags[e.Tag] = true
	}
	if !entryTags[parser.TagReview] || !entryTags[parser.TagDiscuss] {
		t.Errorf("expected review and discuss tags, got %v", entryTags)
	}
}

func TestCrossFileDeleteAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTestFile(t, dir, "a.go", "package main\n\n// review(x): comment a\nfunc a() {}\n")
	file2 := writeTestFile(t, dir, "b.go", "package main\n\n// review(x): comment b\nfunc b() {}\n")

	_, fileContents, fileRanges, filePerms, err := processFiles([]string{file1, file2})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	// Both files should have comments removed.
	for _, f := range []string{file1, file2} {
		result, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(result), "review") {
			t.Errorf("comment not deleted from %s", f)
		}
	}
}

// --- Extract pipeline (format + delete) ---

func TestExtractPipeline(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", `package main

// review(api): check error handling
func handleRequest() {
	resp := doRequest()
	return resp
}
`)

	comments, fileContents, fileRanges, filePerms, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	// 1. Format output.
	var mdBuf bytes.Buffer
	if err := writeOutput(&mdBuf, groups, "markdown"); err != nil {
		t.Fatal(err)
	}

	mdOutput := mdBuf.String()
	if !strings.Contains(mdOutput, "## api") {
		t.Error("markdown output missing api group")
	}
	if !strings.Contains(mdOutput, "check error handling") {
		t.Error("markdown output missing comment message")
	}

	// 2. Delete from files.
	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	if strings.Contains(got, "review") {
		t.Error("comment not deleted after extract")
	}
	if !strings.Contains(got, "func handleRequest()") {
		t.Error("code removed after extract")
	}

	// 3. Verify YAML output works too.
	var yamlBuf bytes.Buffer
	if err := writeOutput(&yamlBuf, groups, "yaml"); err != nil {
		t.Fatal(err)
	}
	var parsed interface{}
	if err := yaml.Unmarshal(yamlBuf.Bytes(), &parsed); err != nil {
		t.Errorf("invalid YAML from extract: %v", err)
	}
}

// --- Edge cases ---

func TestNoCommentsProducesEmptyOutput(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "clean.go", `package main

// regular comment, not a review tag
func hello() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}

	groups := grouper.GroupComments(comments, fileContents, 3)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}

	// Markdown output should be empty.
	var mdBuf bytes.Buffer
	if err := formatter.FormatMarkdown(&mdBuf, groups); err != nil {
		t.Fatal(err)
	}
	if mdBuf.Len() != 0 {
		t.Errorf("expected empty markdown output, got %q", mdBuf.String())
	}

	// YAML output should be empty array.
	var yamlBuf bytes.Buffer
	if err := formatter.FormatYAML(&yamlBuf, groups); err != nil {
		t.Fatal(err)
	}
	var parsed []interface{}
	if err := yaml.Unmarshal(yamlBuf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 0 {
		t.Errorf("expected empty YAML array, got %d elements", len(parsed))
	}
}

func TestBinaryFileSkipped(t *testing.T) {
	dir := t.TempDir()

	// Write a file with binary content (null bytes).
	binPath := filepath.Join(dir, "data.go")
	binContent := []byte("package main\n\x00\x00\x00// review: in binary\n")
	if err := os.WriteFile(binPath, binContent, 0o644); err != nil {
		t.Fatal(err)
	}

	goodFile := writeTestFile(t, dir, "good.go", "package main\n// review: msg\n")

	comments, _, _, _, err := processFiles([]string{binPath, goodFile})
	if err != nil {
		t.Fatal(err)
	}

	// Only the good file's comment should be found.
	if len(comments) != 1 {
		t.Errorf("expected 1 comment (binary skipped), got %d", len(comments))
	}
}

func TestUnreadableFileSkipped(t *testing.T) {
	dir := t.TempDir()
	goodFile := writeTestFile(t, dir, "good.go", "package main\n// review: msg\n")
	badFile := filepath.Join(dir, "nonexistent.go")

	comments, _, _, _, err := processFiles([]string{badFile, goodFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) == 0 {
		t.Error("expected comments from good file")
	}
}

func TestUnsupportedLanguageSkipped(t *testing.T) {
	dir := t.TempDir()

	// .xyz is not a known language.
	unknownFile := writeTestFile(t, dir, "data.xyz", "review: something\n")
	goodFile := writeTestFile(t, dir, "good.go", "package main\n// review: msg\n")

	comments, _, _, _, err := processFiles([]string{unknownFile, goodFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 1 {
		t.Errorf("expected 1 comment (unknown lang skipped), got %d", len(comments))
	}
}

func TestCaseSensitiveTags(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "case.go", `package main

// Review: capitalized
// REVIEW: uppercase
// review: lowercase
func hello() {}
`)

	comments, _, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	// Only lowercase "review:" should match.
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment (case sensitive), got %d", len(comments))
	}
	if comments[0].Message != "lowercase" {
		t.Errorf("expected message 'lowercase', got %q", comments[0].Message)
	}
}

// --- processFiles details ---

func TestProcessFilesReturnsCorrectCommentFields(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "fields.go", `package main

// review(auth): check this
func hello() {}

// explain: why pattern
func other() {}
`)

	comments, _, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	c1 := comments[0]
	if c1.Tag != parser.TagReview {
		t.Errorf("expected tag review, got %s", c1.Tag)
	}
	if c1.Name != "auth" {
		t.Errorf("expected name 'auth', got %q", c1.Name)
	}
	if c1.Message != "check this" {
		t.Errorf("expected message 'check this', got %q", c1.Message)
	}
	if c1.Line != 3 {
		t.Errorf("expected line 3, got %d", c1.Line)
	}
	if c1.EndLine != 3 {
		t.Errorf("expected endline 3, got %d", c1.EndLine)
	}
	if !strings.HasSuffix(c1.File, "fields.go") {
		t.Errorf("expected file ending in fields.go, got %s", c1.File)
	}

	c2 := comments[1]
	if c2.Tag != parser.TagExplain {
		t.Errorf("expected tag explain, got %s", c2.Tag)
	}
	if c2.Name != "" {
		t.Errorf("expected empty name, got %q", c2.Name)
	}
}

func TestProcessFilesReturnsByteRanges(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "ranges.go", "package main\n\n// review: delete\nfunc hello() {}\n")

	_, _, fileRanges, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	ranges, ok := fileRanges[goFile]
	if !ok {
		t.Fatal("no ranges for file")
	}
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}

	r := ranges[0]
	if r.Start >= r.End {
		t.Errorf("invalid range: start=%d >= end=%d", r.Start, r.End)
	}
	// The expanded range should cover the entire comment-only line including newline.
	// "package main\n\n" = 14 bytes (12 chars + 2 newlines)
	if r.Start != 14 {
		t.Errorf("expected start at 14, got %d", r.Start)
	}
}

// --- expandByteRange ---

func TestExpandByteRange(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		start    int64
		end      int64
		expected deleter.ByteRange
	}{
		{
			name:     "comment-only line",
			content:  "line1\n    // review: msg\nline3\n",
			start:    10, // start of "//"
			end:      24, // end of "msg"
			expected: deleter.ByteRange{Start: 6, End: 25}, // whole line including \n
		},
		{
			name:     "trailing comment",
			content:  "code // review: msg\nmore\n",
			start:    5, // start of "//"
			end:      19, // end of "msg"
			expected: deleter.ByteRange{Start: 4, End: 19}, // trims whitespace before //
		},
		{
			name:     "comment at file start",
			content:  "// review: msg\ncode\n",
			start:    0,
			end:      14,
			expected: deleter.ByteRange{Start: 0, End: 15}, // includes newline
		},
		{
			name:     "comment at file end no newline",
			content:  "code\n// review: msg",
			start:    5,
			end:      19,
			expected: deleter.ByteRange{Start: 5, End: 19}, // no trailing newline
		},
		{
			name:     "tab indented comment",
			content:  "code\n\t\t// review: msg\nmore\n",
			start:    7, // start of "//"
			end:      21,
			expected: deleter.ByteRange{Start: 5, End: 22}, // whole line
		},
		{
			name:     "negative start clamped",
			content:  "// review: msg\n",
			start:    -5,
			end:      14,
			expected: deleter.ByteRange{Start: 0, End: 15},
		},
		{
			name:     "oversized end clamped",
			content:  "// review: msg",
			start:    0,
			end:      100,
			expected: deleter.ByteRange{Start: 0, End: 14},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandByteRange([]byte(tt.content), tt.start, tt.end)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// --- looksLikeFilePath ---

func TestLooksLikeFilePath(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"file.go", true},
		{"src/main.go", true},
		{"main", false},
		{"list", false},
		{".", true},
		{"../file.py", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeFilePath(tt.input)
			if got != tt.expected {
				t.Errorf("looksLikeFilePath(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// --- writeOutput ---

func TestWriteOutput(t *testing.T) {
	groups := []grouper.Group{
		{
			Name: "test",
			Entries: []grouper.Entry{
				{Tag: parser.TagReview, File: "test.go", Line: 1, Comment: "hello"},
			},
		},
	}

	t.Run("markdown", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeOutput(&buf, groups, "markdown"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "## test") {
			t.Error("expected markdown heading")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeOutput(&buf, groups, "yaml"); err != nil {
			t.Fatal(err)
		}
		var parsed interface{}
		if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Errorf("invalid YAML: %v", err)
		}
	})

	t.Run("default is markdown", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeOutput(&buf, groups, "unknown-format"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "## test") {
			t.Error("expected markdown output for unknown format")
		}
	})
}

// --- Extract --dir tracking file tests ---

func TestExtractDirNamedGroup(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".aireview")

	goFile := writeTestFile(t, srcDir, "auth.go", `package auth

// review(auth): check token expiry
func validate() {
	token := getToken()
	return token.Valid()
}

// discuss(auth): is this safe?
func other() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	// Named group "auth" should produce auth.md.
	authFile := filepath.Join(outDir, "auth.md")
	content, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("expected auth.md to exist: %v", err)
	}

	s := string(content)

	// Check frontmatter.
	if !strings.Contains(s, "status: open") {
		t.Error("missing status: open in frontmatter")
	}
	if !strings.Contains(s, "group: auth") {
		t.Error("missing group: auth in frontmatter")
	}

	// Check section headings (## not ###).
	if !strings.Contains(s, "## review —") {
		t.Error("missing ## review heading")
	}
	if !strings.Contains(s, "## discuss —") {
		t.Error("missing ## discuss heading")
	}
	if !strings.Contains(s, "check token expiry") {
		t.Error("missing review comment text")
	}
}

func TestExtractDirUnnamedGroups(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".aireview")

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review: standalone one
func first() {}

// explain: why this
func second() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	// Unnamed groups should produce review-001.md, review-002.md.
	f1 := filepath.Join(outDir, "review-001.md")
	f2 := filepath.Join(outDir, "review-002.md")

	c1, err := os.ReadFile(f1)
	if err != nil {
		t.Fatalf("expected review-001.md: %v", err)
	}
	c2, err := os.ReadFile(f2)
	if err != nil {
		t.Fatalf("expected review-002.md: %v", err)
	}

	// Each should have frontmatter with status: open and no group field.
	for _, c := range [][]byte{c1, c2} {
		s := string(c)
		if !strings.Contains(s, "status: open") {
			t.Error("missing status: open")
		}
		if strings.Contains(s, "group:") {
			t.Error("unnamed group should not have group: in frontmatter")
		}
	}
}

func TestExtractDirAppendToExisting(t *testing.T) {
	outDir := t.TempDir()

	// Pre-create an existing tracking file.
	existing := "---\nstatus: open\ngroup: auth\n---\n\n## review — old.go:10\n\nold comment\n\n"
	if err := os.WriteFile(filepath.Join(outDir, "auth.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	srcDir := t.TempDir()
	goFile := writeTestFile(t, srcDir, "new.go", `package auth

// review(auth): new comment
func newFunc() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "auth.md"))
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)

	// Should contain both old and new content.
	if !strings.Contains(s, "old comment") {
		t.Error("existing content was lost")
	}
	if !strings.Contains(s, "new comment") {
		t.Error("new content was not appended")
	}
}

func TestExtractDirReopensResolvedFile(t *testing.T) {
	outDir := t.TempDir()

	// Pre-create a resolved tracking file.
	resolved := "---\nstatus: resolved\ngroup: auth\n---\n\n## [resolved] review — old.go:10\n\nold comment\n\n"
	if err := os.WriteFile(filepath.Join(outDir, "auth.md"), []byte(resolved), 0o644); err != nil {
		t.Fatal(err)
	}

	srcDir := t.TempDir()
	goFile := writeTestFile(t, srcDir, "new.go", `package auth

// review(auth): new comment
func newFunc() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "auth.md"))
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)

	// Status should be reopened.
	if strings.Contains(s, "status: resolved") {
		t.Error("file should have been reopened")
	}
	if !strings.Contains(s, "status: open") {
		t.Error("missing status: open after reopen")
	}
	if !strings.Contains(s, "new comment") {
		t.Error("new content was not appended")
	}
}

func TestExtractDirReviewNumbering(t *testing.T) {
	outDir := t.TempDir()

	// Pre-create review-001.md and review-003.md.
	for _, name := range []string{"review-001.md", "review-003.md"} {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte("---\nstatus: open\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	srcDir := t.TempDir()
	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review: new standalone
func hello() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	// Should use review-004.md (max existing is 003).
	f := filepath.Join(outDir, "review-004.md")
	if _, err := os.Stat(f); os.IsNotExist(err) {
		t.Error("expected review-004.md to be created")
	}
}

func TestExtractDirMixedNamedAndUnnamed(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".aireview")

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review(api): check error handling
func handleRequest() {}

// review: standalone fix
func other() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	// Named group produces api.md.
	if _, err := os.Stat(filepath.Join(outDir, "api.md")); os.IsNotExist(err) {
		t.Error("expected api.md")
	}

	// Unnamed group produces review-001.md.
	if _, err := os.Stat(filepath.Join(outDir, "review-001.md")); os.IsNotExist(err) {
		t.Error("expected review-001.md")
	}
}

func TestExtractDirWithDelete(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".aireview")

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review(api): check error handling
func handleRequest() {
	resp := doRequest()
	return resp
}
`)

	comments, fileContents, fileRanges, filePerms, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	// Write tracking files.
	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	// Delete comments from source.
	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	// Tracking file should exist with content.
	trackingContent, err := os.ReadFile(filepath.Join(outDir, "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(trackingContent), "check error handling") {
		t.Error("tracking file missing comment")
	}

	// Source file should have comment removed.
	srcContent, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(srcContent), "review") {
		t.Error("comment not deleted from source")
	}
	if !strings.Contains(string(srcContent), "func handleRequest()") {
		t.Error("code incorrectly removed from source")
	}
}

func TestExtractDirPathTraversalRejected(t *testing.T) {
	outDir := t.TempDir()

	badNames := []string{
		"../escape",
		"../../etc/evil",
		"sub/dir",
		"back\\slash",
		"..",
		".",
	}

	for _, name := range badNames {
		t.Run(name, func(t *testing.T) {
			groups := []grouper.Group{
				{Name: name, Entries: []grouper.Entry{{Tag: "review", File: "test.go", Line: 1, Comment: "test"}}},
			}
			err := writeTrackingFiles(outDir, groups)
			if err == nil {
				t.Errorf("expected error for group name %q, got nil", name)
			}
		})
	}
}

func TestExtractDirInvalidFilenameCharsRejected(t *testing.T) {
	outDir := t.TempDir()

	badChars := []string{"col:on", "less<than", "great>than", "pi|pe", "star*isk", "quest?ion", `qu"ote`}

	for _, name := range badChars {
		t.Run(name, func(t *testing.T) {
			groups := []grouper.Group{
				{Name: name, Entries: []grouper.Entry{{Tag: "review", File: "test.go", Line: 1, Comment: "test"}}},
			}
			err := writeTrackingFiles(outDir, groups)
			if err == nil {
				t.Errorf("expected error for group name %q, got nil", name)
			}
		})
	}
}

func TestReopenIfResolvedOnlyChangesFrontmatter(t *testing.T) {
	// File where "status: resolved" appears both in frontmatter and body.
	content := []byte("---\nstatus: resolved\ngroup: auth\n---\n\n## review — test.go:1\n\nThe status: resolved field should not change\n")

	result := reopenIfResolved(content)
	s := string(result)

	// Frontmatter should be reopened.
	if !strings.Contains(s, "status: open\ngroup: auth\n---") {
		t.Error("frontmatter status was not changed to open")
	}

	// Body text should NOT be modified.
	if !strings.Contains(s, "The status: resolved field should not change") {
		t.Error("body text was incorrectly modified")
	}
}

func TestExtractDirNoComments(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".aireview")

	goFile := writeTestFile(t, srcDir, "clean.go", `package main

func hello() {}
`)

	comments, fileContents, _, _, err := processFiles([]string{goFile})
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	if err := writeTrackingFiles(outDir, groups); err != nil {
		t.Fatal(err)
	}

	// Directory should be created but empty.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dir, got %d entries", len(entries))
	}
}
