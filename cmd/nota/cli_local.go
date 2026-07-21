package main

import (
	"fmt"
	"os"

	"github.com/urso/nota/pkg/extension"
	"github.com/urso/nota/pkg/extract"
	"github.com/urso/nota/pkg/grouper"
)

// LocalCmd groups source file operations.
type LocalCmd struct {
	List    ListCmd    `cmd:"" default:"withargs" help:"List comments in source files"`
	Extract ExtractCmd `cmd:"" help:"Extract comments to .nota/"`
	Delete  DeleteCmd  `cmd:"" help:"Delete comments from source"`
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
		return fmt.Errorf("could not create backup file: %w", err)
	}
	if err := writeOutput(backup, groups, c.Format); err != nil {
		backup.Close()
		return fmt.Errorf("could not write backup: %w", err)
	}
	backup.Close()
	fmt.Fprintf(os.Stderr, "backup: %s\n", backup.Name())

	return extract.DeleteFromFiles(fileContents, fileRanges, filePerms)
}
