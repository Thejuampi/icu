package main

import (
	"bytes"
	"io"
	"testing"
)

// captureStdout runs fn with an injected buffer as the command's stdout and
// returns what it wrote. No process-global state is touched, so callers may
// run in parallel.
func captureStdout(t *testing.T, fn func(stdout io.Writer) error) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	err := fn(&buf)

	return buf.String(), err
}
