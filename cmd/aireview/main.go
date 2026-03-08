package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urso/aireview/pkg/deleter"
	"github.com/urso/aireview/pkg/formatter"
	"github.com/urso/aireview/pkg/git"
	"github.com/urso/aireview/pkg/grouper"
)

func main() {
	if len(os.Args) < 2 {
		runList(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "list":
		runList(os.Args[2:])
	case "delete":
		runDelete(os.Args[2:])
	case "extract":
		runExtract(os.Args[2:])
	default:
		// F8: If first arg starts with "-" or looks like a file path, default to list.
		// Otherwise report an error for unknown subcommands.
		if strings.HasPrefix(os.Args[1], "-") || looksLikeFilePath(os.Args[1]) {
			runList(os.Args[1:])
			return
		}
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\nUsage: aireview [list|delete|extract] [flags] [files...]\n", os.Args[1])
		os.Exit(1)
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

func resolveFiles(fs *flag.FlagSet, modified, unstaged, staged, all *bool) ([]string, error) {
	if fs.NArg() > 0 {
		return fs.Args(), nil
	}

	if !git.IsAvailable() {
		return nil, fmt.Errorf("not in a git repository. Use explicit file paths: aireview [command] file1.go file2.go")
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

	root, err := git.RepoRoot("")
	if err != nil {
		return nil, fmt.Errorf("not in a git repository. Use explicit file paths: aireview [command] file1.go file2.go")
	}

	gitFiles, err := git.ListFiles(scope, "")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, f := range gitFiles {
		files = append(files, filepath.Join(root, f))
	}

	return files, nil
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	format, ctx, modified, unstaged, staged, all := addSharedFlags(fs)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	files, err := resolveFiles(fs, modified, unstaged, staged, all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	comments, fileContents, _, _, err := processFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	groups := grouper.GroupComments(comments, fileContents, *ctx)

	if err := writeOutput(os.Stdout, groups, *format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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

func runDelete(args []string) {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	_, _, modified, unstaged, staged, all := addSharedFlags(fs)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	files, err := resolveFiles(fs, modified, unstaged, staged, all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	_, fileContents, fileRanges, filePerms, err := processFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	format, ctx, modified, unstaged, staged, all := addSharedFlags(fs)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	files, err := resolveFiles(fs, modified, unstaged, staged, all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	comments, fileContents, fileRanges, filePerms, err := processFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	groups := grouper.GroupComments(comments, fileContents, *ctx)

	// 1. Format output to stdout.
	if err := writeOutput(os.Stdout, groups, *format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// F3: Write backup BEFORE deleting, so interruption after delete still has backup.
	ext := "md"
	if *format == "yaml" {
		ext = "yaml"
	}
	backup, err := os.CreateTemp("", fmt.Sprintf("aireview-backup-*.%s", ext))
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create backup file: %v\n", err)
	} else {
		if writeErr := writeOutput(backup, groups, *format); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write backup: %v\n", writeErr)
		}
		_ = backup.Close()
		fmt.Fprintf(os.Stderr, "backup: %s\n", backup.Name())
	}

	// 2. Delete comments from files.
	// F7: Exit with error code on failure instead of silently continuing.
	if err := deleteFromFiles(fileContents, fileRanges, filePerms); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func writeOutput(w io.Writer, groups []grouper.Group, format string) error {
	switch format {
	case "yaml":
		return formatter.FormatYAML(w, groups)
	default:
		return formatter.FormatMarkdown(w, groups)
	}
}
