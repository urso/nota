package main

import "fmt"

// SyncCmd is a placeholder for GitHub sync commands (story 0006).
type SyncCmd struct{}

func (c *SyncCmd) Run() error {
	return fmt.Errorf("sync commands not yet implemented (see story 0006)")
}
