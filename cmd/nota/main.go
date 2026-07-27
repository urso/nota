package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("nota"),
		kong.Description("Extract and track code review comments"),
		kong.UsageOnError(),
	)
	if err := ctx.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
