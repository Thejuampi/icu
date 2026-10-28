package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func eventsTestClient(t *testing.T) (*icu.Client, *commandServer) {
	t.Helper()
	serverState := &commandServer{}
	server := httptest.NewServer(http.HandlerFunc(serverState.handle))
	t.Cleanup(server.Close)

	return icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL)), serverState
}

func runEventsCommand(t *testing.T, client *icu.Client, sub string, args []string, values map[string]string) error {
	t.Helper()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cmd, ok := registry.Lookup("events", sub)
	if !ok {
		t.Fatalf("missing command events %s", sub)
	}

	return cmd.Run(args, values, client, io.Discard, io.Discard)
}

func TestEventsCreateWeightTrainingKeepsTypeWithoutFTPPricing(t *testing.T) {
	t.Parallel()
	client, state := eventsTestClient(t)

	err := runEventsCommand(t, client, "create", nil, flags("type", "WeightTraining", "name", "Strength A",
		"start-date", "2026-06-01", "moving-time", "2700", "training-load", "30", "desc", "Squat 5x5\nRDL 3x8"))

	if err != nil || len(state.Requests) != 1 ||
		!strings.Contains(state.Requests[0].Body, `"type":"WeightTraining"`) ||
		!strings.Contains(state.Requests[0].Body, `"icu_training_load":30`) ||
		!strings.Contains(state.Requests[0].Body, `Squat 5x5\nRDL 3x8`) {
		t.Fatalf("requests = %+v err=%v, want WeightTraining body with given load", state.Requests, err)
	}
}

func TestEventsCreateCalculateLoadRejectsNonCyclingType(t *testing.T) {
	t.Parallel()
	for _, eventType := range []string{"WeightTraining", "Run", "Workout"} {
		client, state := eventsTestClient(t)

		err := runEventsCommand(t, client, "create", nil, flags("type", eventType, "name", "X",
			"start-date", "2026-06-01", "desc", "- 60m 70%", "calculate-load", "true", "ftp", "300"))

		if !errors.Is(err, errInvalidValue) || !strings.Contains(err.Error(), "not FTP-priced") || len(state.Requests) != 0 {
			t.Fatalf("%s: err=%v requests=%d, want not-FTP-priced error and no write", eventType, err, len(state.Requests))
		}
	}
}

// Runs against its own t.TempDir() config dir instead of the shared TestMain
// dir: `events update` takes an event backup before the type check rejects the
// request, and that file must not leak into the dir other tests read.
// t.Setenv rules out t.Parallel.
//
//nolint:paralleltest // t.Setenv
func TestEventsUpdateCalculateLoadRejectsNonCyclingTypeFlag(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	client, state := eventsTestClient(t)

	err := runEventsCommand(t, client, "update", []string{"1"}, flags("type", "Run", "desc", "- 30m 70%",
		"calculate-load", "true", "ftp", "300"))

	for _, request := range state.Requests {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected write %+v", request)
		}
	}
	if err == nil || !strings.Contains(err.Error(), "not FTP-priced") {
		t.Fatalf("err=%v, want not-FTP-priced error", err)
	}
}

func TestEventsUpdateSendsType(t *testing.T) {
	t.Parallel()
	client, state := eventsTestClient(t)

	err := runEventsCommand(t, client, "update", []string{"1"}, flags("type", "WeightTraining", "name", "Strength"))

	if err != nil || len(state.Requests) != 2 || !strings.Contains(state.Requests[1].Body, `"type":"WeightTraining"`) {
		t.Fatalf("requests=%+v err=%v, want PUT with type", state.Requests, err)
	}
}

func TestEventsCreateDescFileKeepsRealNewlines(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "desc.txt")
	if err := os.WriteFile(path, []byte("Warmup\n- 10m 60%\n\nMain\n- 3x10m 90%\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client, state := eventsTestClient(t)

	err := runEventsCommand(t, client, "create", nil, flags("name", "W", "start-date", "2026-06-01", "desc-file", path))

	want := `"description":"Warmup\n- 10m 60%\n\nMain\n- 3x10m 90%"`
	if err != nil || len(state.Requests) != 1 || !strings.Contains(state.Requests[0].Body, want) {
		t.Fatalf("requests=%+v err=%v, want body containing %s", state.Requests, err, want)
	}
}

func TestEventsDescAndDescFileConflict(t *testing.T) {
	t.Parallel()
	client, _ := eventsTestClient(t)

	err := runEventsCommand(t, client, "create", nil, flags("name", "W", "start-date", "2026-06-01",
		"desc", "x", "desc-file", "/nonexistent"))

	if !errors.Is(err, errInvalidValue) {
		t.Fatalf("err=%v, want errInvalidValue", err)
	}
}

func TestEventsRejectLiteralEscapedNewlines(t *testing.T) {
	t.Parallel()
	for _, sub := range []string{"create", "update"} {
		client, state := eventsTestClient(t)
		var args []string
		if sub == "update" {
			args = []string{"1"}
		}

		err := runEventsCommand(t, client, sub, args, flags("name", "W", "start-date", "2026-06-01",
			"desc", `Warmup\n- 10m 60%`))

		if !errors.Is(err, icu.ErrEscapedDescription) || len(state.Requests) != 0 {
			t.Fatalf("%s: err=%v requests=%d, want ErrEscapedDescription before any request", sub, err, len(state.Requests))
		}
	}
}
