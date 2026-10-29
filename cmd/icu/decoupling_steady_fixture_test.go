package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// fixtureServer serves testdata/fixtures read-only, the way icu-mock lays
// them out (/api/v1/<path> -> <path>.json); anything missing is [].
func fixtureServer(t *testing.T) *icu.Client {
	t.Helper()
	root, err := os.OpenRoot(filepath.Join("..", "..", "testdata", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("non-GET %s %s", request.Method, request.URL.Path)
		}
		path := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/"), ".json")
		// os.Root confines reads to the fixture tree.
		if body, err := root.ReadFile(path + ".json"); err == nil {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write(body) //nolint:gosec // test-only JSON fixture from the confined root, served to the test client

			return
		}
		_, _ = writer.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	return icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))
}

// i900000004 is a synthetic steady Z2 ride (95 min at ~170 W, FTP 266, no
// coasting, HR settles in the first minutes): every coach criterion passes,
// so the stream value is valid and consumed.
func TestSteadyRideFixtureIsValid(t *testing.T) {
	t.Parallel()
	out := runDecouplingCommand(t, fixtureServer(t), []string{"i900000004"}, map[string]string{decouplingSourceFlag: "streams"})
	failed, _ := out["failedCriteria"].([]any)
	value, _ := out["streamsDecoupling"].(float64)
	if out["validity"] != icu.DecouplingValid || len(failed) != 0 || value <= 0 || value >= icu.CoachDecouplingGood {
		t.Fatalf("steady ride = validity %v failed %v value %v", out["validity"], out["failedCriteria"], out["streamsDecoupling"])
	}
}
