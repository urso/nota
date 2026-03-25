package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/urso/nota/pkg/deleter"
	"github.com/urso/nota/pkg/extension"
	"github.com/urso/nota/pkg/formatter"
	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/tracking"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return runList(args[1:])
	}

	switch args[1] {
	case "list":
		return runList(args[2:])
	case "delete":
		return runDelete(args[2:])
	case "extract":
		return runExtract(args[2:])
	case "behavior":
		return runBehavior(args[2:])
	case "find":
		return runFind(args[2:])
	case "validate":
		return runValidate(args[2:])
	case "read":
		return runRead(args[2:])
	default:
		// F8: If first arg starts with "-" or looks like a file path, default to list.
		// Otherwise report an error for unknown subcommands.
		if strings.HasPrefix(args[1], "-") || looksLikeFilePath(args[1]) {
			return runList(args[1:])
		}
		return fmt.Errorf("unknown subcommand: %s\nUsage: nota [list|delete|extract|behavior|find|validate|read] [flags] [files...]", args[1])
	}
}

// looksLikeFilePath returns true if the string looks like a file path rather than a subcommand.
func looksLikeFilePath(s string) bool {
	return strings.Contains(s, "/") || strings.Contains(s, ".") || strings.Contains(s, string(filepath.Separator))
}

func addSharedFlags(fs *flag.FlagSet) (format *string, ctx *int, modified, unstaged, staged, all *bool) {
	format = fs.String("format", "markdown", "Output format: markdown or yaml")
	ctx = fs.Int("context", 3, "Context lines around comments")
	modified = fs.Bool("modified", false, "Staged + unstaged + untracked (default scope)")
	unstaged = fs.Bool("unstaged", false, "Unstaged + untracked only")
	staged = fs.Bool("staged", false, "Staged files only")
	all = fs.Bool("all", false, "All tracked files")
	return
}

func resolveFiles(fs *flag.FlagSet, modified, unstaged, staged, all *bool) (iter.Seq[string], error) {
	if fs.NArg() > 0 {
		return slices.Values(fs.Args()), nil
	}

	count := 0
	if *modified {
		count++
	}
	if *unstaged {
		count++
	}
	if *staged {
		count++
	}
	if *all {
		count++
	}
	if count > 1 {
		return nil, fmt.Errorf("only one scope flag allowed (--modified, --unstaged, --staged, --all)")
	}

	// Try git first.
	root, gitErr := git.RepoRoot("")
	if gitErr == nil {
		var scope git.Scope
		switch {
		case *unstaged:
			scope = git.ScopeUnstaged
		case *staged:
			scope = git.ScopeStaged
		case *all:
			scope = git.ScopeAll
		default:
			scope = git.ScopeModified
		}

		gitFiles, err := git.ListFiles(scope, "")
		if err != nil {
			return nil, err
		}

		files := make([]string, len(gitFiles))
		for i, f := range gitFiles {
			files[i] = filepath.Join(root, f)
		}
		return slices.Values(files), nil
	}

	// Not in a git repo. Only --all (or default) makes sense without git.
	if *modified || *unstaged || *staged {
		return nil, fmt.Errorf("--modified, --unstaged, and --staged require a git repository. Use explicit file paths or --all")
	}

	// Walk up from $PWD to find a project root (directory containing .nota/).
	root, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository and no .nota/ found. Use explicit file paths: nota [command] file1.go file2.go")
	}

	return walkFiles(root), nil
}

// findProjectRoot walks up from the current directory looking for a .nota/ directory.
// Returns the directory containing .nota/, or an error if none found.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".nota")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .nota/ directory found")
		}
		dir = parent
	}
}

// walkFiles yields regular files under root, skipping hidden directories.
func walkFiles(root string) iter.Seq[string] {
	return func(yield func(string) bool) {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if info.IsDir() {
				name := info.Name()
				if strings.HasPrefix(name, ".") && name != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if !yield(path) {
				return filepath.SkipAll
			}
			return nil
		})
	}
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	format, ctx, modified, unstaged, staged, all := addSharedFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	files, err := resolveFiles(fs, modified, unstaged, staged, all)
	if err != nil {
		return err
	}

	_, tagSet := extension.LoadAll(localExtDir())
	comments, fileContents, _, _, err := processFiles(files, tagSet)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, *ctx)
	return writeOutput(os.Stdout, groups, *format)
}

// F2: Use content already read by processFiles instead of re-reading.
// This eliminates the TOCTOU race.
func deleteFromFiles(fileContents map[string][]byte, fileRanges map[string][]deleter.ByteRange, filePerms map[string]os.FileMode) error {
	for filePath, ranges := range fileRanges {
		if len(ranges) == 0 {
			continue
		}

		content, ok := fileContents[filePath]
		if !ok {
			return fmt.Errorf("no content for %s", filePath)
		}

		perm, ok := filePerms[filePath]
		if !ok {
			perm = 0o644
		}

		modified, err := deleter.DeleteComments(content, ranges)
		if err != nil {
			return fmt.Errorf("deleting comments in %s: %w", filePath, err)
		}

		if err := deleter.WriteAtomic(filePath, modified, perm); err != nil {
			return fmt.Errorf("writing %s: %w", filePath, err)
		}
	}
	return nil
}

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	_, _, modified, unstaged, staged, all := addSharedFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	files, err := resolveFiles(fs, modified, unstaged, staged, all)
	if err != nil {
		return err
	}

	_, tagSet := extension.LoadAll(localExtDir())
	_, fileContents, fileRanges, filePerms, err := processFiles(files, tagSet)
	if err != nil {
		return err
	}

	return deleteFromFiles(fileContents, fileRanges, filePerms)
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	format, ctx, modified, unstaged, staged, all := addSharedFlags(fs)
	dir := fs.String("dir", "", "Write tracking files to directory (e.g. .nota/)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	files, err := resolveFiles(fs, modified, unstaged, staged, all)
	if err != nil {
		return err
	}

	_, tagSet := extension.LoadAll(localExtDir())
	comments, fileContents, fileRanges, filePerms, err := processFiles(files, tagSet)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, *ctx)

	if *dir != "" {
		// --dir mode: write tracking files, no stdout, no backup.
		if *format != "markdown" {
			fmt.Fprintf(os.Stderr, "warning: --format is ignored when --dir is used\n")
		}
		if err := writeTrackingFiles(*dir, groups); err != nil {
			return err
		}
	} else {
		// Default mode: stdout + backup.
		if err := writeOutput(os.Stdout, groups, *format); err != nil {
			return err
		}

		// F3: Write backup BEFORE deleting, so interruption after delete still has backup.
		ext := "md"
		if *format == "yaml" {
			ext = "yaml"
		}
		backup, err := os.CreateTemp("", fmt.Sprintf("nota-backup-*.%s", ext))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create backup file: %v\n", err)
		} else {
			if writeErr := writeOutput(backup, groups, *format); writeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write backup: %v\n", writeErr)
			}
			_ = backup.Close()
			fmt.Fprintf(os.Stderr, "backup: %s\n", backup.Name())
		}
	}

	// Delete comments from files.
	return deleteFromFiles(fileContents, fileRanges, filePerms)
}

var (
	reReviewFile     = regexp.MustCompile(`^review-(\d+)\.md$`)
	reFrontmatter    = regexp.MustCompile(`(?s)\A---\n(.*?\n)---\n`)
	reStatusResolved = regexp.MustCompile(`(?m)^status:\s*resolved\s*$`)
)

// writeTrackingFiles writes groups as tracking files in dir.
// Named groups go to {dir}/{name}.md (appending if file exists).
// Unnamed groups go to {dir}/review-{NNN}.md (new file each).
// File paths are printed to stderr.
func writeTrackingFiles(dir string, groups []grouper.Group) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory %s: %w", dir, err)
	}

	nextNum := nextReviewNumber(dir)

	for _, g := range groups {
		var filePath string
		if g.Name != "" {
			safeName, err := sanitizeGroupName(g.Name)
			if err != nil {
				return fmt.Errorf("invalid group name %q: %w", g.Name, err)
			}
			filePath = filepath.Join(dir, safeName+".md")
		} else {
			filePath = filepath.Join(dir, fmt.Sprintf("review-%03d.md", nextNum))
			nextNum++
		}

		// Verify the resolved path stays within the target directory.
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", filePath, err)
		}
		if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
			return fmt.Errorf("group name %q resolves outside target directory", g.Name)
		}

		if err := writeOrAppendTracking(filePath, g); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "%s\n", filePath)
	}

	return nil
}

// sanitizeGroupName validates and sanitizes a group name for use as a filename.
func sanitizeGroupName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty group name")
	}

	// Reject path separators, traversal, and OS-invalid characters.
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

// nextReviewNumber finds the next available review-NNN number in dir.
func nextReviewNumber(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}

	max := 0
	for _, e := range entries {
		m := reReviewFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1
}

// writeOrAppendTracking writes a new tracking file or appends to an existing one.
// When appending to a resolved file, status is set back to open.
func writeOrAppendTracking(filePath string, g grouper.Group) error {
	existing, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}

	isNewFile := os.IsNotExist(err)

	if isNewFile {
		// New file: write frontmatter + sections.
		var buf bytes.Buffer
		if err := formatter.FormatTrackingFile(&buf, g); err != nil {
			return fmt.Errorf("formatting %s: %w", filePath, err)
		}
		return deleter.WriteAtomic(filePath, buf.Bytes(), 0o644)
	}

	// Existing file: append sections, reopen if resolved.
	content := reopenIfResolved(existing)

	var sections bytes.Buffer
	if err := formatter.FormatTracking(&sections, g); err != nil {
		return fmt.Errorf("formatting sections for %s: %w", filePath, err)
	}

	// Ensure there's a newline before appending.
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}

	content = append(content, sections.Bytes()...)
	return deleter.WriteAtomic(filePath, content, 0o644)
}

// reopenIfResolved checks if the frontmatter has status: resolved and changes it to open.
// Only modifies the YAML frontmatter block (between first two --- lines).
func reopenIfResolved(content []byte) []byte {
	loc := reFrontmatter.FindIndex(content)
	if loc == nil {
		return content
	}

	frontmatter := content[loc[0]:loc[1]]
	if !reStatusResolved.Match(frontmatter) {
		return content
	}

	updated := reStatusResolved.ReplaceAll(frontmatter, []byte("status: open"))
	result := make([]byte, 0, len(content))
	result = append(result, updated...)
	result = append(result, content[loc[1]:]...)
	return result
}

// projectRoot returns the project root path, trying git first then walking up for .nota/.
func projectRoot() string {
	if root, err := git.RepoRoot(""); err == nil {
		return root
	}
	if root, err := findProjectRoot(); err == nil {
		return root
	}
	return ""
}

// localExtDir returns the .nota directory path based on the project root.
func localExtDir() string {
	if root := projectRoot(); root != "" {
		return filepath.Join(root, ".nota")
	}
	return ".nota"
}

func runBehavior(args []string) error {
	fs := flag.NewFlagSet("behavior", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: nota behavior [tagname]\n\n")
		fmt.Fprintf(os.Stderr, "With no arguments, outputs a table of all known tags and behaviors.\n")
		fmt.Fprintf(os.Stderr, "With a tag name, outputs the behavior text for that tag.\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("too many arguments")
	}

	localDir := localExtDir()

	if fs.NArg() == 1 {
		ext := extension.LoadExtension(fs.Arg(0), localDir)
		if ext == nil {
			return fmt.Errorf("unknown tag: %s", args[0])
		}
		fmt.Println(ext.Behavior)
		return nil
	}

	// No args: output triage table of all known tags.
	exts, tagSet := extension.LoadAll(localDir)

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		if ext, ok := exts[tag]; ok {
			// Collapse newlines to spaces for one-line-per-tag format.
			behavior := strings.ReplaceAll(ext.Behavior, "\n", " ")
			fmt.Printf("%s\t%s\n", tag, behavior)
		} else {
			// see/also are in TagSet but not in the extension map.
			fmt.Printf("%s\t(cross-reference)\n", tag)
		}
	}

	return nil
}

func runFind(args []string) error {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	status := fs.String("status", "", "Filter by status (open or resolved)")
	tag := fs.String("tag", "", "Filter by tag")
	refsOf := fs.String("refs-of", "", "List files that <name> references")
	depsOf := fs.String("deps-of", "", "List files that <name> depends on")
	referencedBy := fs.String("referenced-by", "", "List files that reference <name>")
	blockedBy := fs.String("blocked-by", "", "List files that depend on <name>")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: nota find [flags] [name]\n\n")
		fmt.Fprintf(os.Stderr, "Query .nota/ tracking files by name, status, tag, or relationships.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	root := projectRoot()
	dir := filepath.Join(root, ".nota")

	opts := tracking.FindOptions{
		Dir:          dir,
		Status:       *status,
		Tag:          *tag,
		RefsOf:       *refsOf,
		DepsOf:       *depsOf,
		ReferencedBy: *referencedBy,
		BlockedBy:    *blockedBy,
	}
	if fs.NArg() > 0 {
		opts.Name = fs.Arg(0)
	}

	results, err := tracking.Find(opts)
	if err != nil {
		return err
	}
	for _, r := range results {
		fmt.Println(r)
	}
	return nil
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: nota validate <file>\n\n")
		fmt.Fprintf(os.Stderr, "Validate a .nota/ tracking file.\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("exactly one file required")
	}

	filePath := fs.Arg(0)
	errs, err := tracking.ValidateFile(filePath)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stdout, "Tracking file validation failed for %s:\n", filePath)
		for _, e := range errs {
			fmt.Println(e)
		}
		return fmt.Errorf("validation failed")
	}
	return nil
}

func runRead(args []string) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: nota read <file>\n\n")
		fmt.Fprintf(os.Stderr, "Read a tracking file, stripping resolved/wontfix sections.\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("exactly one file required")
	}

	content, err := tracking.ReadOpen(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Print(content)
	return nil
}

func writeOutput(w io.Writer, groups []grouper.Group, format string) error {
	switch format {
	case "yaml":
		return formatter.FormatYAML(w, groups)
	default:
		return formatter.FormatMarkdown(w, groups)
	}
}
