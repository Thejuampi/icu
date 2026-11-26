package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Synthetic Zepp mock for `icu wellness sync --zepp-base-url` and for
// exercising every Zepp series the CLI supports. Every value is generated
// from the date (no fixtures, no real account data). The mock serves:
//
//	GET /huami.health.getUserInfo.json      (profile)
//	GET /v1/data/band_data.json             (steps, sleep stages, resting HR, data_hr)
//	GET /v2/users/me/events                 (all V2 presets, synthetic)
//	GET /users/{id}/events                  (legacy stress/spo2/PAI, synthetic)
//	GET /v2/watch/users/{id}/WatchSportStatistics/{SPORT_LOAD|VO2_MAX}
//	GET /users/{id}/heartRate               (app-source heart rate)
//	GET /users/{id}/members/-1/weightRecords
//	GET /users/me/bloodPressure             (user blood pressure)
//	GET /v1/user/manualData.json            (manual wellness records)
//	GET /users/me/fileInfo/events           (second-heart-rate file index)
//	GET /users/{id}/events/dateString       (SpO2 ODI windows)
//	GET /v1/sport/{sport}/history.json      (workout summaries)
//	GET /v1/sport/{sport}/detail.json       (workout detail)
//
// The event shapes mirror the real ones documented by open-source clients
// (anonymized: no real ids). A sample of 0 and the 255 sentinel are
// included because Zepp uses them for failed reads. Days where
// mockZeppNoReadiness is true have no readiness item, so the sync falls
// back to the HRVRMSSD sample mean (which equals the same HRV value).
//
// The Apptoken header selects a scenario: mockZeppTokenExpired answers 401
// on every Zepp path, mockZeppTokenEventsDown answers 503 on every path
// except band_data, a missing token answers 401. Days before mockZeppFirstDay
// have no data, so a range there exercises the empty-range path.
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
	// mockSleepNightStartMin/MockSleepNightEndMin/MockSleepNightMinutes define
	// the synthetic night (22:00 -> 06:00 UTC, 480 minutes) used by the sleep
	// stage sequence and the summary st/ed.
	mockSleepNightStartMin = 1320
	mockSleepNightMinutes  = 480
	// JSON keys and synthetic values repeated across the mock handlers.
	mockUserID       = "mock-user"
	mockKeyMessage   = "message"
	mockSuccess      = "success"
	mockKeyItems     = "items"
	mockKeyValue     = "value"
	mockKeySubType   = "subType"
	mockSubRealData  = "real_data"
	mockChargeType   = "Charge"
	mockDeviceID     = "MOCKDEVICE0000"
	mockDeviceSN     = "MOCKSN000000"
	mockWatchType    = "watch"
	mockTimeZone     = "-5"
	mockSamplesKey   = "samples"
	mockDeviceSrcKey = "deviceSource"
	mockDeviceIdKey  = "deviceId"
	mockStartTimeKey = "startTime"
	mockKeyTimestamp = "timestamp"
	mockKeyUserID    = "userId"
)

func isZeppPath(p string) bool {
	switch p {
	case mockZeppBandPath, mockZeppEventsPath,
		"/huami.health.getUserInfo.json",
		"/v1/user/manualData.json",
		"/users/me/bloodPressure",
		"/users/me/fileInfo/events":
		return true
	}

	switch {
	case strings.HasSuffix(p, "/events/dateString"),
		strings.HasSuffix(p, "/heartRate"),
		strings.HasSuffix(p, "/weightRecords"):
		return true
	case strings.HasSuffix(p, "/events") && strings.HasPrefix(p, "/users/"):
		return true
	case strings.HasPrefix(p, "/v2/watch/users/") && strings.Contains(p, "/WatchSportStatistics/"):
		return true
	case strings.HasPrefix(p, "/v1/sport/") && (strings.HasSuffix(p, "/history.json") || strings.HasSuffix(p, "/detail.json")):
		return true
	}

	return false
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

// mockSleepStage is one synthetic sleep stage (minutes since midnight on a
// 0..1439 clock; stop < start means the stage crosses midnight).
type mockSleepStage struct {
	Start int `json:"start"`
	Stop  int `json:"stop"`
	Mode  int `json:"mode"`
}

// mockSleepStageMinutes computes a stage's length in minutes, matching the
// cross-midnight semantics the client uses (stop < start means the stage
// runs past midnight).
//
//nolint:mnd // minutes per day
func mockSleepStageMinutes(s mockSleepStage) int {
	switch {
	case s.Stop > s.Start:
		return s.Stop - s.Start
	case s.Stop < s.Start:
		return s.Stop + 1440 - s.Start
	default:
		return 0
	}
}

// mockSleepStages builds a synthetic stage sequence for the night (22:00 to
// 06:00 UTC, 480 minutes). dp/lt in the summary are derived from the stages
// so the mock stays internally consistent. Stage lengths vary per day via
// the synthetic day values.
//
//nolint:mnd // synthetic stage lengths and minutes-per-day
func mockSleepStages(v MockZeppDay) []mockSleepStage {
	deep := 60 + v.DeepMinutes%50     // 60..109
	light := 180 + v.LightMinutes%100 // 180..269
	awake := 15
	rem := 45
	light2 := mockSleepNightMinutes - deep - light - awake - rem
	if light2 < 0 {
		light2 = 0
	}
	start := mockSleepNightStartMin
	stages := []mockSleepStage{}
	add := func(mode, minutes int) {
		if minutes <= 0 {
			return
		}
		stop := (start + minutes) % 1440
		stages = append(stages, mockSleepStage{Start: start, Stop: stop, Mode: mode})
		start = stop
	}
	add(5, deep)   // deep
	add(4, light)  // light (may cross midnight)
	add(7, awake)  // awake
	add(8, rem)    // REM
	add(4, light2) // light tail
	return stages
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
	case r.URL.Path == mockZeppEventsPath:
		s.zeppEvents(w, r)
	case r.URL.Path == "/huami.health.getUserInfo.json":
		s.zeppUserInfo(w)
	case r.URL.Path == "/v1/user/manualData.json":
		s.zeppManualData(w, r)
	case r.URL.Path == "/users/me/bloodPressure":
		s.zeppBloodPressureUser(w, r)
	case r.URL.Path == "/users/me/fileInfo/events":
		s.zeppSecondHeartRate(w, r)
	case strings.HasSuffix(r.URL.Path, "/events/dateString"):
		s.zeppSpO2Windows(w, r)
	case strings.HasSuffix(r.URL.Path, "/events") && strings.HasPrefix(r.URL.Path, "/users/"):
		s.zeppOldEvents(w, r)
	case strings.HasSuffix(r.URL.Path, "/heartRate"):
		s.zeppHeartRateApp(w, r)
	case strings.HasSuffix(r.URL.Path, "/weightRecords"):
		s.zeppWeightRecords(w, r)
	case strings.HasPrefix(r.URL.Path, "/v2/watch/users/"):
		s.zeppSportStatistics(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/sport/"):
		s.zeppSport(w, r)
	default:
		writeError(w, http.StatusNotFound, "no fixture for "+r.URL.Path)
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

// mockZeppHeartRateDay builds a synthetic `data_hr` blob in the real wire
// shape: 1440 bytes, one unsigned byte per minute. The first six hours are
// 254 ("no read"), after that a reading every third minute with 255 ("not
// required") in between. Readings are resting + a small deterministic wave.
func mockZeppHeartRateDay(resting int) []byte {
	const (
		minutesPerDay = 1440
		noReadMinutes = 360
	)

	blob := make([]byte, minutesPerDay)
	for i := range blob {
		switch {
		case i < noReadMinutes:
			blob[i] = 254
		case i%3 == 0:
			blob[i] = byte(resting + i%25) //nolint:gosec // resting is 46..54, so the value stays below 254
		default:
			blob[i] = 255
		}
	}

	return blob
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
		stages := mockSleepStages(v)
		deep := 0
		light := 0
		for _, s := range stages {
			minutes := mockSleepStageMinutes(s)
			switch s.Mode {
			case 5:
				deep += minutes
			case 4:
				light += minutes
			}
		}
		summary, _ := json.Marshal(map[string]any{ //nolint:errchkjson // plain map
			"slp": map[string]any{
				// st/ed span the synthetic night (22:00 -> 06:00), consistent
				// with the stage minutes (relative to the day's midnight).
				"st": start.Add(time.Duration(mockSleepNightStartMin) * time.Minute).Unix(),
				"ed": start.Add(time.Duration(mockSleepNightStartMin+mockSleepNightMinutes) * time.Minute).Unix(),
				"dp": deep, "lt": light, "rhr": v.RestingHR, "ss": v.SleepScore,
				"stage": stages,
			},
		})
		items = append(items, map[string]any{
			"uid": mockUserID, "data_type": 0, "date_time": day,
			"summary": base64.StdEncoding.EncodeToString(summary),
			"data_hr": base64.StdEncoding.EncodeToString(mockZeppHeartRateDay(v.RestingHR)),
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{"code": 1, mockKeyMessage: mockSuccess, "data": items}))
}

func (s *server) zeppEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// A fixture at <fixtures>/_zepp/events/<eventType>/<subType>.json is
	// served as-is (for raw-output checks with synthetic payloads).
	if body, ok := s.fixtures[path.Join("/_zepp/events", path.Base("/"+q.Get("eventType")), path.Base("/"+q.Get(mockKeySubType)))]; ok {
		writeJSON(w, http.StatusOK, body)

		return
	}
	items := []map[string]any{}
	et, st := q.Get("eventType"), q.Get(mockKeySubType)
	if et == "" {
		writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))

		return
	}

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

		if item := mockZeppEventItem(et, st, start, v); item != nil {
			items = append(items, item)
		}
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

func mustMarshal(v map[string]any) []byte {
	body, _ := json.Marshal(v) //nolint:errchkjson // plain map

	return body
}

// mockZeppEventItem builds one synthetic item for a known preset. Unknown
// (eventType, subType) pairs return nil: the mock serves nothing, which is
// honest (the CLI only supports the registered presets).
//
//nolint:mnd // synthetic event values
func mockZeppEventItem(et, st string, start int64, v MockZeppDay) map[string]any {
	switch {
	case et == "readiness" && st == "watch_score":
		if v.NoReadiness {
			return nil
		}

		return mockReadinessItem(start, v.HRV)
	case et == "HRVRMSSD" && st == mockSubRealData:
		return mockHRVRMSSDItem(start, v.HRV)
	case et == "hrv_sdnn" && st == mockSubRealData:
		return mockSDNNItem(start, v.HRV)
	case et == mockChargeType && st == mockSubRealData:
		// body-battery (real object shape)
		return mockBodyBatteryItem(start, v)
	case et == mockChargeType && st == "insight_data":
		// hybridcharge / biocharge (real object shape)
		return mockHybridChargeItem(start, v)
	case et == mockChargeType && st == "stress_data":
		// per-minute stress (real object shape)
		return mockStressMinuteItem(start, v)
	case et == "RespiratoryRate" && st == mockSubRealData:
		// respiratory rate (real object shape)
		return mockRespiratoryRateItem(start)
	case et == "DailyHealth" && st == "summary":
		// health summary (real object shape)
		return mockHealthSummaryItem(start, v)
	case et == "blood_pressure" && st == mockSubRealData:
		return map[string]any{
			mockKeyTimestamp: start, "type": et, mockKeySubType: st, mockKeyUserID: mockUserID,
			"systolic":  115 + v.RestingHR%15,
			"diastolic": 70 + v.SleepScore%15,
		}
	case et == "Emotion" && st == mockSubRealData:
		return mockScalarItem(et, st, start, float64(40+v.DeepMinutes%40))
	case et == "skinTemp" && st == mockSubRealData:
		return mockScalarItem(et, st, start, float64(-3+v.LightMinutes%7))
	}

	return nil
}

func mockScalarItem(et, st string, start int64, value float64) map[string]any {
	return map[string]any{
		mockKeyTimestamp: start, "type": et, mockKeySubType: st, mockKeyUserID: mockUserID,
		mockKeyValue: value,
	}
}

// mockDeviceMetadata returns the device-metadata object shared by the real
// object-shaped builders: startTime, device identifiers and timeZone.
func mockDeviceMetadata(start int64) map[string]any {
	return map[string]any{
		mockStartTimeKey: start, mockDeviceIdKey: mockDeviceID, "deviceSN": mockDeviceSN,
		mockDeviceSrcKey: 1, "deviceType": mockWatchType, "timeZone": mockTimeZone,
	}
}

// mockBodyBatteryItem is one body-battery item in the real shape (value is
// an object with startTime, device metadata and samples [{s, total,
// mental, physical, status, jsonExtra}]).
//
//nolint:mnd // synthetic values
func mockBodyBatteryItem(start int64, v MockZeppDay) map[string]any {
	value := mockDeviceMetadata(start)
	value[mockSamplesKey] = []map[string]any{
		{"s": 0, "total": 55 + v.RestingHR%40, "mental": 40 + v.LightMinutes%30, "physical": 60 + v.SleepScore%30, "status": 0, "jsonExtra": "{}"},
	}

	return map[string]any{
		mockKeyTimestamp: start, "type": mockChargeType, mockKeySubType: mockSubRealData, mockKeyUserID: mockUserID,
		mockKeyValue: value,
	}
}

// mockHybridChargeItem is one hybridcharge/biocharge item in the real shape
// (value is an object with startTime and samples [{s, u, type, insight,
// insightId, diff, slope, thres, trackId, jsonExtra}]).
//
//nolint:mnd // synthetic values
func mockHybridChargeItem(start int64, v MockZeppDay) map[string]any {
	value := mockDeviceMetadata(start)
	value[mockSamplesKey] = []map[string]any{
		{"s": 0, "u": 12345, "type": 4, "insight": 70 + v.SleepScore%30, "insightId": "MOCKINSIGHT000", "diff": 0, "slope": 0.0, "thres": 0.0, "trackId": "MOCKTRACK00000000", "jsonExtra": "{}"},
	}

	return map[string]any{
		mockKeyTimestamp: start, "type": mockChargeType, mockKeySubType: "insight_data", mockKeyUserID: mockUserID,
		mockKeyValue: value,
	}
}

// mockStressMinuteItem is one stress-minute item in the real shape (value
// is an object with startTime and samples [{s, e?, u, stressInfo, minuteOfDay}]).
//
//nolint:mnd // synthetic values
func mockStressMinuteItem(start int64, v MockZeppDay) map[string]any {
	value := mockDeviceMetadata(start)
	value[mockSamplesKey] = []map[string]any{
		{"s": 0, "u": 1000 + v.RestingHR%100, "stressInfo": "mock", "minuteOfDay": 400 + v.DeepMinutes%300},
		{"s": 0, "e": 60000000 + v.LightMinutes*1000, "u": 1001 + v.RestingHR%100, "stressInfo": "mock", "minuteOfDay": 500 + v.DeepMinutes%300},
	}

	return map[string]any{
		mockKeyTimestamp: start, "type": mockChargeType, mockKeySubType: "stress_data", mockKeyUserID: mockUserID,
		mockKeyValue: value,
	}
}

// mockRespiratoryRateItem is one respiratory-rate item in the real shape
// (value is an object with measurements and timeZone).
func mockRespiratoryRateItem(start int64) map[string]any {
	return map[string]any{
		mockKeyTimestamp: start, "type": "RespiratoryRate", mockKeySubType: mockSubRealData, mockKeyUserID: mockUserID,
		mockKeyValue: map[string]any{
			"measurements": "bW9jaw==", "timeZone": mockTimeZone,
		},
	}
}

// mockHealthSummaryItem is one health-summary item in the real shape (value
// is an object with startTime and samples [{s, u, dateString, totalSteps,
// totalCalories, totalBurningDuration, stepGoal, calorieGoal,
// burningDurationGoal, hyroxCount}]).
//
//nolint:mnd // synthetic values
func mockHealthSummaryItem(start int64, v MockZeppDay) map[string]any {
	value := mockDeviceMetadata(start)
	value[mockSamplesKey] = []map[string]any{
		{
			"s": 0, "u": 12345, "dateString": time.UnixMilli(start).UTC().Format(time.DateOnly),
			"totalSteps": 4000 + v.SleepScore*10, "totalCalories": 400 + v.LightMinutes%50, "totalBurningDuration": 60 + v.DeepMinutes%30,
			"stepGoal": 10000, "calorieGoal": 400, "burningDurationGoal": 45, "hyroxCount": 0,
		},
	}

	return map[string]any{
		mockKeyTimestamp: start, "type": "DailyHealth", mockKeySubType: "summary", mockKeyUserID: mockUserID,
		mockKeyValue: value,
	}
}

// mockSDNNItem is one night of per-minute SDNN in the real shape (value is
// an object with startTime and samples).
func mockSDNNItem(start int64, hrv float64) map[string]any {
	return map[string]any{
		mockKeyTimestamp: start, "eventType": "hrv_sdnn", mockKeySubType: mockSubRealData, mockKeyUserID: mockUserID,
		mockKeyValue: map[string]any{
			mockStartTimeKey: start, mockDeviceIdKey: mockDeviceID, "deviceSource": 1,
			mockSamplesKey: []map[string]any{
				{"sdnn": hrv - 3, "s": 0},
				{"sdnn": 0, "s": 1 * mockSampleStepMs},
				{"sdnn": hrv, "s": 2 * mockSampleStepMs},
				{"sdnn": mockHRVNone, "s": 3 * mockSampleStepMs},
				{"sdnn": hrv + 3, "s": 4 * mockSampleStepMs},
			},
		},
	}
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
		mockKeyTimestamp: start, "eventType": "HRVRMSSD", mockKeySubType: mockSubRealData, mockKeyUserID: mockUserID,
		mockKeyValue: map[string]any{
			mockStartTimeKey: start, mockDeviceIdKey: mockDeviceID, "deviceSource": 1,
			mockSamplesKey: []map[string]any{
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
// (value object; sleepHRV is Zepp nightly HRV; 255 means not measured).
//
//nolint:mnd // synthetic scores
func mockReadinessItem(start int64, hrv float64) map[string]any {
	// sleepHRV differs from the samples mean (hrv) on purpose, so a test can
	// tell which source a sync used.
	sleepHRV := hrv + mockSleepHRVOffset
	morning := start + 4*int64(time.Hour/time.Millisecond)

	return map[string]any{
		mockKeyTimestamp: morning, "eventType": "readiness", mockKeySubType: "watch_score", mockKeyUserID: mockUserID,
		mockKeyValue: map[string]any{
			"sleepHRV": sleepHRV, "hrvBaseline": 40, "hrvScore": 66, "sleepRHR": 52, "rhrBaseline": 50,
			"rdnsScore": 70, "afibScore": 255, "skinTempCalibrated": -10, "skinTempBaseLine": 2,
			"skinTempScore": 80, mockDeviceIdKey: mockDeviceID, "status": 200,
			"timestampUpdate": morning,
		},
	}
}

// mockZeppDayList parses epoch-millis from/to and returns the mock days in
// range (inclusive). ok is false when the params are missing or invalid.
func mockZeppDayList(q url.Values, fromKey, toKey string) ([]string, bool) {
	fromMS, errFrom := strconv.ParseInt(q.Get(fromKey), 10, 64)
	toMS, errTo := strconv.ParseInt(q.Get(toKey), 10, 64)
	if errFrom != nil || errTo != nil {
		return nil, false
	}

	from := time.UnixMilli(fromMS).UTC().Truncate(24 * time.Hour)
	to := time.UnixMilli(toMS).UTC()

	return mockZeppDays(from, to), true
}

// zeppUserInfo serves /huami.health.getUserInfo.json in the real shape.
//
//nolint:mnd // synthetic profile values
func (s *server) zeppUserInfo(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{
		"code": 1, mockKeyMessage: mockSuccess,
		"data": map[string]any{
			mockKeyUserID: mockUserID,
			"nickname":    "Mock User",
			"email":       "",
			"gender":      1,
			"height":      178,
			"weight":      74,
			"birthday":    "1985-06-15",
			"region":      "US",
		},
	}))
}

// zeppManualData serves /v1/user/manualData.json: one synthetic entry per day.
func (s *server) zeppManualData(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days, ok := mockZeppDayList(q, "from", "to")
	if !ok {
		writeError(w, http.StatusBadRequest, "from/to must be epoch millis")

		return
	}

	items := []map[string]any{}
	for _, day := range days {
		at, _ := time.Parse(time.DateOnly, day)
		items = append(items, map[string]any{
			mockKeyTimestamp: at.Add(12 * time.Hour).UnixMilli(),
			"type":           "manual",
			mockKeyValue:     map[string]any{"note": "mock"},
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{"code": 1, mockKeyMessage: mockSuccess, mockKeyItems: items}))
}

// zeppBloodPressureUser serves /users/me/bloodPressure: one synthetic entry per day.
//
//nolint:mnd // synthetic blood-pressure values
func (s *server) zeppBloodPressureUser(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days, ok := mockZeppDayList(q, "from", "to")
	if !ok {
		writeError(w, http.StatusBadRequest, "from/to must be epoch millis")

		return
	}

	items := []map[string]any{}
	for _, day := range days {
		at, _ := time.Parse(time.DateOnly, day)
		v := mockZeppDayValues(day)
		items = append(items, map[string]any{
			mockKeyTimestamp: at.Add(8 * time.Hour).UnixMilli(),
			"systolic":       110 + v.RestingHR%20,
			"diastolic":      68 + v.SleepScore%20,
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

// zeppSecondHeartRate serves /users/me/fileInfo/events: one synthetic COS
// file index entry per day. The URLs are synthetic, not real COS links.
//
//nolint:mnd // synthetic blob size
func (s *server) zeppSecondHeartRate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days, ok := mockZeppDayList(q, "from", "to")
	if !ok {
		writeError(w, http.StatusBadRequest, "from/to must be epoch millis")

		return
	}

	items := []map[string]any{}
	for _, day := range days {
		at, _ := time.Parse(time.DateOnly, day)
		items = append(items, map[string]any{
			mockKeyTimestamp: at.Add(mockZeppHRVHourUTC*time.Hour + mockZeppHRVMinute*time.Minute).UnixMilli(),
			"url":            fmt.Sprintf("https://cos.mock.local/mock/%s.zip", day),
			"size":           144000,
			"type":           "second_heart_rate",
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

// zeppSpO2Windows serves /users/{id}/events/dateString: synthetic ODI windows.
//
//nolint:mnd // synthetic SpO2 value
func (s *server) zeppSpO2Windows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	at, err := time.Parse(time.RFC3339, q.Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "from must be an ISO-8601 datetime")

		return
	}

	items := []map[string]any{
		{
			mockKeyTimestamp: at.UnixMilli(),
			"startTime":      at.Add(2 * time.Hour).UnixMilli(),
			"endTime":        at.Add(2*time.Hour + 30*time.Minute).UnixMilli(),
			"type":           "blood_oxygen",
			mockKeySubType:   "odi",
			mockKeyValue:     92,
		},
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

// zeppHeartRateApp serves /users/{id}/heartRate (app-source heart rate).
func (s *server) zeppHeartRateApp(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fromSec, errFrom := strconv.ParseInt(q.Get("startTime"), 10, 64)
	_, errTo := strconv.ParseInt(q.Get("endTime"), 10, 64)
	if errFrom != nil || errTo != nil {
		writeError(w, http.StatusBadRequest, "startTime/endTime must be epoch seconds")

		return
	}

	from := time.Unix(fromSec, 0).UTC().Truncate(24 * time.Hour)
	// The real backend serves one flat item per 7-day window (not one item
	// per day): {type, generatedTime (epoch seconds), deviceSource,
	// timeZone, heartRateData (base64), deviceId} — no timestamp/value fields.
	items := []map[string]any{
		{
			"type":           2,
			"generatedTime":  from.Add(mockZeppHRVHourUTC*time.Hour + mockZeppHRVMinute*time.Minute).Unix(),
			mockDeviceSrcKey: 1,
			"timeZone":       mockTimeZone,
			"heartRateData":  "bW9jaw==",
			mockDeviceIdKey:  mockDeviceID,
		},
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

// zeppWeightRecords serves /users/{id}/members/-1/weightRecords.
//
//nolint:mnd // synthetic weight
func (s *server) zeppWeightRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fromSec, errFrom := strconv.ParseInt(q.Get("fromTime"), 10, 64)
	toSec, errTo := strconv.ParseInt(q.Get("toTime"), 10, 64)
	if errFrom != nil || errTo != nil {
		writeError(w, http.StatusBadRequest, "fromTime/toTime must be epoch seconds")

		return
	}

	from := time.Unix(fromSec, 0).UTC().Truncate(24 * time.Hour)
	to := time.Unix(toSec, 0).UTC()
	items := []map[string]any{}
	for _, day := range mockZeppDays(from, to) {
		at, _ := time.Parse(time.DateOnly, day)
		v := mockZeppDayValues(day)
		items = append(items, map[string]any{
			mockKeyTimestamp: at.Add(mockZeppHRVHourUTC*time.Hour + mockZeppHRVMinute*time.Minute).UnixMilli(),
			mockKeyValue:     72.0 + float64(v.RestingHR%10),
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

// zeppSportStatistics serves /v2/watch/users/{id}/WatchSportStatistics/{type}.
//
//nolint:mnd // synthetic sport statistics
func (s *server) zeppSportStatistics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statType := path.Base(r.URL.Path)
	fromMS, errFrom := strconv.ParseInt(q.Get("from"), 10, 64)
	toMS, errTo := strconv.ParseInt(q.Get("to"), 10, 64)
	if errFrom != nil || errTo != nil {
		writeError(w, http.StatusBadRequest, "from/to must be epoch millis")

		return
	}

	from := time.UnixMilli(fromMS).UTC().Truncate(24 * time.Hour)
	to := time.UnixMilli(toMS).UTC()
	items := []map[string]any{}
	for _, day := range mockZeppDays(from, to) {
		at, _ := time.Parse(time.DateOnly, day)
		v := mockZeppDayValues(day)
		var value float64
		switch statType {
		case "SPORT_LOAD":
			value = float64(40 + v.SleepScore%60)
		case "VO2_MAX":
			value = float64(42 + v.RestingHR%12)
		default:
			value = float64(v.RestingHR)
		}

		items = append(items, map[string]any{
			mockKeyTimestamp: at.Add(12 * time.Hour).UnixMilli(),
			"type":           statType,
			mockKeyValue:     value,
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}

// zeppSport serves /v1/sport/{sport}/history.json and /detail.json.
//
//nolint:mnd // synthetic workout summary
func (s *server) zeppSport(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/detail.json") {
		s.zeppSportDetail(w, r)

		return
	}

	q := r.URL.Query()
	next := int64(0)
	if v := q.Get("trackid"); v != "" {
		next, _ = strconv.ParseInt(v, 10, 64)
	}

	// One synthetic summary per mock day, newest first: the real endpoint
	// returns newest-first and the client stops at the first item older
	// than the range. Sparse (one workout every third day) to mirror a
	// realistic cadence.
	items := []map[string]any{}
	for day := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC); !day.Before(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); day = day.AddDate(0, 0, -1) {
		date := day.Format(time.DateOnly)
		if day.Day()%3 != 0 {
			continue
		}
		trackSec := day.Add(mockZeppHRVHourUTC * time.Hour).Unix()
		if trackSec <= next {
			continue
		}

		v := mockZeppDayValues(date)
		items = append(items, map[string]any{
			"trackid":        strconv.FormatInt(trackSec, 10),
			"dis":            strconv.Itoa(5000 + v.DeepMinutes*10),
			"calorie":        strconv.Itoa(300 + v.LightMinutes),
			"end_time":       strconv.FormatInt(trackSec+3600, 10),
			"run_time":       "3600",
			"avg_pace":       "0",
			"avg_heart_rate": strconv.Itoa(v.RestingHR),
			"type":           1,
		})
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{
		"code": 1, mockKeyMessage: mockSuccess,
		"data": map[string]any{
			"next":    -1,
			"summary": items,
		},
	}))
}

// zeppSportDetail serves /v1/sport/{sport}/detail.json.
//
//nolint:mnd // synthetic workout detail
func (s *server) zeppSportDetail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	trackID := q.Get("trackid")
	if trackID == "" {
		writeError(w, http.StatusBadRequest, "trackid is required")

		return
	}

	trackSec, _ := strconv.ParseInt(trackID, 10, 64)
	v := mockZeppDayValues(time.Unix(trackSec, 0).UTC().Format(time.DateOnly))

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{
		"code": 1, mockKeyMessage: mockSuccess,
		"data": map[string]any{
			"trackid":        trackID,
			"type":           1,
			"startTime":      trackSec,
			"endTime":        trackSec + 3600,
			"duration":       3600,
			"distance":       10000,
			"calories":       450,
			"avgHR":          v.RestingHR,
			"maxHR":          v.RestingHR + 20,
			"minHR":          v.RestingHR - 5,
			"avgPower":       180,
			"maxPower":       220,
			"hr_split":       "",
			"pace_split":     "",
			"altitude_split": "",
			"power_split":    "",
			"step_split":     "",
		},
	}))
}

// zeppOldEvents serves /users/{id}/events for the legacy eventTypes the CLI
// supports: all_day_stress (stress), blood_oxygen (spo2) and PaiHealthInfo
// (pai). Each item is synthetic and derived from the date. Unknown
// eventTypes return an empty list.
//
//nolint:mnd // synthetic event values
func (s *server) zeppOldEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	et := q.Get("eventType")
	fromMS, errFrom := strconv.ParseInt(q.Get("from"), 10, 64)
	toMS, errTo := strconv.ParseInt(q.Get("to"), 10, 64)
	if errFrom != nil || errTo != nil {
		writeError(w, http.StatusBadRequest, "from/to must be epoch millis")

		return
	}

	from := time.UnixMilli(fromMS).UTC().Truncate(24 * time.Hour)
	to := time.UnixMilli(toMS).UTC()

	items := []map[string]any{}
	for _, day := range mockZeppDays(from, to) {
		at, _ := time.Parse(time.DateOnly, day)
		v := mockZeppDayValues(day)
		ts := at.Add(12 * time.Hour).UnixMilli()

		switch et {
		case "all_day_stress":
			items = append(items, map[string]any{
				mockKeyTimestamp:   ts,
				"minStress":        15 + v.RestingHR%15,
				"maxStress":        70 + v.SleepScore%30,
				"avgStress":        35 + v.SleepScore%20,
				"relaxProportion":  20 + v.DeepMinutes%30,
				"normalProportion": 30 + v.LightMinutes%20,
				"mediumProportion": 20 + v.RestingHR%20,
				"highProportion":   10 + v.SleepScore%20,
			})
		case "blood_oxygen":
			items = append(items, map[string]any{
				mockKeyTimestamp: ts,
				mockKeySubType:   "odi",
				"odi":            95 + v.RestingHR%5,
				"score":          80 + v.SleepScore%15,
				"extra":          `{"spo2":97,"spo2Decrease":2}`,
			})
		case "PaiHealthInfo":
			items = append(items, map[string]any{
				mockKeyTimestamp:       ts,
				"dailyPai":             40 + v.SleepScore%30,
				"totalPai":             120 + v.DeepMinutes,
				"maxHr":                160 + v.RestingHR%10,
				"restHr":               v.RestingHR,
				"lowZoneMinutes":       100 + v.LightMinutes%50,
				"lowZoneLowerLimit":    70,
				"lowZonePai":           20 + v.SleepScore%20,
				"mediumZoneMinutes":    30 + v.DeepMinutes%30,
				"mediumZoneLowerLimit": 70,
				"mediumZonePai":        15 + v.RestingHR%15,
				"highZoneMinutes":      10 + v.LightMinutes%15,
				"highZoneLowerLimit":   70,
				"highZonePai":          8 + v.DeepMinutes%8,
			})
		}
	}

	writeJSON(w, http.StatusOK, mustMarshal(map[string]any{mockKeyItems: items}))
}
