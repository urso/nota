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
	"github.com/urso/nota/pkg/formatter"
	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/thread"
)

// CLI is the root command structure for nota.
type CLI struct {
	Local    LocalCmd    `cmd:"" help:"Source file operations"`
	Thread   ThreadCmd   `cmd:"" help:"Thread management"`
	Sync     SyncCmd     `cmd:"" help:"GitHub sync"`
	Trace    TraceCmd    `cmd:"" help:"Update anchor positions to HEAD"`
	Init     InitCmd     `cmd:"" help:"Create .nota/ directory"`
	Validate ValidateCmd `cmd:"" help:"Validate thread files"`
	Behavior BehaviorCmd `cmd:"" help:"Show tag behaviors"`
	Daemon   DaemonCmd   `cmd:"" help:"Start JSON-RPC daemon on stdio"`
}

// InitCmd creates the .nota/ directory.
type InitCmd struct{}

func (c *InitCmd) Run() error {
	dir := filepath.Join(projectRoot(), ".nota")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create .nota/: %w", err)
	}
	fmt.Printf("Initialized %s\n", dir)
	return nil
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

// ValidateCmd validates thread XML files.
type ValidateCmd struct {
	Files []string `arg:"" optional:"" help:"Thread files to validate (default: all .nota/*.xml)"`
}

func (c *ValidateCmd) Run() error {
	return c.run(os.Stdout, os.Stderr, projectRoot())
}

func (c *ValidateCmd) run(stdout, stderr io.Writer, root string) error {
	dir := filepath.Join(root, ".nota")

	files := c.Files
	if len(files) == 0 {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("reading .nota/: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".xml") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(stdout, "No thread files found")
		return nil
	}

	var failed []string
	for _, f := range files {
		th, err := thread.ReadThread(f)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", f, err)
			failed = append(failed, f)
			continue
		}
		if err := thread.ValidateThread(th); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", f, err)
			failed = append(failed, f)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("validation failed for %d file(s)", len(failed))
	}
	fmt.Fprintf(stdout, "Validated %d thread file(s)\n", len(files))
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
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
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
