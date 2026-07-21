package main

import (
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

// CLI is the root command structure for nota.
type CLI struct {
	List     ListCmd     `cmd:"" default:"withargs" help:"List comments in source files"`
	Delete   DeleteCmd   `cmd:"" help:"Delete comments from source files"`
	Extract  ExtractCmd  `cmd:"" help:"Extract comments to tracking files"`
	Behavior BehaviorCmd `cmd:"" help:"Show tag behaviors"`
	Find     FindCmd     `cmd:"" help:"Query tracking files"`
	Validate ValidateCmd `cmd:"" help:"Validate a tracking file"`
	Read     ReadCmd     `cmd:"" help:"Read a tracking file"`
}

// SharedFlags contains flags shared across list, delete, and extract commands.
type SharedFlags struct {
	Format   string `help:"Output format: markdown or yaml" default:"markdown" enum:"markdown,yaml"`
	Context  int    `help:"Context lines around comments" default:"3"`
	Modified bool   `help:"Staged + unstaged + untracked (default scope)"`
	Unstaged bool   `help:"Unstaged + untracked only"`
	Staged   bool   `help:"Staged files only"`
	All      bool   `help:"All tracked files"`
}

// ListCmd lists comments in source files.
type ListCmd struct {
	SharedFlags
	Files []string `arg:"" optional:"" help:"Files to scan"`
}

func (c *ListCmd) Run() error {
	files, err := resolveFilesFromCmd(c.Files, c.Modified, c.Unstaged, c.Staged, c.All)
	if err != nil {
		return err
	}

	_, tagSet := extension.LoadAll(localExtDir())
	comments, fileContents, _, _, _, err := extract.ProcessFiles(files, tagSet)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, c.Context)
	return writeOutput(os.Stdout, groups, c.Format)
}

// DeleteCmd deletes comments from source files.
type DeleteCmd struct {
	SharedFlags
	Files []string `arg:"" optional:"" help:"Files to process"`
}

func (c *DeleteCmd) Run() error {
	files, err := resolveFilesFromCmd(c.Files, c.Modified, c.Unstaged, c.Staged, c.All)
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

// ExtractCmd extracts comments to tracking files.
type ExtractCmd struct {
	SharedFlags
	Dir   string   `help:"Write tracking files to directory (e.g. .nota/)"`
	Files []string `arg:"" optional:"" help:"Files to process"`
}

func (c *ExtractCmd) Run() error {
	files, err := resolveFilesFromCmd(c.Files, c.Modified, c.Unstaged, c.Staged, c.All)
	if err != nil {
		return err
	}

	_, tagSet := extension.LoadAll(localExtDir())

	if c.Dir != "" {
		if c.Format != "markdown" {
			fmt.Fprintf(os.Stderr, "warning: --format is ignored when --dir is used\n")
		}
		return extract.Run(extract.Config{
			Files:        files,
			KnownTags:    tagSet,
			Dir:          c.Dir,
			ContextLines: c.Context,
		})
	}

	comments, fileContents, fileRanges, _, filePerms, err := extract.ProcessFiles(files, tagSet)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, c.Context)

	if err := writeOutput(os.Stdout, groups, c.Format); err != nil {
		return err
	}

	ext := "md"
	if c.Format == "yaml" {
		ext = "yaml"
	}
	backup, err := os.CreateTemp("", fmt.Sprintf("nota-backup-*.%s", ext))
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create backup file: %v\n", err)
	} else {
		if writeErr := writeOutput(backup, groups, c.Format); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write backup: %v\n", writeErr)
		}
		_ = backup.Close()
		fmt.Fprintf(os.Stderr, "backup: %s\n", backup.Name())
	}

	return extract.DeleteFromFiles(fileContents, fileRanges, filePerms)
}

// BehaviorCmd shows tag behaviors.
type BehaviorCmd struct {
	Tag string `arg:"" optional:"" help:"Tag name to show behavior for"`
}

func (c *BehaviorCmd) Run() error {
	localDir := localExtDir()

	if c.Tag != "" {
		ext := extension.LoadExtension(c.Tag, localDir)
		if ext == nil {
			return fmt.Errorf("unknown tag: %s", c.Tag)
		}
		fmt.Println(ext.Behavior)
		return nil
	}

	exts, tagSet := extension.LoadAll(localDir)

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		if ext, ok := exts[tag]; ok {
			behavior := strings.ReplaceAll(ext.Behavior, "\n", " ")
			fmt.Printf("%s\t%s\n", tag, behavior)
		} else {
			fmt.Printf("%s\t(cross-reference)\n", tag)
		}
	}

	return nil
}

// FindCmd queries tracking files.
type FindCmd struct {
	Name         string `arg:"" optional:"" help:"Name to search for"`
	Status       string `help:"Filter by status (open or resolved)"`
	Tag          string `help:"Filter by tag"`
	RefsOf       string `help:"List files that <name> references" name:"refs-of"`
	DepsOf       string `help:"List files that <name> depends on" name:"deps-of"`
	ReferencedBy string `help:"List files that reference <name>" name:"referenced-by"`
	BlockedBy    string `help:"List files that depend on <name>" name:"blocked-by"`
}

func (c *FindCmd) Run() error {
	root := projectRoot()
	dir := filepath.Join(root, ".nota")

	opts := tracking.FindOptions{
		Dir:          dir,
		Name:         c.Name,
		Status:       c.Status,
		Tag:          c.Tag,
		RefsOf:       c.RefsOf,
		DepsOf:       c.DepsOf,
		ReferencedBy: c.ReferencedBy,
		BlockedBy:    c.BlockedBy,
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

// ValidateCmd validates a tracking file.
type ValidateCmd struct {
	File string `arg:"" required:"" help:"Tracking file to validate"`
}

func (c *ValidateCmd) Run() error {
	errs, err := tracking.ValidateFile(c.File)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stdout, "Tracking file validation failed for %s:\n", c.File)
		for _, e := range errs {
			fmt.Println(e)
		}
		return fmt.Errorf("validation failed")
	}
	return nil
}

// ReadCmd reads a tracking file.
type ReadCmd struct {
	File string `arg:"" required:"" help:"Tracking file to read"`
}

func (c *ReadCmd) Run() error {
	content, err := tracking.ReadOpen(c.File)
	if err != nil {
		return err
	}
	fmt.Print(content)
	return nil
}

// resolveFilesFromCmd resolves files from positional args or git scope flags.
func resolveFilesFromCmd(files []string, modified, unstaged, staged, all bool) (iter.Seq[string], error) {
	if len(files) > 0 {
		return slices.Values(files), nil
	}

	flags := []bool{modified, unstaged, staged, all}
	count := 0
	for _, f := range flags {
		if f {
			count++
		}
	}
	if count > 1 {
		return nil, fmt.Errorf("only one scope flag allowed (--modified, --unstaged, --staged, --all)")
	}

	root, gitErr := git.RepoRoot("")
	if gitErr == nil {
		var scope git.Scope
		switch {
		case unstaged:
			scope = git.ScopeUnstaged
		case staged:
			scope = git.ScopeStaged
		case all:
			scope = git.ScopeAll
		default:
			scope = git.ScopeModified
		}

		gitFiles, err := git.ListFiles(scope, "")
		if err != nil {
			return nil, err
		}

		absFiles := make([]string, len(gitFiles))
		for i, f := range gitFiles {
			absFiles[i] = filepath.Join(root, f)
		}
		return slices.Values(absFiles), nil
	}

	if modified || unstaged || staged {
		return nil, fmt.Errorf("--modified, --unstaged, and --staged require a git repository. Use explicit file paths or --all")
	}

	root, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository and no .nota/ found. Use explicit file paths: nota [command] file1.go file2.go")
	}

	return walkFiles(root), nil
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

// findProjectRoot walks up from the current directory looking for a .nota/ directory.
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
				return nil
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

// writeOutput writes groups in the specified format.
func writeOutput(w io.Writer, groups []grouper.Group, format string) error {
	switch format {
	case "yaml":
		return formatter.FormatYAML(w, groups)
	default:
		return formatter.FormatMarkdown(w, groups)
	}
}
