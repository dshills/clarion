package cli

// Exit code constants used by all commands.
// All os.Exit calls in the codebase must use these constants.
const (
	ExitSuccess = 0 // operation completed successfully
	ExitFailure = 1 // verification or drift failure
	ExitFatal   = 2 // fatal error preventing execution
)
