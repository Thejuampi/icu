package main

import (
	"testing"
)

func TestOpenBrowserURLEmptyReturnsError(t *testing.T) {
	t.Parallel()

	err := openBrowserURL("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}
