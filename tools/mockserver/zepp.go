package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"time"
)

// Synthetic Zepp mock for `icu wellness sync --zepp-base-url`. Every value
// is generated from the date (no fixtures, no real account data). It serves
// the two read endpoints the sync uses on one host:
//
//	GET /v1/data/band_data.json  (sleep + resting HR in summary.slp)
//	GET /v2/users/me/events      (eventType=HRVRMSSD nightly HRV)
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

func isZeppPath(path string) bool {
	return path == mockZeppBandPath || path == mockZeppEventsPath
}

// MockZeppDay is the synthetic Zepp wellness for one date, exported for
// tests and documented in the RC.
type MockZeppDay struct {
	RestingHR    int
	SleepScore   int
	DeepMinutes  int
	LightMinutes int
	HRV          float64
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
		HRV:          float64(35+n%25) + 0.5,
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
	items := []map[string]any{}
	if q.Get("eventType") == "HRVRMSSD" {
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
			ts := at.Add(mockZeppHRVHourUTC*time.Hour + mockZeppHRVMinute*time.Minute).UnixMilli()
			if ts < fromMS || ts > toMS {
				continue
			}
			items = append(items, map[string]any{
				"timestamp": ts, "eventType": "HRVRMSSD", "subType": "real_data",
				"value": fmt.Sprintf("%.1f", mockZeppDayValues(day).HRV),
			})
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
