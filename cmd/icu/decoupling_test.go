package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func synthStreamsJSON(minutes int) []icu.ActivityStream {
	n := minutes * 60
	timeData, watts, hr := make([]float64, n), make([]float64, n), make([]float64, n)
	for sec := range n {
		timeData[sec], watts[sec], hr[sec] = float64(sec), 150, 135
	}

	return []icu.ActivityStream{
		{Type: "time", Data: timeData}, {Type: "watts", Data: watts}, {Type: "heartrate", Data: hr},
	}
}

func TestEnrichActivitiesDecouplingStreamsCacheAndFallback(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	var streamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("non-GET request %s %s", request.Method, request.URL.Path)
		}
		if !strings.HasSuffix(request.URL.Path, "/streams") {
			http.NotFound(writer, request)

			return
		}
		streamCalls.Add(1)
		if strings.Contains(request.URL.Path, "i2") {
			http.Error(writer, "boom", http.StatusInternalServerError)

			return
		}
		_ = json.NewEncoder(writer).Encode(synthStreamsJSON(90))
	}))
	defer server.Close()
	client := icu.NewClient("k", "0", icu.WithHTTPClient(server.Client()), icu.WithBaseURL(server.URL))

	makeActivities := func() []icu.Activity {
		return []icu.Activity{
			{ID: "i1", Type: "Ride", Decoupling: 8.26, AverageHeartRate: 135},
			{ID: "i2", Type: "Ride", Decoupling: 5.5, AverageHeartRate: 140},
			{ID: "i3", Type: "Swim", Decoupling: 3},
		}
	}
	activities := makeActivities()
	if err := enrichActivitiesDecoupling(client, activities, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if activities[0].DecouplingSource != icu.DecouplingSourceStreams || activities[0].Decoupling != 0 ||
		activities[0].DecouplingAPI == nil || *activities[0].DecouplingAPI != 8.26 {
		t.Fatalf("i1 = %+v, want streams 0 with api 8.26", activities[0])
	}
	if activities[1].DecouplingSource != icu.DecouplingSourceAPI || activities[1].Decoupling != 5.5 {
		t.Fatalf("i2 = %+v, want api_fallback 5.5", activities[1])
	}
	if activities[2].DecouplingSource != icu.DecouplingSourceAPI {
		t.Fatalf("i3 = %+v, want api_fallback (not a candidate)", activities[2])
	}
	if got := streamCalls.Load(); got != 2 {
		t.Fatalf("stream calls = %d, want 2", got)
	}

	// Second run: i1 comes from cache, i2 (fetch error, not cached) retried.
	again := makeActivities()
	if err := enrichActivitiesDecoupling(client, again, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if got := streamCalls.Load(); got != 3 || again[0].DecouplingSource != icu.DecouplingSourceStreams {
		t.Fatalf("after cached run: calls = %d, i1 = %+v", got, again[0])
	}

	// api mode: no stream fetch at all.
	apiMode := makeActivities()
	if err := enrichActivitiesDecoupling(client, apiMode, map[string]string{decouplingSourceFlag: "api"}); err != nil {
		t.Fatal(err)
	}
	if got := streamCalls.Load(); got != 3 || apiMode[0].DecouplingSource != icu.DecouplingSourceAPI || apiMode[0].Decoupling != 8.26 {
		t.Fatalf("api mode: calls = %d, i1 = %+v", got, apiMode[0])
	}
}

func TestDecouplingOptionsFromFlags(t *testing.T) {
	t.Parallel()
	options, mode, err := decouplingOptionsFromFlags(map[string]string{
		decouplingWarmupFlag: "15", decouplingCooldownFlag: "5", decouplingMinPowerFlag: "80",
		decouplingHRLagFlag: "30", decouplingIQRKFlag: "3",
	})
	if err != nil || mode != icu.DecouplingSourceStreams {
		t.Fatalf("err = %v mode = %q", err, mode)
	}
	want := icu.DecouplingOptions{WarmupSeconds: 900, CooldownSeconds: 300, MinPowerWatts: 80, HRLagSeconds: 30, IQRK: 3,
		MinSteadyMinutes: icu.DefaultDecouplingMinSteadyMinutes}
	if options != want {
		t.Fatalf("options = %+v, want %+v", options, want)
	}
	if defaults, _, _ := decouplingOptionsFromFlags(map[string]string{}); defaults != icu.DefaultDecouplingOptions() {
		t.Fatalf("defaults = %+v", defaults)
	}
	for _, bad := range []map[string]string{{decouplingSourceFlag: "x"}, {decouplingIQRKFlag: "-1"}, {decouplingWarmupFlag: "abc"}} {
		if _, _, err := decouplingOptionsFromFlags(bad); err == nil {
			t.Fatalf("flags %v: want error", bad)
		}
	}
}
