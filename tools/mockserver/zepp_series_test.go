package main

import (
	"net/http"
	"testing"
)

func TestZeppMockLegacyEventsRejectEarlierFormAndPage(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	base := srv.URL + "/users/u1/events?eventType=all_day_stress&from=1786320000000&to=1786579199000"

	// Earlier form: no userId, limit 1000.
	if code, _ := zeppGet(t, base+"&limit=1000", "tok"); code != http.StatusInternalServerError {
		t.Fatalf("earlier form code = %d, want 500", code)
	}

	code, body := zeppGet(t, base+"&userId=u1&limit=2", "tok")
	items, _ := body["items"].([]any)
	next, _ := body["next"].(map[string]any)
	if code != http.StatusOK || len(items) != 2 || next["timestamp"] == nil {
		t.Fatalf("paged code=%d body=%v", code, body)
	}
}

func TestZeppMockSportStatisticsTakeDays(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	base := srv.URL + "/v2/watch/users/u1/WatchSportStatistics/SPORT_LOAD"

	if code, _ := zeppGet(t, base+"?from=1786320000000&to=1786406399000", "tok"); code != http.StatusInternalServerError {
		t.Fatalf("from/to code = %d, want 500", code)
	}

	code, body := zeppGet(t, base+"?startDay=2026-08-10&endDay=2026-08-11&limit=900&isReverse=true", "tok")
	if items, _ := body["items"].([]any); code != http.StatusOK || len(items) != 2 {
		t.Fatalf("code=%d body=%v", code, body)
	}
}

func TestZeppMockManualDataAndBloodPressureForms(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	if code, _ := zeppGet(t, srv.URL+"/v1/user/manualData.json?from=1&to=2", "tok"); code != http.StatusBadRequest {
		t.Fatalf("manual earlier form code = %d, want 400", code)
	}

	code, body := zeppGet(t, srv.URL+"/v1/user/manualData.json?userid=u1&type=sleep", "tok")
	if items, _ := body["items"].([]any); code != http.StatusOK || len(items) == 0 {
		t.Fatalf("manual code=%d body=%v", code, body)
	}

	code, body = zeppGet(t, srv.URL+"/v1/user/manualData.json?userid=u1&type=other", "tok")
	if items, _ := body["items"].([]any); code != http.StatusOK || len(items) != 0 {
		t.Fatalf("manual other code=%d body=%v", code, body)
	}

	if code, _ := zeppGet(t, srv.URL+"/users/me/bloodPressure?from=1&to=2", "tok"); code != http.StatusBadRequest {
		t.Fatalf("bp earlier form code = %d, want 400", code)
	}

	code, body = zeppGet(t, srv.URL+"/users/me/bloodPressure?days=2&toDate=2026-08-11&sourceArrayStr=a,b", "tok")
	if items, _ := body["items"].([]any); code != http.StatusOK || len(items) != 2 {
		t.Fatalf("bp code=%d body=%v", code, body)
	}
}

func TestZeppMockWorkoutDetailNeedsRunSegmentAndSource(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	q := "?trackid=1786348800&userid=u1&source=" + mockWorkoutSource

	if code, _ := zeppGet(t, srv.URL+"/v1/sport/ride/detail.json"+q, "tok"); code != http.StatusNotFound {
		t.Fatalf("ride code = %d, want 404", code)
	}

	_, body := zeppGet(t, srv.URL+"/v1/sport/run/detail.json?trackid=1786348800&userid=u1&source=x", "tok")
	if data, _ := body["data"].(map[string]any); len(data) != 0 {
		t.Fatalf("wrong source data = %v, want empty", data)
	}

	_, body = zeppGet(t, srv.URL+"/v1/sport/run/detail.json"+q, "tok")
	if data, _ := body["data"].(map[string]any); data["trackid"] != "1786348800" {
		t.Fatalf("detail data = %v", data)
	}

	_, body = zeppGet(t, srv.URL+"/v1/sport/run/history.json", "tok")
	data, _ := body["data"].(map[string]any)
	summary, _ := data["summary"].([]any)
	first, _ := summary[0].(map[string]any)
	if first["source"] != mockWorkoutSource {
		t.Fatalf("history item %v lacks the source", first)
	}
}

func TestZeppMockServesTheFullReadSurface(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ms := "&from=1786320000000&to=1786492799000"

	presets := [][2]string{
		{"hrv_sdnn", "real_data"}, {"Charge", "real_data"}, {"Charge", "insight_data"},
		{"Charge", "stress_data"}, {"RespiratoryRate", "real_data"}, {"DailyHealth", "summary"},
		{"blood_pressure", "real_data"}, {"Emotion", "real_data"}, {"skinTemp", "real_data"},
	}
	for _, p := range presets {
		code, body := zeppGet(t, srv.URL+mockZeppEventsPath+"?eventType="+p[0]+"&subType="+p[1]+"&limit=1000"+ms, "tok")
		if items, _ := body["items"].([]any); code != http.StatusOK || len(items) != 2 {
			t.Fatalf("%s/%s code=%d body=%v", p[0], p[1], code, body)
		}
	}

	paths := []string{
		"/huami.health.getUserInfo.json",
		"/users/me/fileInfo/events?eventType=second_heart_rate" + ms,
		"/users/u1/events/dateString?from=2026-08-10T00:00:00Z&to=2026-08-10T23:59:59Z",
		"/users/u1/heartRate?startTime=1786320000&endTime=1786406399",
		"/users/u1/members/-1/weightRecords?fromTime=1786320000&toTime=1786406399",
	}
	for _, p := range paths {
		if code, body := zeppGet(t, srv.URL+p, "tok"); code != http.StatusOK || body == nil {
			t.Fatalf("%s code=%d body=%v", p, code, body)
		}
	}

	bad := []string{
		"/users/me/fileInfo/events?from=x",
		"/users/u1/events/dateString?from=x",
		"/users/u1/heartRate?startTime=x",
		"/users/u1/members/-1/weightRecords?fromTime=x",
		"/users/u1/events?eventType=all_day_stress&from=x&to=y&userId=u1&limit=200",
		"/users/u1/events?eventType=all_day_stress&from=1&to=2&userId=u1&limit=9999",
		"/users/me/bloodPressure?days=0&toDate=2026-08-11&sourceArrayStr=a",
	}
	for _, p := range bad {
		if code, _ := zeppGet(t, srv.URL+p, "tok"); code < http.StatusBadRequest {
			t.Fatalf("%s code=%d, want an error", p, code)
		}
	}

	if code, _ := zeppGet(t, srv.URL+"/v1/sport/run/detail.json", "tok"); code != http.StatusBadRequest {
		t.Fatalf("detail without trackid code=%d", code)
	}
}
