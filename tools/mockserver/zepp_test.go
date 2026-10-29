package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func zeppGet(t *testing.T, url, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if token != "" {
		req.Header.Set("Apptoken", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)

	return resp.StatusCode, body
}

func TestZeppMockBandAndEvents(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	code, body := zeppGet(t, srv.URL+mockZeppBandPath+"?from_date=2026-08-10&to_date=2026-08-11", "tok")
	data, _ := body["data"].([]any)
	if code != http.StatusOK || len(data) != 2 {
		t.Fatalf("band code=%d body=%v", code, body)
	}
	first, _ := data[0].(map[string]any)
	encoded, _ := first["summary"].(string)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	want := mockZeppDayValues("2026-08-10")
	if err != nil || !strings.Contains(string(raw), `"rhr":`) || first["date_time"] != "2026-08-10" {
		t.Fatalf("summary %s %v", raw, err)
	}
	var summary struct {
		Slp struct {
			RHR int `json:"rhr"`
			SS  int `json:"ss"`
		} `json:"slp"`
	}
	if json.Unmarshal(raw, &summary) != nil || summary.Slp.RHR != want.RestingHR || summary.Slp.SS != want.SleepScore {
		t.Fatalf("summary %s want %+v", raw, want)
	}

	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
	to := time.Date(2026, 8, 11, 23, 59, 59, 0, time.UTC).UnixMilli()
	q := "?eventType=HRVRMSSD&subType=real_data&from=" + itoa(from) + "&to=" + itoa(to)
	code, body = zeppGet(t, srv.URL+mockZeppEventsPath+q, "tok")
	items, _ := body["items"].([]any)
	if code != http.StatusOK || len(items) != 2 {
		t.Fatalf("events code=%d body=%v", code, body)
	}
	// Real shape: value is an object with startTime and samples, never a scalar.
	item0, _ := items[0].(map[string]any)
	val0, _ := item0["value"].(map[string]any)
	if samples, _ := val0["samples"].([]any); len(samples) == 0 || val0["startTime"] == nil {
		t.Fatalf("HRVRMSSD shape %v", item0)
	}
	q = "?eventType=readiness&subType=watch_score&from=" + itoa(from) + "&to=" + itoa(to)
	_, body = zeppGet(t, srv.URL+mockZeppEventsPath+q, "tok")
	items, _ = body["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		v, _ := m["value"].(map[string]any)
		if hrv, ok := v["sleepHRV"].(float64); !ok || hrv <= 0 {
			t.Fatalf("readiness shape %v", it)
		}
	}

	// Other event types are empty; before the first day nothing at all.
	if _, body = zeppGet(t, srv.URL+mockZeppEventsPath+"?eventType=Charge&from=1&to=2", "tok"); listLen(body["items"]) != 0 {
		t.Fatalf("other type %v", body)
	}
	if _, body = zeppGet(t, srv.URL+mockZeppBandPath+"?from_date=2025-12-01&to_date=2025-12-31", "tok"); listLen(body["data"]) != 0 {
		t.Fatalf("pre-history %v", body)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestZeppMockScenarios(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	cases := []struct {
		path, token string
		want        int
	}{
		{mockZeppBandPath + "?from_date=2026-08-10&to_date=2026-08-10", "", http.StatusUnauthorized},
		{mockZeppBandPath + "?from_date=2026-08-10&to_date=2026-08-10", mockZeppTokenExpired, http.StatusUnauthorized},
		{mockZeppEventsPath + "?eventType=HRVRMSSD&from=1&to=2", mockZeppTokenExpired, http.StatusUnauthorized},
		{mockZeppEventsPath + "?eventType=HRVRMSSD&from=1&to=2", mockZeppTokenEventsDown, http.StatusServiceUnavailable},
		{mockZeppBandPath + "?from_date=2026-08-10&to_date=2026-08-10", mockZeppTokenEventsDown, http.StatusOK},
		{mockZeppBandPath + "?from_date=bad&to_date=2026-08-10", "tok", http.StatusBadRequest},
		{mockZeppEventsPath + "?eventType=HRVRMSSD&from=x&to=2", "tok", http.StatusBadRequest},
	}
	for _, tc := range cases {
		if code, _ := zeppGet(t, srv.URL+tc.path, tc.token); code != tc.want {
			t.Errorf("%s token=%q: %d want %d", tc.path, tc.token, code, tc.want)
		}
	}
}

func TestWellnessBulkIsRecordedAndVisible(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	body := `[{"id":"2026-10-10","hrv":44.5,"locked":true},{"id":"2026-10-11","restingHR":50}]`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/v1/athlete/i5/wellness-bulk", strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var echoed []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&echoed)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(echoed) != 2 {
		t.Fatalf("put %d %v", resp.StatusCode, echoed)
	}

	_, writes := zeppGet(t, srv.URL+"/_mock/writes", "")
	if writes["count"] != float64(1) {
		t.Fatalf("writes %v", writes)
	}

	resp, err = http.Get(srv.URL + "/api/v1/athlete/0/wellness?oldest=2026-10-10&newest=2026-10-11") //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	byID := map[string]map[string]any{}
	for _, item := range list {
		if id, ok := item["id"].(string); ok {
			byID[id] = item
		}
	}
	if len(byID) != 2 || byID["2026-10-10"]["hrv"] != 44.5 || byID["2026-10-10"]["ctl"] != float64(51) || byID["2026-10-11"]["restingHR"] != float64(50) {
		t.Fatalf("list %v", list)
	}

	// A non-array body is still recorded and answers [].
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/v1/athlete/0/wellness-bulk", strings.NewReader(`{}`))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func listLen(v any) int {
	list, ok := v.([]any)
	if !ok {
		return -1
	}

	return len(list)
}

// mockPartialSleepDay is the fixture day whose intervals.icu sleepSecs is a
// suspected partial sync (under half of the synthetic Zepp night): the RC
// scenario for the wellness sync partial-sleep flag.
const mockPartialSleepDay = "2026-08-11"

func TestMockPartialSleepScenario(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "api", "v1", "athlete", "0", "wellness.json"))
	if err != nil {
		t.Fatal(err)
	}
	var days []struct {
		ID        string `json:"id"`
		SleepSecs int    `json:"sleepSecs"`
	}
	if err := json.Unmarshal(raw, &days); err != nil {
		t.Fatal(err)
	}
	v := mockZeppDayValues(mockPartialSleepDay)
	zeppSecs := (v.DeepMinutes + v.LightMinutes) * 60
	for _, day := range days {
		if day.ID != mockPartialSleepDay {
			continue
		}
		if day.SleepSecs <= 0 || 2*day.SleepSecs >= zeppSecs {
			t.Fatalf("fixture %s sleepSecs=%d, want >0 and < half of Zepp %d", day.ID, day.SleepSecs, zeppSecs)
		}

		return
	}
	t.Fatalf("fixture day %s missing", mockPartialSleepDay)
}
