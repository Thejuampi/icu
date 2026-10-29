package main

import (
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"net/http"
	"path"
	"strconv"
	"time"
)

// Synthetic Zepp mock for `icu wellness sync --zepp-base-url`. Every value
// is generated from the date (no fixtures, no real account data). It serves
// the two read endpoints the sync uses on one host:
//
//	GET /v1/data/band_data.json  (sleep + resting HR in summary.slp)
//	GET /v2/users/me/events      (eventType=readiness&subType=watch_score:
//	                              value.sleepHRV nightly HRV; eventType=HRVRMSSD:
//	                              value.samples[{hrv,s}] per-minute rMSSD)
//
// The event shapes mirror the real ones documented by open-source clients
// (anonymized: no real ids). A sample of 0 and the 255 sentinel are
// included because Zepp uses them for failed reads. Days where
// mockZeppNoReadiness is true have no readiness item, so the sync falls
// back to the HRVRMSSD sample mean (which equals the same HRV value).
//
// The Apptoken header selects a scenario: mockZeppTokenExpired answers 401
// on both, mockZeppTokenEventsDown answers 503 on events only, a missing
// token answers 401. Days before mockZeppFirstDay have no data, so a range
// there exercises the empty-range path.
const (
	mockZeppBandPath        = "/v1/data/band_data.json"
	mockZeppEventsPath      = "/v2/users/me/events"
	mockZeppTokenExpired    = "mock-expired"
	mockZeppTokenEventsDown = "mock-events-down"
	mockZeppFirstDay        = "2026-01-01"
	// mockZeppHRVHourUTC puts each night's HRV reading at 03:30 UTC of the
	// day, i.e. the previous local evening in the Americas: it exercises the
	// evening-rolls-forward day mapping.
	mockZeppHRVHourUTC = 3
	mockZeppHRVMinute  = 30
)

func isZeppPath(p string) bool {
	return p == mockZeppBandPath || p == mockZeppEventsPath
}

// MockZeppDay is the synthetic Zepp wellness for one date, exported for
// tests and documented in the RC.
type MockZeppDay struct {
	RestingHR    int
	SleepScore   int
	DeepMinutes  int
	LightMinutes int
	HRV          float64
	NoReadiness  bool
}

// mockZeppDayValues derives deterministic synthetic values from the date.
//
//nolint:mnd // synthetic ranges, documented by the field names
func mockZeppDayValues(date string) MockZeppDay {
	h := fnv.New32a()
	_, _ = h.Write([]byte(date))
	n := int(h.Sum32() % 1000)

	return MockZeppDay{
		RestingHR:    46 + n%9,
		SleepScore:   62 + n%30,
		DeepMinutes:  60 + n%50,
		LightMinutes: 240 + n%90,
		HRV:          float64(35 + n%25),
		NoReadiness:  n%7 == 0,
	}
}

func (s *server) handleZepp(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Apptoken")
	switch {
	case token == "" || token == mockZeppTokenExpired:
		writeError(w, http.StatusUnauthorized, "invalid apptoken")
	case r.URL.Path == mockZeppBandPath:
		s.zeppBand(w, r)
	case token == mockZeppTokenEventsDown:
		writeError(w, http.StatusServiceUnavailable, "events unavailable")
	default:
		s.zeppEvents(w, r)
	}
}

func mockZeppDays(from, to time.Time) []string {
	var days []string
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if day := d.Format(time.DateOnly); day >= mockZeppFirstDay {
			days = append(days, day)
		}
	}

	return days
}

func (s *server) zeppBand(w http.ResponseWriter, r *http.Request) {
	from, errFrom := time.Parse(time.DateOnly, r.URL.Query().Get("from_date"))
	to, errTo := time.Parse(time.DateOnly, r.URL.Query().Get("to_date"))
	if errFrom != nil || errTo != nil {
		writeError(w, http.StatusBadRequest, "from_date/to_date must be YYYY-MM-DD")

		return
	}

	items := []map[string]any{}
	for _, day := range mockZeppDays(from, to) {
		v := mockZeppDayValues(day)
		start, _ := time.Parse(time.DateOnly, day)
		summary, _ := json.Marshal(map[string]any{ //nolint:errchkjson // plain map
			"slp": map[string]any{
				"st": start.Add(-2 * time.Hour).Unix(), "ed": start.Add(6 * time.Hour).Unix(),
				"dp": v.DeepMinutes, "lt": v.LightMinutes, "rhr": v.RestingHR, "ss": v.SleepScore,
			},
		})
		items = append(items, map[string]any{
			"uid": "mock-user", "data_type": 0, "date_time": day,
			"summary": base64.StdEncoding.EncodeToString(summary),
		})
	}

	body, _ := json.Marshal(map[string]any{"code": 1, "message": "success", "data": items}) //nolint:errchkjson // plain map
	writeJSON(w, http.StatusOK, body)
}

func (s *server) zeppEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// A fixture at <fixtures>/_zepp/events/<eventType>/<subType>.json is
	// served as-is (for raw-output checks with synthetic payloads).
	if body, ok := s.fixtures[path.Join("/_zepp/events", path.Base("/"+q.Get("eventType")), path.Base("/"+q.Get("subType")))]; ok {
		writeJSON(w, http.StatusOK, body)

		return
	}
	items := []map[string]any{}
	et, st := q.Get("eventType"), q.Get("subType")
	if et == "HRVRMSSD" || (et == "readiness" && st == "watch_score") {
		fromMS, errFrom := strconv.ParseInt(q.Get("from"), 10, 64)
		toMS, errTo := strconv.ParseInt(q.Get("to"), 10, 64)
		if errFrom != nil || errTo != nil {
			writeError(w, http.StatusBadRequest, "from/to must be epoch millis")

			return
		}
		from := time.UnixMilli(fromMS).UTC().Truncate(24 * time.Hour)
		to := time.UnixMilli(toMS).UTC()
		for _, day := range mockZeppDays(from, to) {
			at, _ := time.Parse(time.DateOnly, day)
			v := mockZeppDayValues(day)
			// Sleep window starts the previous UTC evening (03:30 UTC is
			// the previous local evening in the Americas: exercises the
			// evening-rolls-forward day mapping).
			start := at.Add(mockZeppHRVHourUTC*time.Hour + mockZeppHRVMinute*time.Minute).UnixMilli()
			if start < fromMS || start > toMS {
				continue
			}
			if et == "HRVRMSSD" {
				items = append(items, mockHRVRMSSDItem(start, v.HRV))
			} else if !v.NoReadiness {
				items = append(items, mockReadinessItem(start, v.HRV))
			}
		}
	}

	body, _ := json.Marshal(map[string]any{"items": items}) //nolint:errchkjson // plain map
	writeJSON(w, http.StatusOK, body)
}

// applyWellnessBulk merges a PUT wellness-bulk body (array of records keyed
// by id = date) into the in-memory wellness list, so a later GET wellness
// sees the write, like intervals.icu. Returns the list of updated records.
func (s *server) applyWellnessBulk(bulkKey string, raw []byte) []byte {
	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err != nil {
		return []byte(`[]`)
	}

	listKey := bulkKey[:len(bulkKey)-len("-bulk")]
	current := s.overlay[listKey]
	if current == nil {
		current = s.fixtures[listKey]
	}

	var list []map[string]any
	_ = json.Unmarshal(current, &list)
	index := map[string]int{}
	for i, item := range list {
		if id, ok := item["id"].(string); ok {
			index[id] = i
		}
	}

	for _, rec := range records {
		id, _ := rec["id"].(string)
		i, ok := index[id]
		if !ok {
			list = append(list, map[string]any{"id": id})
			i = len(list) - 1
			index[id] = i
		}
		for k, v := range rec {
			list[i][k] = v
		}
	}

	if merged, err := json.Marshal(list); err == nil {
		s.overlay[listKey] = merged
	}
	out, _ := json.Marshal(records) //nolint:errchkjson // decoded JSON

	return out
}

// mockHRVRMSSDItem is one night of per-minute rMSSD in the real shape:
// value is an object with startTime (ms) and samples [{hrv, s}] where s is
// a millisecond offset. The 0 and 255 samples are failed reads; the valid
// samples average to hrv.
const (
	mockSampleStepMs = 60000 // one sample per minute, offsets in ms
	mockHRVNone      = 255   // Zepp's "no reading" sentinel
	// mockSleepHRVOffset: readiness sleepHRV = samples mean + 17 ms.
	mockSleepHRVOffset = 17
)

func mockHRVRMSSDItem(start int64, hrv float64) map[string]any {
	return map[string]any{
		"timestamp": start, "eventType": "HRVRMSSD", "subType": "real_data", "userId": "mock-user",
		"value": map[string]any{
			"startTime": start, "deviceId": "MOCKDEVICE0000", "deviceSource": 1,
			"samples": []map[string]any{
				{"hrv": hrv - 3, "s": 0},
				{"hrv": 0, "s": 1 * mockSampleStepMs},
				{"hrv": hrv, "s": 2 * mockSampleStepMs},
				{"hrv": mockHRVNone, "s": 3 * mockSampleStepMs},
				{"hrv": hrv + 3, "s": 4 * mockSampleStepMs},
			},
		},
	}
}

// mockReadinessItem is one readiness watch_score item in the real shape
// (value object; sleepHRV is the nightly rMSSD; 255 means not measured).
//
//nolint:mnd // synthetic scores
func mockReadinessItem(start int64, hrv float64) map[string]any {
	// sleepHRV differs from the samples mean (hrv) on purpose, so a test can
	// tell which source a sync used.
	sleepHRV := hrv + mockSleepHRVOffset
	morning := start + 4*int64(time.Hour/time.Millisecond)

	return map[string]any{
		"timestamp": morning, "eventType": "readiness", "subType": "watch_score", "userId": "mock-user",
		"value": map[string]any{
			"sleepHRV": sleepHRV, "hrvBaseline": 40, "hrvScore": 66, "sleepRHR": 52, "rhrBaseline": 50,
			"rdnsScore": 70, "afibScore": 255, "skinTempCalibrated": -10, "skinTempBaseLine": 2,
			"skinTempScore": 80, "deviceId": "MOCKDEVICE0000", "status": 200,
			"timestampUpdate": morning,
		},
	}
}
