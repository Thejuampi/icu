package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestSmallHelperEdgeCases(t *testing.T) {
	t.Parallel()
	if workoutActivityDate(nil) != "" || workoutActivityDate(&icu.Activity{StartDateLocal: "2026"}) != "" ||
		workoutActivityDate(&icu.Activity{StartDateLocal: "2026-10-26T07:00:00"}) != "2026-10-26" {
		t.Fatal("workoutActivityDate")
	}
	if isHTTPNotFound(nil) || isHTTPNotFound(errors.New("HTTP status error 500")) ||
		!isHTTPNotFound(errors.New("HTTP status error 404: missing")) {
		t.Fatal("isHTTPNotFound")
	}
	operation := &icu.CalendarOperation{EventID: 7}
	if appliedCalendarEventID(operation, &icu.Event{ID: 9}) != 9 || appliedCalendarEventID(operation, &icu.Event{}) != 7 {
		t.Fatal("appliedCalendarEventID")
	}
}

func TestRebalanceApproveRejectsMissingReasonAndLimits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.json")
	outside := filepath.Join(dir, "outside.json")
	if err := os.WriteFile(plain, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte(`{"envelope":{"outsideEnvelope":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	command := rebalanceApproveCommand()
	if err := command.Run(nil, map[string]string{"file": plain}, nil, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "reason") {
		t.Fatalf("missing reason: %v", err)
	}
	if err := command.Run(nil, map[string]string{"file": outside, "reason": "r"}, nil, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "outside envelope") {
		t.Fatalf("outside envelope: %v", err)
	}
	if err := command.Run(nil, map[string]string{}, nil, io.Discard); err == nil {
		t.Fatal("missing file accepted")
	}
}
