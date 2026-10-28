package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// decouplingFake serves GET-only activity, streams, events and
// sport-settings endpoints, counting stream fetches. Activity ids containing "fail" return
// 500 on streams; "short" returns a 6-minute ride.
type decouplingFake struct {
	streamCalls, eventCalls, settingsCalls atomic.Int32
	server                                 *httptest.Server
}

func newDecouplingFake(t *testing.T) (*decouplingFake, *icu.Client) {
	t.Helper()
	fake := &decouplingFake{}
	workoutDoc := map[string]any{"steps": []any{
		map[string]any{"duration": 600, "ramp": true, "power": map[string]any{"start": 50, "end": 65}},
		map[string]any{"duration": 3600, "power": map[string]any{"value": 65}},
		map[string]any{"duration": 600, "power": map[string]any{"value": 50}},
	}}
	wavesDoc := map[string]any{
		"steps": []any{
			map[string]any{"duration": 600, "ramp": true, "power": map[string]any{"start": 50, "end": 65}},
			map[string]any{"reps": 12, "steps": []any{
				map[string]any{"duration": 150, "power": map[string]any{"value": 72}},
				map[string]any{"duration": 35, "power": map[string]any{"value": 54}},
			}},
			map[string]any{"duration": 600, "power": map[string]any{"value": 50}},
		},
		"zoneTimes": []any{map[string]any{"id": "Z1", "max": 55}, map[string]any{"id": "Z2", "max": 75}},
	}
	// No zoneTimes: 62/68/72 % FTP are three raw targets but one athlete
	// zone (Z2 56-75 %), so it is steady only with the sport-settings zones.
	sameZoneDoc := map[string]any{"steps": []any{
		map[string]any{"duration": 600, "ramp": true, "power": map[string]any{"start": 50, "end": 65, "units": "%ftp"}},
		map[string]any{"duration": 1200, "power": map[string]any{"value": 62, "units": "%ftp"}},
		map[string]any{"duration": 1200, "power": map[string]any{"value": 68, "units": "%ftp"}},
		map[string]any{"duration": 1200, "power": map[string]any{"value": 72, "units": "%ftp"}},
		map[string]any{"duration": 600, "power": map[string]any{"value": 50, "units": "%ftp"}},
	}}
	fake.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("non-GET request %s %s", request.Method, request.URL.Path)
		}
		path := request.URL.Path
		switch {
		case strings.HasSuffix(path, "/streams"):
			fake.streamCalls.Add(1)
			if strings.Contains(path, "fail") {
				http.Error(writer, "boom", http.StatusInternalServerError)

				return
			}
			minutes := 90
			if strings.Contains(path, "short") {
				minutes = 6
			}
			_ = json.NewEncoder(writer).Encode(synthStreamsJSON(minutes))
		case strings.HasSuffix(path, "/sport-settings/Ride"):
			fake.settingsCalls.Add(1)
			_ = json.NewEncoder(writer).Encode(map[string]any{"ftp": 250, "powerZones": []int{55, 75, 90, 105, 120, 150, 999}})
		case strings.Contains(path, "/events/"):
			fake.eventCalls.Add(1)
			_ = json.NewEncoder(writer).Encode(map[string]any{"id": 77, "workoutDoc": workoutDoc})
		case strings.HasSuffix(path, "/events"):
			fake.eventCalls.Add(1)
			_ = json.NewEncoder(writer).Encode([]map[string]any{{"id": 77, "workoutDoc": workoutDoc}, {"id": 78}, {"id": 79, "workoutDoc": wavesDoc}, {"id": 80, "workoutDoc": sameZoneDoc}})
		case strings.Contains(path, "/activity/"):
			id := path[strings.LastIndex(path, "/")+1:]
			paired := 77
			if strings.Contains(id, "waves") {
				paired = 79
			}
			if strings.Contains(id, "samezone") {
				paired = 80
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": id, "name": "Z2", "type": "Ride", "decoupling": 8.26, "movingTime": 5400,
				"startDateLocal": "2026-10-27T08:00:00", "pairedEventId": paired, "icuVariabilityIndex": 1.05,
				"analyzed": "2026-10-27T10:00:00Z",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(fake.server.Close)

	return fake, icu.NewClient("k", "0", icu.WithHTTPClient(fake.server.Client()), icu.WithBaseURL(fake.server.URL))
}

func decouplingActivities() []icu.Activity {
	event := 77

	return []icu.Activity{
		{ID: "i1", Type: "Ride", Decoupling: 8.26, AverageHeartRate: 135, StartDateLocal: "2026-10-27T08:00:00", PairedEventID: &event, Analyzed: "a1"},
		{ID: "ifail", Type: "Ride", Decoupling: 5.5, AverageHeartRate: 140},
		{ID: "ishort", Type: "VirtualRide", AverageHeartRate: 140},
		{ID: "i3", Type: "Swim", Decoupling: 3},
	}
}

//nolint:paralleltest // t.Setenv
func TestEnrichActivitiesDecouplingStreamsMode(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	fake, client := newDecouplingFake(t)
	streams := map[string]string{decouplingSourceFlag: "streams"}

	activities := decouplingActivities()
	if err := enrichActivitiesDecoupling(client, activities, streams); err != nil {
		t.Fatal(err)
	}
	i1 := activities[0]
	if i1.DecouplingSource != icu.DecouplingSourceStreams || i1.Decoupling != 0 || i1.DecouplingStreams == nil ||
		i1.DecouplingAPI == nil || *i1.DecouplingAPI != 8.26 || i1.DecouplingStreamsStatus != icu.DecouplingStatusMeasured {
		t.Fatalf("i1 = %+v, want streams 0 with api 8.26", i1)
	}
	if fail := activities[1]; fail.DecouplingSource != icu.DecouplingSourceAPI || fail.Decoupling != 5.5 ||
		fail.DecouplingStreamsStatus != icu.DecouplingStatusUnavailable || fail.DecouplingNote == "" {
		t.Fatalf("ifail = %+v, want api 5.5 unavailable", fail)
	}
	if short := activities[2]; short.DecouplingSource != icu.DecouplingSourceAPI || short.DecouplingStreams != nil ||
		short.DecouplingStreamsStatus != icu.DecouplingStatusNotMeasurable || short.HasDecoupling() {
		t.Fatalf("ishort = %+v, want api, not_measurable", short)
	}
	if swim := activities[3]; swim.DecouplingSource != icu.DecouplingSourceAPI || swim.DecouplingStreamsStatus != "" {
		t.Fatalf("swim = %+v, want api, not fetched", swim)
	}
	if fake.streamCalls.Load() != 3 || fake.eventCalls.Load() != 1 {
		t.Fatalf("stream calls %d event calls %d, want 3 and 1", fake.streamCalls.Load(), fake.eventCalls.Load())
	}

	// Cached: i1 measured (no refetch), ishort not measured (cached with TTL),
	// ifail (fetch error, never cached) refetched.
	again := decouplingActivities()
	if err := enrichActivitiesDecoupling(client, again, streams); err != nil {
		t.Fatal(err)
	}
	if fake.streamCalls.Load() != 4 || again[0].DecouplingSource != icu.DecouplingSourceStreams {
		t.Fatalf("cached run: calls %d i1 %+v", fake.streamCalls.Load(), again[0])
	}

	// An activity edit (new analyzed timestamp) invalidates i1's entry.
	edited := decouplingActivities()
	edited[0].Analyzed = "a2"
	if err := enrichActivitiesDecoupling(client, edited, streams); err != nil {
		t.Fatal(err)
	}
	if fake.streamCalls.Load() != 6 {
		t.Fatalf("after edit calls = %d, want 6 (i1 refetched, ifail retried)", fake.streamCalls.Load())
	}

	// Different options are a different key.
	changed := decouplingActivities()
	changed[0].Analyzed = "a2"
	if err := enrichActivitiesDecoupling(client, changed, map[string]string{decouplingSourceFlag: "streams", decouplingIQRKFlag: "3"}); err != nil {
		t.Fatal(err)
	}
	if fake.streamCalls.Load() != 9 {
		t.Fatalf("after option change calls = %d, want 9", fake.streamCalls.Load())
	}

	// Not-measured entries expire: age ishort's entry past its TTL.
	var aged decouplingCacheEntry
	data, err := os.ReadFile(decouplingCachePath("ishort"))
	if err != nil || json.Unmarshal(data, &aged) != nil || aged.ExpiresAt == "" {
		t.Fatalf("ishort cache entry: %v %+v", err, aged)
	}
	aged.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	writeDecouplingCache("ishort", &aged)
	expired := decouplingActivities()
	expired[0].Analyzed = "a2"
	if err := enrichActivitiesDecoupling(client, expired, streams); err != nil {
		t.Fatal(err)
	}
	if fake.streamCalls.Load() != 12 {
		t.Fatalf("after TTL calls = %d, want 12 (ishort expired, ifail retried, i1 back to the default key)", fake.streamCalls.Load())
	}
}

//nolint:paralleltest // t.Setenv
func TestEnrichActivitiesDecouplingAPIModeIsDefault(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	fake, client := newDecouplingFake(t)
	activities := decouplingActivities()
	if err := enrichActivitiesDecoupling(client, activities, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if fake.streamCalls.Load() != 0 || fake.eventCalls.Load() != 0 {
		t.Fatalf("api mode fetched streams/events")
	}
	if activities[0].DecouplingSource != icu.DecouplingSourceAPI || activities[0].Decoupling != 8.26 ||
		activities[2].DecouplingSource != icu.DecouplingSourceAPI || activities[2].DecouplingAPI != nil {
		t.Fatalf("api mode = %+v", activities)
	}
	if err := enrichActivitiesDecoupling(client, activities, map[string]string{decouplingSourceFlag: "nope"}); err == nil {
		t.Fatal("bad source accepted")
	}
}

func TestDecouplingOptionsFromFlags(t *testing.T) {
	t.Parallel()
	options, mode, err := decouplingOptionsFromFlags(map[string]string{
		decouplingSourceFlag: "STREAMS", decouplingWarmupFlag: "15", decouplingCooldownFlag: "5",
		decouplingHRLagFlag: "30", decouplingIQRKFlag: "3",
	})
	if err != nil || mode != decouplingModeStreams {
		t.Fatalf("err = %v mode = %q", err, mode)
	}
	if *options.WarmupSeconds != 900 || *options.CooldownSeconds != 300 || *options.HRLagSeconds != 30 || options.IQRK != 3 {
		t.Fatalf("options = %+v", options)
	}
	defaults, mode, err := decouplingOptionsFromFlags(map[string]string{})
	if err != nil || mode != decouplingModeAPI || defaults.WarmupSeconds != nil || defaults.HRLagSeconds != nil ||
		defaults.IQRK != icu.DefaultDecouplingIQRK {
		t.Fatalf("defaults = %+v %s %v", defaults, mode, err)
	}
	for _, bad := range []map[string]string{
		{decouplingSourceFlag: "x"},
		{decouplingIQRKFlag: "-1"},
		{decouplingWarmupFlag: "abc"},
		{decouplingWarmupFlag: "1.5"},
		{decouplingCooldownFlag: "-2"},
		{decouplingHRLagFlag: "2.5"},
	} {
		if _, _, err := decouplingOptionsFromFlags(bad); err == nil {
			t.Fatalf("flags %v: want error", bad)
		}
	}
}

func runDecouplingCommand(t *testing.T, client *icu.Client, args []string, flags map[string]string) map[string]any {
	t.Helper()
	out, err := captureStdout(t, func() error { return activityDecouplingCommand().Run(args, flags, client) })
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}

	return decoded
}

//nolint:paralleltest // t.Setenv and captureStdout
func TestActivityDecouplingCommand(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	fake, client := newDecouplingFake(t)

	got := runDecouplingCommand(t, client, []string{"i9"}, map[string]string{})
	if got["apiDecoupling"] != 8.26 || got["decoupling"] != 8.26 || got["decouplingSource"] != icu.DecouplingSourceAPI ||
		got["streamsDecoupling"] != 0.0 || got["streamsStatus"] != icu.DecouplingStatusMeasured {
		t.Fatalf("api default output = %v", got)
	}
	detail, ok := got["streamDetail"].(map[string]any)
	trim, _ := detail["trim"].(map[string]any)
	if !ok || detail["validity"] != icu.DecouplingValid || trim["method"] != icu.DecouplingTrimCoachFixed {
		t.Fatalf("streamDetail = %v", got["streamDetail"])
	}
	if fake.settingsCalls.Load() != 1 || fake.eventCalls.Load() != 1 {
		t.Fatalf("settings %d events %d", fake.settingsCalls.Load(), fake.eventCalls.Load())
	}

	summary, _ := got["summary"].(map[string]any)
	if got["experimental"] != true || summary["status"] != icu.DecouplingStatusMeasured || summary["stopRule"] == nil ||
		summary["validity"] != icu.DecouplingValid || summary["interpretation"] != icu.DecouplingInterpretationGood ||
		summary["workoutKind"] != icu.DecouplingWorkoutSteady || summary["failedCriteria"] != nil {
		t.Fatalf("summary = %v", got["summary"])
	}

	// The workout kind is informational: a steady ride paired with an
	// interval workout is measured when it meets the coaching criteria.
	waves := runDecouplingCommand(t, client, []string{"iwaves"}, map[string]string{decouplingSourceFlag: "streams"})
	wavesSummary, _ := waves["summary"].(map[string]any)
	if waves["decouplingSource"] != icu.DecouplingSourceStreams || wavesSummary["workoutKind"] != icu.DecouplingWorkoutIntervals {
		t.Fatalf("waves output = %v", waves)
	}

	// Failed criteria: not valid, no stream number, the API value kept and
	// the failed criterion named.
	short := runDecouplingCommand(t, client, []string{"ishort"}, map[string]string{decouplingSourceFlag: "streams"})
	shortSummary, _ := short["summary"].(map[string]any)
	failedCriteria, _ := shortSummary["failedCriteria"].([]any)
	if short["decoupling"] != 8.26 || short["decouplingSource"] != icu.DecouplingSourceAPI || short["streamsDecoupling"] != nil ||
		shortSummary["validity"] != icu.DecouplingNotValid || len(failedCriteria) == 0 || failedCriteria[0] != icu.CriterionDuration {
		t.Fatalf("short output = %v", short)
	}

	// Command-side zone fallback: the athlete's sport-settings zones classify
	// a doc without zoneTimes (raw targets would read it as intervals).
	sameZone := runDecouplingCommand(t, client, []string{"isamezone"}, map[string]string{})
	if zoneSummary, _ := sameZone["summary"].(map[string]any); zoneSummary["workoutKind"] != icu.DecouplingWorkoutSteady {
		t.Fatalf("same-zone summary = %v, want steady via athlete zones", sameZone["summary"])
	}

	streams := runDecouplingCommand(t, client, []string{"i9"}, map[string]string{decouplingSourceFlag: "streams"})
	if streams["decoupling"] != 0.0 || streams["decouplingSource"] != icu.DecouplingSourceStreams {
		t.Fatalf("streams output = %v", streams)
	}

	failed := runDecouplingCommand(t, client, []string{"ifail"}, map[string]string{decouplingSourceFlag: "streams"})
	if failed["decoupling"] != 8.26 || failed["streamsStatus"] != icu.DecouplingStatusUnavailable ||
		failed["streamsDecoupling"] != nil || failed["decouplingNote"] == nil {
		t.Fatalf("failed output = %v", failed)
	}

	if err := activityDecouplingCommand().Run(nil, map[string]string{}, client); err == nil {
		t.Fatal("missing id accepted")
	}
	if err := activityDecouplingCommand().Run([]string{"i9"}, map[string]string{decouplingIQRKFlag: "x"}, client); err == nil {
		t.Fatal("bad flag accepted")
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	broken := icu.NewClient("k", "0", icu.WithHTTPClient(closed.Client()), icu.WithBaseURL(closed.URL))
	if err := activityDecouplingCommand().Run([]string{"i9"}, map[string]string{}, broken); err == nil {
		t.Fatal("activity fetch error swallowed")
	}
}

//nolint:paralleltest // t.Setenv
func TestDecouplingCacheHelpers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ICU_CONFIG_DIR", dir)
	if decouplingCachePath("../x") != "" || decouplingCachePath("") != "" {
		t.Fatal("unsafe ids must not map to a cache path")
	}
	entry := decouplingCacheEntry{Key: "k"}
	writeDecouplingCache("i1", &entry)
	if _, ok := readDecouplingCache("i1", "k"); !ok {
		t.Fatal("cache miss after write")
	}
	if _, ok := readDecouplingCache("i1", "other"); ok {
		t.Fatal("key mismatch hit")
	}
	entry.ExpiresAt = "garbage"
	writeDecouplingCache("i1", &entry)
	if _, ok := readDecouplingCache("i1", "k"); ok {
		t.Fatal("unparseable expiry hit")
	}
	invalidateDecouplingCache("i1")
	if _, err := os.Stat(decouplingCachePath("i1")); !os.IsNotExist(err) {
		t.Fatalf("invalidate left the file: %v", err)
	}
	t.Setenv("ICU_CONFIG_DIR", "")
	t.Setenv("HOME", dir)
	if !strings.HasPrefix(decouplingCacheDir(), dir) {
		t.Fatalf("home cache dir = %s", decouplingCacheDir())
	}
}
