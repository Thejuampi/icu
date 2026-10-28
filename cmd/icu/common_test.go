package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteJSONWritesToInjectedWriter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeJSON(&buf, map[string]int{"a": 1}); err != nil || !strings.Contains(buf.String(), `"a": 1`) {
		t.Fatalf("out=%q err=%v", buf.String(), err)
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

func TestWriteOutputReturnsErrorOnWriteFailure(t *testing.T) {
	t.Parallel()

	if err := writeOutput(&errBuf{}, []byte("data")); err == nil {
		t.Fatal("expected write error")
	}
}

type errBuf struct{}

func (b *errBuf) Write(_ []byte) (int, error) {
	return 0, assertionError("write failed")
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
