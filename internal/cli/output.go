package cli

import (
	"fmt"
	"os"
)

// jsonMode controls whether Summary suppresses its output.
// Set by root command flag wiring before any command runs.
var jsonMode bool

// SetJSONMode configures whether Summary output is suppressed.
func SetJSONMode(v bool) { jsonMode = v }

// Logf writes a formatted message to stderr only when verbose is true.
func Logf(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// Warnf writes a formatted warning message to stderr, always.
func Warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "WARN: "+format+"\n", args...)
}

// Fatalf writes a formatted error message to stderr and exits with ExitFatal.
func Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(ExitFatal)
}

// Summary prints one line per file path to stdout.
// Suppressed entirely when --json mode is active.
func Summary(files []string) {
	if jsonMode {
		return
	}
	for _, f := range files {
		fmt.Println(f)
	}
}
