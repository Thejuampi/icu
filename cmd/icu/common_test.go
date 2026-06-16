package main

import (
	"testing"
)

func TestSetStdoutForTestSwitchesOutput(t *testing.T) {
	t.Parallel()

	var buf testBuf

	setStdoutForTest(&buf)

	defer setStdoutForTest(nil)

	if stdoutOverride == nil {
		t.Fatal("stdoutOverride should be set")
	}
}

func TestWrapCommandErrorNilReturnsNil(t *testing.T) {
	t.Parallel()

	if err := wrapCommandError(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWrapCommandErrorWrapsNonNil(t *testing.T) {
	t.Parallel()

	err := wrapCommandError(assertionError("some error"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFloatFlagValParseErrorReturnsDefault(t *testing.T) {
	t.Parallel()

	got := floatFlagVal(map[string]string{"val": "not-a-number"}, "val", 42.5)
	if got != 42.5 {
		t.Fatalf("got %f, want 42.5", got)
	}
}

func TestFloatFlagValMissingKeyReturnsDefault(t *testing.T) {
	t.Parallel()

	got := floatFlagVal(nil, "nonexistent", 10.0)
	if got != 10.0 {
		t.Fatalf("got %f, want 10.0", got)
	}
}

//nolint:paralleltest // sets a package-level stdout override that would race with other writers.
func TestWriteOutputReturnsErrorOnWriteFailure(t *testing.T) {
	var eb errBuf

	setStdoutForTest(&eb)

	defer setStdoutForTest(nil)

	if err := writeOutput([]byte("data")); err == nil {
		t.Fatal("expected write error")
	}
}

type testBuf struct{}

func (b *testBuf) Write(p []byte) (int, error) {
	return len(p), nil
}

type errBuf struct{}

func (b *errBuf) Write(_ []byte) (int, error) {
	return 0, assertionError("write failed")
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
