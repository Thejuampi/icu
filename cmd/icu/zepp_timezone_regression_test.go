package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

// exportBandHR runs the heart-rate command against the same band_data fixture
// and returns the exported timestamps. The profile state is simulated by a
// mock intervals.icu server serving /athlete: tzName is the profile timezone
// ("America/New_York", "Europe/London"), or empty when intervals.icu is down.
func exportBandHR(t *testing.T, tzName string) []int64 {
	t.Helper()

	withZeppTestEnv(t)

	encodedHR := base64.StdEncoding.EncodeToString([]byte{72, 95, 110})

	srv := newZeppMockServer(func(_ zeppMockRequest) (int, string) {
		body := `{"code":1,"message":"success","data":[{"date_time":"2026-06-01","summary":"","data":"","data_hr":"` + encodedHR + `"}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	// The profile state is only a mock here: the export must not read it.
	// A profile-based default (the wrong behavior) would read it and stamp
	// the record with the profile's zone instead of the fixed default.
	athleteSrv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if tzName == "" {
			writer.WriteHeader(http.StatusInternalServerError)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"i1","timezone":"` + tzName + `"}`))
	}))
	t.Cleanup(athleteSrv.Close)

	client := icu.NewClient("key", "i1", icu.WithHTTPClient(athleteSrv.Client()), icu.WithBaseURL(athleteSrv.URL))

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "heart-rate")

	var stdout bytes.Buffer

	if err := cmd.Run(nil, map[string]string{
		"source": "band",
		"oldest": "2026-06-01",
		"newest": "2026-06-01",
	}, client, &stdout, io.Discard); err != nil {
		t.Fatalf("heart-rate: %v", err)
	}

	var series [][]icu.BandDataHeartPoint
	if err := json.Unmarshal(stdout.Bytes(), &series); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}

	var timestamps []int64
	for _, points := range series {
		for _, p := range points {
			timestamps = append(timestamps, p.Timestamp)
		}
	}

	return timestamps
}

// Historical exports must not depend on the profile, the system timezone, or
// the machine: re-exporting the same record must produce the same timestamps
// regardless of where the CLI runs or which profile is configured.
func TestZeppBandHRExportStableAcrossProfile(t *testing.T) {
	ny := exportBandHR(t, "America/New_York")
	london := exportBandHR(t, "Europe/London")
	down := exportBandHR(t, "")

	if len(ny) != 3 {
		t.Fatalf("exported %d points, want 3", len(ny))
	}

	for i := range ny {
		if london[i] != ny[i] || down[i] != ny[i] {
			t.Fatalf("point %d: NY=%d London=%d down=%d — historical exports must not depend on the profile", i, ny[i], london[i], down[i])
		}
	}

	// The default anchor is UTC midnight of the day plus the minute offset.
	midnight := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	for i, ts := range ny {
		if want := midnight + int64(i)*60_000; ts != want {
			t.Fatalf("point %d timestamp = %d, want UTC midnight + %d (%d)", i, ts, i*60_000, want)
		}
	}
}
