package main

import (
	"errors"
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
		// Exit-code policy (see internal/cli/exitcodes.go):
		//   ExitFailure (1) — user-facing outcome (verify/drift threshold exceeded):
		//     the tool ran correctly but a post-condition was not met.
		//   ExitFatal (2) — program error: bad config, I/O failure, etc.
		if errors.Is(err, cli.ErrVerifyFailed) {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(cli.ExitFailure)
		}
		fmt.Fprintln(os.Stderr, "ERROR: "+err.Error())
		os.Exit(cli.ExitFatal)
	}
}
