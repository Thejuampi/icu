package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestReadRebalanceInputFromScopeReadsAllSources(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", request.Method, request.URL.Path)
		}
		paths = append(paths, request.URL.Path)
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/activities"):
			_, _ = response.Write([]byte(`[{"id":"a1","type":"Ride","start_date_local":"2026-06-02T08:00:00","icu_training_load":50}]`))
		case strings.HasSuffix(request.URL.Path, "/events"):
			_, _ = response.Write([]byte(`[{"id":1,"category":"WORKOUT","type":"VirtualRide","start_date_local":"2026-06-04T08:00:00"}]`))
		case strings.Contains(request.URL.Path, "/sport-settings/"):
			_, _ = response.Write([]byte(`{"ftp":266,"indoor_ftp":250,"types":["Ride","VirtualRide"]}`))
		case strings.HasSuffix(request.URL.Path, "/wellness"):
			_, _ = response.Write([]byte(`[{"id":"2026-06-01","ctl":30,"atl":35}]`))
		default:
			_, _ = response.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	got, err := readRebalanceInputFromScope(client, "2026-06-01", "2026-06-07", 300, "medium", "auto")
	if err != nil {
		t.Fatalf("readRebalanceInputFromScope: %v", err)
	}
	if len(got.Activities) != 1 || len(got.Events) != 1 || got.SportSettings == nil || got.Wellness == nil ||
		got.Scope.StartDate != "2026-06-01" || got.Scope.EndDate != "2026-06-07" || got.NowDate != "2026-06-07" {
		t.Fatalf("input = %+v (paths %v), want every source read", got, paths)
	}
}

func TestReadRebalanceInputFromScopeReportsActivityFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)
	client := icu.NewClient("test-key", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	if _, err := readRebalanceInputFromScope(client, "2026-06-01", "2026-06-07", 300, "", ""); err == nil {
		t.Fatal("error = nil, want upstream failure")
	}
}
