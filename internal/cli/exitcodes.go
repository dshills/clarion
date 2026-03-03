package cli

import "errors"

// Exit code constants used by all commands.
// All os.Exit calls in the codebase must use these constants.
const (
	ExitSuccess = 0 // operation completed successfully
	ExitFailure = 1 // verification or drift failure
	ExitFatal   = 2 // fatal error preventing execution
)

// ErrVerifyFailed is returned by the verify command when documentation
// contains unsupported claims. main maps this to ExitFailure (1) rather
// than ExitFatal (2) so callers can distinguish user-facing failures from
// program errors.
var ErrVerifyFailed = errors.New("verification failed: documentation contains unsupported claims")
