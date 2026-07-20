package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/urso/nota/pkg/deleter"
	"github.com/urso/nota/pkg/extension"
	"github.com/urso/nota/pkg/extract"
	"github.com/urso/nota/pkg/formatter"
	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/parser"
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

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile, pyFile}), nil)
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

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile, pyFile}), nil)
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
		Name       string      `yaml:"name"`
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

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

	comments, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(comments))
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

	_, fileContents, fileRanges, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

	_, fileContents, fileRanges, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{pyFile}), nil)
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

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{pyFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{cFile}), nil)
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

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{cFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{rbFile}), nil)
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

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{file1, file2, file3}), nil)
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

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{file1, file2}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

	comments, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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
	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
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

// --- Adjusted line numbers (golden file tests) ---

func TestAdjustedLineNumbers(t *testing.T) {
	entries, err := os.ReadDir("testdata/adjusted_lines")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".golden.json") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			srcPath := filepath.Join("testdata/adjusted_lines", name)
			goldenPath := srcPath + ".golden.json"

			goldenData, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}

			var expected []struct {
				Name string `json:"name"`
				Line int    `json:"line"`
			}
			if err := json.Unmarshal(goldenData, &expected); err != nil {
				t.Fatalf("parsing golden file: %v", err)
			}

			comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{srcPath}), nil)
			if err != nil {
				t.Fatal(err)
			}

			if len(comments) != len(expected) {
				t.Fatalf("expected %d comments, got %d", len(expected), len(comments))
			}

			for i, exp := range expected {
				if comments[i].Name != exp.Name {
					t.Errorf("comment %d: expected name %q, got %q", i, exp.Name, comments[i].Name)
				}
				if comments[i].Line != exp.Line {
					t.Errorf("comment %d (%s): expected line %d, got %d", i, exp.Name, exp.Line, comments[i].Line)
				}
			}
		})
	}
}

// --- Edge cases ---

func TestNoCommentsProducesEmptyOutput(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "clean.go", `package main

// regular comment, not a review tag
func hello() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{binPath, goodFile}), nil)
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

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{badFile, goodFile}), nil)
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

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{unknownFile, goodFile}), nil)
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

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

// --- merged line comment tests ---

func TestMultiLineReviewCommentMerged(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "multi.go", `package main

// review: first line
// continuation here
func hello() {}
`)

	comments, _, ranges, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	if comments[0].Message != "first line continuation here" {
		t.Errorf("expected merged message, got %q", comments[0].Message)
	}

	if comments[0].Line != 3 {
		t.Errorf("expected line 3, got %d", comments[0].Line)
	}

	if comments[0].EndLine != 4 {
		t.Errorf("expected end line 4, got %d", comments[0].EndLine)
	}

	// Byte range should cover both comment lines.
	if len(ranges[goFile]) != 1 {
		t.Fatalf("expected 1 byte range, got %d", len(ranges[goFile]))
	}
}

func TestMultiLineReviewCommentDeletesBothLines(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "delete.go", `package main

// review: should be removed
// along with this line
func hello() {}
`)

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "package main\n\nfunc hello() {}\n"
	if string(after) != expected {
		t.Errorf("after delete:\ngot:  %q\nwant: %q", string(after), expected)
	}
}

func TestMultiLineReviewSeparatedByBlankLine(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "sep.go", `package main

// review: first block
// continues

// review: second block
// also continues
func hello() {}
`)

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	if comments[0].Message != "first block continues" {
		t.Errorf("comment[0] message = %q", comments[0].Message)
	}

	if comments[1].Message != "second block also continues" {
		t.Errorf("comment[1] message = %q", comments[1].Message)
	}
}

func TestMultiLineReviewDifferentIndentNotMerged(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "indent.go", `package main

func hello() {
	// review: indented
	// continues
	x := 1
		// review: deeper indent
		// continues
	_ = x
}
`)

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	if comments[0].Message != "indented continues" {
		t.Errorf("comment[0] message = %q", comments[0].Message)
	}

	if comments[1].Message != "deeper indent continues" {
		t.Errorf("comment[1] message = %q", comments[1].Message)
	}
}

// --- extract.ProcessFiles details ---

func TestProcessFilesReturnsCorrectCommentFields(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "fields.go", `package main

// review(auth): check this
func hello() {}

// explain: why pattern
func other() {}
`)

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

	_, _, fileRanges, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
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

	t.Run("unknown format falls back to markdown", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeOutput(&buf, groups, "unknown-format"); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		// Verify it produces markdown (has heading), not YAML (would start with "- name:").
		if !strings.Contains(out, "## test") {
			t.Error("expected markdown heading for unknown format")
		}
		if strings.HasPrefix(out, "- name:") {
			t.Error("unknown format should not produce YAML")
		}
	})
}

// --- Extract --dir tracking file tests ---

func TestExtractDirNamedGroup(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".nota")

	goFile := writeTestFile(t, srcDir, "auth.go", `package auth

// review(auth): check token expiry
func validate() {
	token := getToken()
	return token.Valid()
}

// discuss(auth): is this safe?
func other() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	// Named group "auth" should produce auth.xml.
	authFile := filepath.Join(outDir, "auth.xml")
	content, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("expected auth.xml to exist: %v", err)
	}

	s := string(content)

	// Check XML structure.
	if !strings.Contains(s, `status="open"`) {
		t.Error("missing status=open attribute")
	}
	if !strings.Contains(s, `group="auth"`) {
		t.Error("missing group=auth attribute")
	}
	if !strings.Contains(s, `goal="review"`) {
		t.Error("missing goal=review attribute")
	}
	if !strings.Contains(s, "check token expiry") {
		t.Error("missing review comment text")
	}
}

func TestExtractDirUnnamedGroups(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".nota")

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review: standalone one
func first() {}

// explain: why this
func second() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	// Unnamed groups should produce review-001.xml, review-002.xml.
	f1 := filepath.Join(outDir, "review-001.xml")
	f2 := filepath.Join(outDir, "review-002.xml")

	c1, err := os.ReadFile(f1)
	if err != nil {
		t.Fatalf("expected review-001.xml: %v", err)
	}
	c2, err := os.ReadFile(f2)
	if err != nil {
		t.Fatalf("expected review-002.xml: %v", err)
	}

	// Each should be valid XML with status="open" and no group attribute.
	for _, c := range [][]byte{c1, c2} {
		s := string(c)
		if !strings.Contains(s, `status="open"`) {
			t.Error("missing status=open")
		}
		if strings.Contains(s, `group="`) {
			t.Error("unnamed group should not have group attribute")
		}
	}
}

func TestExtractDirOverwritesExistingXML(t *testing.T) {
	outDir := t.TempDir()

	// Pre-create an existing XML file (will be overwritten).
	existing := `<?xml version="1.0"?><nota-thread status="open" group="auth"><nota-comment id="old" author="user"><nota-body time="t"><![CDATA[old comment]]></nota-body></nota-comment></nota-thread>`
	if err := os.WriteFile(filepath.Join(outDir, "auth.xml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	srcDir := t.TempDir()
	goFile := writeTestFile(t, srcDir, "new.go", `package auth

// review(auth): new comment
func newFunc() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "auth.xml"))
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)

	// New XML overwrites old (XML threads are complete files, not appended).
	if strings.Contains(s, "old comment") {
		t.Error("old content should be overwritten")
	}
	if !strings.Contains(s, "new comment") {
		t.Error("new content missing")
	}
}

func TestExtractDirCreatesNewXMLFile(t *testing.T) {
	outDir := t.TempDir()

	srcDir := t.TempDir()
	goFile := writeTestFile(t, srcDir, "new.go", `package auth

// review(auth): new comment
func newFunc() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "auth.xml"))
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)

	// New file should have status="open".
	if !strings.Contains(s, `status="open"`) {
		t.Error("missing status=open")
	}
	if !strings.Contains(s, "new comment") {
		t.Error("missing comment content")
	}
}

func TestExtractDirReviewNumbering(t *testing.T) {
	outDir := t.TempDir()

	// Pre-create review-001.xml and review-003.xml.
	for _, name := range []string{"review-001.xml", "review-003.xml"} {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(`<?xml version="1.0"?><nota-thread status="open"/>`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	srcDir := t.TempDir()
	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review: new standalone
func hello() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	// Should use review-004.xml (max existing is 003).
	f := filepath.Join(outDir, "review-004.xml")
	if _, err := os.Stat(f); os.IsNotExist(err) {
		t.Error("expected review-004.xml to be created")
	}
}

func TestExtractDirMixedNamedAndUnnamed(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".nota")

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review(api): check error handling
func handleRequest() {}

// review: standalone fix
func other() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	// Named group produces api.xml.
	if _, err := os.Stat(filepath.Join(outDir, "api.xml")); os.IsNotExist(err) {
		t.Error("expected api.xml")
	}

	// Unnamed group produces review-001.xml.
	if _, err := os.Stat(filepath.Join(outDir, "review-001.xml")); os.IsNotExist(err) {
		t.Error("expected review-001.xml")
	}
}

func TestExtractDirWithDelete(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".nota")

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// review(api): check error handling
func handleRequest() {
	resp := doRequest()
	return resp
}
`)

	comments, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	// Write tracking files.
	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
		t.Fatal(err)
	}

	// Delete comments from source.
	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	// Tracking file should exist with content.
	trackingContent, err := os.ReadFile(filepath.Join(outDir, "api.xml"))
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
			err := extract.WriteThreadFiles(outDir, groups, nil)
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
			err := extract.WriteThreadFiles(outDir, groups, nil)
			if err == nil {
				t.Errorf("expected error for group name %q, got nil", name)
			}
		})
	}
}

func TestExtractDirNoComments(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), ".nota")

	goFile := writeTestFile(t, srcDir, "clean.go", `package main

func hello() {}
`)

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), nil)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 3)

	if err := extract.WriteThreadFiles(outDir, groups, fileContents); err != nil {
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

// --- Extension system integration tests ---

func TestExtensionTagExtracted(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", `package main

// critique(perf): check allocation
func hot() {}
`)

	// Use knownTags that include "critique".
	knownTags := map[string]struct{}{
		"review": {}, "discuss": {}, "explain": {},
		"critique": {}, "see": {}, "also": {},
	}

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), knownTags)
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Tag != parser.Tag("critique") {
		t.Errorf("expected tag critique, got %s", comments[0].Tag)
	}
	if comments[0].Name != "perf" {
		t.Errorf("expected name perf, got %s", comments[0].Name)
	}
	if comments[0].Message != "check allocation" {
		t.Errorf("expected message 'check allocation', got %q", comments[0].Message)
	}
}

func TestUnknownTagSkippedWithKnownTags(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", `package main

// todo: fix this later

// note: remember this

// fixme: broken thing

// review: real comment
func hello() {}
`)

	knownTags := map[string]struct{}{
		"review": {}, "discuss": {}, "explain": {},
		"see": {}, "also": {},
	}

	comments, _, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), knownTags)
	if err != nil {
		t.Fatal(err)
	}

	// Only "review" should be extracted; todo/note/fixme should be skipped.
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Tag != parser.TagReview {
		t.Errorf("expected tag review, got %s", comments[0].Tag)
	}
}

func TestExtractDoesNotDeleteUnknownTags(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", "package main\n\n// note: something\n\n// fixme: broken\n\n// review: delete me\nfunc hello() {}\n")

	knownTags := map[string]struct{}{
		"review": {}, "see": {}, "also": {},
	}

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), knownTags)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	// review should be deleted.
	if strings.Contains(got, "// review:") {
		t.Error("review comment was not deleted")
	}
	// note and fixme should NOT be deleted.
	if !strings.Contains(got, "// note: something") {
		t.Error("note comment was incorrectly deleted")
	}
	if !strings.Contains(got, "// fixme: broken") {
		t.Error("fixme comment was incorrectly deleted")
	}
}

func TestExtensionTagGroupedAsEntry(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", `package main

// critique(perf): check allocation
func hot() {}

// propose(perf): use a pool
func other() {}
`)

	knownTags := map[string]struct{}{
		"review": {}, "discuss": {}, "explain": {},
		"critique": {}, "propose": {}, "see": {}, "also": {},
	}

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), knownTags)
	if err != nil {
		t.Fatal(err)
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "perf" {
		t.Errorf("expected group name perf, got %q", groups[0].Name)
	}
	if len(groups[0].Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(groups[0].Entries))
	}
}

func TestBehaviorSubcommandTable(t *testing.T) {
	// Capture stdout from runBehavior with no args.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	err := runBehavior(nil)

	_ = w.Close()

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	// Should contain embedded tags.
	if !strings.Contains(out, "review\t") {
		t.Error("missing review in triage table")
	}
	if !strings.Contains(out, "critique\t") {
		t.Error("missing critique in triage table")
	}
	// Should contain see/also as cross-reference.
	if !strings.Contains(out, "see\t(cross-reference)") {
		t.Error("missing see cross-reference in triage table")
	}
	if !strings.Contains(out, "also\t(cross-reference)") {
		t.Error("missing also cross-reference in triage table")
	}
}

func TestBehaviorSubcommandSingleTag(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	err := runBehavior([]string{"review"})

	_ = w.Close()

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := strings.TrimSpace(buf.String())

	if out == "" {
		t.Error("expected non-empty behavior for review")
	}
}

func TestBehaviorSubcommandSee(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	err := runBehavior([]string{"see"})

	_ = w.Close()

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := strings.TrimSpace(buf.String())

	if out != "(cross-reference)" {
		t.Errorf("expected (cross-reference), got %q", out)
	}
}

func TestBehaviorSubcommandUnknown(t *testing.T) {
	err := runBehavior([]string{"nonexistent_tag_xyz"})
	if err == nil {
		t.Error("expected error for unknown tag")
	}
}

func TestBehaviorSubcommandTooManyArgs(t *testing.T) {
	err := runBehavior([]string{"a", "b"})
	if err == nil {
		t.Error("expected error for too many args")
	}
}

// --- E2E tests using extension.LoadAll() ---

func TestE2EExtensionTagWithLoadAll(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", `package main

// critique(perf): check allocation
func hot() {}

// propose(perf): use a pool
func other() {}

// todo: not extracted

// review: standalone
func last() {}
`)

	// Use LoadAll (no local dir) to get the embedded tag set.
	_, tagSet := extension.LoadAll("")

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), tagSet)
	if err != nil {
		t.Fatal(err)
	}

	// critique, propose, and review should be extracted; todo should not.
	if len(comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(comments))
	}

	tagsSeen := map[parser.Tag]bool{}
	for _, c := range comments {
		tagsSeen[c.Tag] = true
	}

	if !tagsSeen[parser.Tag("critique")] {
		t.Error("expected critique tag extracted")
	}
	if !tagsSeen[parser.Tag("propose")] {
		t.Error("expected propose tag extracted")
	}
	if !tagsSeen[parser.TagReview] {
		t.Error("expected review tag extracted")
	}

	// Grouper should handle extension tags correctly.
	groups := grouper.GroupComments(comments, fileContents, 2)

	// Should have 1 named group "perf" and 1 unnamed.
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "perf" {
		t.Errorf("expected first group 'perf', got %q", groups[0].Name)
	}
	if len(groups[0].Entries) != 2 {
		t.Errorf("expected 2 entries in perf group, got %d", len(groups[0].Entries))
	}
}

func TestE2ECustomExtensionWithLocalDir(t *testing.T) {
	srcDir := t.TempDir()
	localDir := t.TempDir()

	// Create a custom extension.
	extDir := filepath.Join(localDir, "extensions")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "custom.yaml"),
		[]byte("tag: custom\nbehavior: do custom things\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	goFile := writeTestFile(t, srcDir, "code.go", `package main

// custom(feat): do the custom thing
func hello() {}

// review: regular comment
func other() {}
`)

	_, tagSet := extension.LoadAll(localDir)

	// custom should be in the tag set.
	if _, ok := tagSet["custom"]; !ok {
		t.Fatal("expected custom in tag set")
	}

	comments, fileContents, _, _, _, err := extract.ProcessFiles(slices.Values([]string{goFile}), tagSet)
	if err != nil {
		t.Fatal(err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	groups := grouper.GroupComments(comments, fileContents, 2)

	// Should have named group "feat" from custom tag and unnamed from review.
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Verify custom tag flows through to group entry.
	found := false
	for _, g := range groups {
		for _, e := range g.Entries {
			if e.Tag == parser.Tag("custom") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected custom tag in group entries")
	}
}

func TestE2EExtractDoesNotDeleteNonExtensionTags(t *testing.T) {
	dir := t.TempDir()

	goFile := writeTestFile(t, dir, "code.go", "package main\n\n// note: keep this\n\n// review: delete this\nfunc hello() {}\n")

	_, tagSet := extension.LoadAll("")

	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(slices.Values([]string{goFile}), tagSet)
	if err != nil {
		t.Fatal(err)
	}

	if err := extract.DeleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	if !strings.Contains(got, "// note: keep this") {
		t.Error("note comment was incorrectly deleted")
	}
	if strings.Contains(got, "// review: delete this") {
		t.Error("review comment was not deleted")
	}
}
