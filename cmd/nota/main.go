package main

import (
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/urso/nota/pkg/extension"
	"github.com/urso/nota/pkg/extract"
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

	flags := []bool{*modified, *unstaged, *staged, *all}
	count := 0
	for _, f := range flags {
		if f {
			count++
		}
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
	comments, fileContents, _, _, _, err := extract.ProcessFiles(files, tagSet)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, *ctx)
	return writeOutput(os.Stdout, groups, *format)
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
	_, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(files, tagSet)
	if err != nil {
		return err
	}

	return extract.DeleteFromFiles(fileContents, fileRanges, filePerms)
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

	if *dir != "" {
		// --dir mode: use extract.Run for full pipeline.
		if *format != "markdown" {
			fmt.Fprintf(os.Stderr, "warning: --format is ignored when --dir is used\n")
		}
		return extract.Run(extract.Config{
			Files:        files,
			KnownTags:    tagSet,
			Dir:          *dir,
			ContextLines: *ctx,
		})
	}

	// Default mode: stdout + backup.
	comments, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(files, tagSet)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, *ctx)

	if err := writeOutput(os.Stdout, groups, *format); err != nil {
		return err
	}

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

	return extract.DeleteFromFiles(fileContents, fileRanges, filePerms)
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
