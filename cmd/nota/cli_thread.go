package main

import "fmt"

// ThreadCmd is a placeholder for thread management commands (story 0003).
type ThreadCmd struct{}

func (c *ThreadCmd) Run() error {
	return fmt.Errorf("thread commands not yet implemented (see story 0003)")
}
