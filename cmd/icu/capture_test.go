package main

import (
	"bytes"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	var buf bytes.Buffer

	stdoutOverrideMu.Lock()
	prev := stdoutOverride
	stdoutOverride = &buf
	stdoutOverrideMu.Unlock()

	defer func() {
		stdoutOverrideMu.Lock()
		stdoutOverride = prev
		stdoutOverrideMu.Unlock()
	}()

	err := fn()

	return buf.String(), err
}
