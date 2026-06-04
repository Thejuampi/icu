package main

import (
	"bytes"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	prev := stdoutOverride
	stdoutOverride = &buf
	defer func() { stdoutOverride = prev }()

	err := fn()
	return buf.String(), err
}
