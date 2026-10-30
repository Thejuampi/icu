package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	icu "github.com/Thejuampi/icu"
)

//nolint:paralleltest // Isolates configuration and credentials with t.Setenv.
func TestWorkoutsCalculateWithoutCredentialsOrNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "unexpected network request", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "")
	t.Setenv("ICU_BASE_URL", server.URL)

	for _, flag := range []string{"--desc", "--description"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"icu", "workouts", "calculate", "--ftp", "300", flag, "- 60m 70%"}, &stdout, &stderr)
			var got icu.PlannedLoadEstimate
			err := json.Unmarshal(stdout.Bytes(), &got)
			if code != 0 || stderr.Len() != 0 || err != nil || got.DurationSeconds != 3600 || got.TrainingLoad != 49 ||
				got.AveragePower != 210 || got.NormalizedPower != 210 || got.IntensityFactor != 0.7 {
				t.Fatalf("calculate: code=%d stdout=%s stderr=%s decode=%v", code, &stdout, &stderr, err)
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("local calculate made %d network requests", hits.Load())
	}
}

//nolint:paralleltest // Isolates configuration and credentials with t.Setenv.
func TestWorkoutsCalculateInvalidInputWithoutCredentials(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "")
	t.Setenv("ICU_BASE_URL", "http://127.0.0.1:1")
	for _, args := range [][]string{
		{"--ftp", "300"},
		{"--ftp", "300", "--desc", "- nonsense"},
		{"--ftp", "300", "--desc", "- 60m 70%", "--bogus"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"icu", "workouts", "calculate"}, args...), &stdout, &stderr)
		if code != 1 || stdout.Len() != 0 || stderr.Len() == 0 || strings.Contains(stderr.String(), "API key required") {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
	}
}

//nolint:paralleltest // Isolates configuration and credentials with t.Setenv.
func TestAPICommandsStillRequireCredentials(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "")
	t.Setenv("ICU_BASE_URL", "http://127.0.0.1:1")
	for _, args := range [][]string{
		{"workouts", "list"},
		{"workouts", "get", "1"},
		{"workouts", "create", "--name", "synthetic", "--type", "Ride"},
		{"workouts", "update", "1", "--name", "synthetic"},
		{"workouts", "delete", "1"},
		{"workouts", "tags"},
		{"athlete", "show"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"icu"}, args...), &stdout, &stderr)
		if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "API key required") {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
	}
}

func TestCommandAuthExemptionIsExact(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	for resource, commands := range registry.commands {
		for action := range commands {
			want := resource != "config" && resource != "zepp" && (resource != "workouts" || action != "calculate")
			if got := commandRequiresAuth(resource, action); got != want {
				t.Errorf("%s %s requires auth=%v, want %v", resource, action, got, want)
			}
		}
	}
}
