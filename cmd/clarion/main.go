package main

import (
	"fmt"
	"os"

	"github.com/clarion-dev/clarion/internal/cli"
)

// Populated by -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	built   = "unknown"
)

func main() {
	root := cli.New(version, commit, built)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: "+err.Error())
		os.Exit(cli.ExitFatal)
	}
}
