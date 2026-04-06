package util

import "fmt"

// Exit codes per spec section 5.3
const (
	ExitOK       = 0
	ExitError    = 1
	ExitNotFound = 2
	ExitTimeout  = 3
)

// CLIError wraps an error with a specific exit code.
type CLIError struct {
	Code    int
	Message string
}

func (e *CLIError) Error() string { return e.Message }

func NotFoundError(kind, name, ns string) error {
	return &CLIError{Code: ExitNotFound, Message: fmt.Sprintf("%s %q not found in namespace %q", kind, name, ns)}
}

func TimeoutError(kind, name string) error {
	return &CLIError{Code: ExitTimeout, Message: fmt.Sprintf("timed out waiting for %s %q to become ready", kind, name)}
}
