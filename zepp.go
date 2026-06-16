package icu

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Zepp DTOs reflect the real Zepp/Amazfit cloud API as reverse-engineered
// from huami-token, bentasker/zepp_to_influxdb, and rolandsz/Mi-Fit-and-Zepp-workout-exporter.
//
// The real API returns base64-packed binary data inside `summary` and `data_hr`.
// These DTOs preserve both the raw strings AND the decoded structures so that
// downstream consumers (CLI, AI agent) can pick the level of detail they need.

type ZeppResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type ZeppUserInfo struct {
	UserID   string `json:"userId,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Email    string `json:"email,omitempty"`
	Gender   int    `json:"gender,omitempty"`
	Height   int    `json:"height,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Birthday string `json:"birthday,omitempty"`
	Region   string `json:"region,omitempty"`
}

// BandDataDay is one day of data from /v1/data/band_data.json.
// SummaryRaw is the base64-packed JSON returned by the API; Summary is the
// decoded structure (steps, sleep with stages, goal, serial, last-sync).
// DataHRRaw is the base64-packed binary blob of 2-byte shorts (HR per minute,
// 1440 entries/day); HeartRate is the decoded per-minute series.
type BandDataDay struct {
	Date       string                 `json:"date"`
	UID        string                 `json:"uid,omitempty"`
	DataType   int                    `json:"dataType,omitempty"`
	Source     int                    `json:"source,omitempty"`
	UUID       string                 `json:"uuid,omitempty"`
	SummaryRaw string                 `json:"summaryRaw,omitempty"`
	DataRaw    string                 `json:"dataRaw,omitempty"`
	DataHRRaw  string                 `json:"dataHrRaw,omitempty"`
	Summary    *BandDataSummary       `json:"summary,omitempty"`
	HeartRate  []BandDataHeartPoint   `json:"heartRate,omitempty"`
	Stress     *BandDataStressSummary `json:"stress,omitempty"`
}

// BandDataSummary is the decoded JSON inside BandDataDay.SummaryRaw.
type BandDataSummary struct {
	Steps    *BandDataSteps `json:"stp,omitempty"`
	Sleep    *BandDataSleep `json:"slp,omitempty"`
	Goal     int            `json:"goal,omitempty"`
	Serial   string         `json:"sn,omitempty"`
	LastSync int64          `json:"sync,omitempty"`
	Extras   map[string]any `json:"-"`
}

type BandDataSteps struct {
	Total    int             `json:"ttl,omitempty"`
	Calories int             `json:"cal,omitempty"`
	Distance int             `json:"dis,omitempty"`
	RunDist  int             `json:"runDist,omitempty"`
	Stages   []BandDataStage `json:"stage,omitempty"`
}

type BandDataStage struct {
	Start    int `json:"start,omitempty"`
	Stop     int `json:"stop,omitempty"`
	Mode     int `json:"mode,omitempty"`
	Steps    int `json:"step,omitempty"`
	Calories int `json:"cal,omitempty"`
	Distance int `json:"dis,omitempty"`
}

// BandDataSleep is the decoded structure inside summary.slp.
// st/ed are epoch seconds. dp = deep sleep minutes, lt = light sleep minutes.
// mode 4 = light, mode 5 = deep, mode 7 = awake, mode 8 = REM.
type BandDataSleep struct {
	StartEpoch   int64                `json:"st,omitempty"`
	EndEpoch     int64                `json:"ed,omitempty"`
	DeepMinutes  int                  `json:"dp,omitempty"`
	LightMinutes int                  `json:"lt,omitempty"`
	Stages       []BandDataSleepStage `json:"stage,omitempty"`
}

type BandDataSleepStage struct {
	Start      int   `json:"start,omitempty"`      // minutes since midnight
	Stop       int   `json:"stop,omitempty"`       // minutes since midnight
	StartEpoch int64 `json:"startEpoch,omitempty"` // converted to epoch seconds
	EndEpoch   int64 `json:"endEpoch,omitempty"`   // converted to epoch seconds
	Mode       int   `json:"mode,omitempty"`
}

// BandDataHeartPoint is one minute of heart rate (decoded from DataHRRaw).
// Values 254/255 are sentinel for "no read" / "not required".
type BandDataHeartPoint struct {
	Timestamp int64 `json:"timestamp"`
	BPM       int   `json:"bpm"`
}

// BandDataStressSummary is a convenience roll-up derived from
// per-minute stress events (not from band_data.json itself).
type BandDataStressSummary struct {
	Min       int           `json:"min,omitempty"`
	Max       int           `json:"max,omitempty"`
	Average   int           `json:"avg,omitempty"`
	RelaxPct  int           `json:"relaxPct,omitempty"`
	NormalPct int           `json:"normalPct,omitempty"`
	MediumPct int           `json:"mediumPct,omitempty"`
	HighPct   int           `json:"highPct,omitempty"`
	Points    []StressPoint `json:"points,omitempty"`
}

// StressPoint is one minute of stress from the per-minute series in
// the `all_day_stress` event's `data` field.
type StressPoint struct {
	Time  int64 `json:"time"`
	Value int   `json:"value"`
}

// StressDay is one day of stress from /users/{id}/events?eventType=all_day_stress.
type StressDay struct {
	Date      string        `json:"date"`
	Min       int           `json:"minStress,omitempty"`
	Max       int           `json:"maxStress,omitempty"`
	Average   int           `json:"avgStress,omitempty"`
	RelaxPct  int           `json:"relaxPct,omitempty"`
	NormalPct int           `json:"normalPct,omitempty"`
	MediumPct int           `json:"mediumPct,omitempty"`
	HighPct   int           `json:"highPct,omitempty"`
	RawTime   int64         `json:"timestamp,omitempty"`
	Points    []StressPoint `json:"points,omitempty"`
}

// SpO2Reading is one SpO2 event from /users/{id}/events?eventType=blood_oxygen.
// subType: "odi" (Oxygen Desaturation Index), "osa_event" (apnea),
// "click" (manual reading from the watch).
type SpO2Reading struct {
	Date         string  `json:"date"`
	Timestamp    int64   `json:"timestamp,omitempty"`
	SubType      string  `json:"subType,omitempty"`
	Value        float64 `json:"spo2,omitempty"`
	ODI          float64 `json:"odi,omitempty"`
	ODIScore     float64 `json:"score,omitempty"`
	SpO2Decrease float64 `json:"spo2Decrease,omitempty"`
}

// PAIDay is one day of PAI (Personal Activity Intelligence) data
// from /users/{id}/events?eventType=PaiHealthInfo.
type PAIDay struct {
	Date                 string  `json:"date"`
	Timestamp            int64   `json:"timestamp,omitempty"`
	DailyPAI             float64 `json:"dailyPai,omitempty"`
	TotalPAI             float64 `json:"totalPai,omitempty"`
	MaxHR                int     `json:"maxHr,omitempty"`
	RestingHR            int     `json:"restHr,omitempty"`
	LowZoneMinutes       int     `json:"lowZoneMinutes,omitempty"`
	LowZoneLowerLimit    int     `json:"lowZoneLowerLimit,omitempty"`
	LowZonePAI           float64 `json:"lowZonePai,omitempty"`
	MediumZoneMinutes    int     `json:"mediumZoneMinutes,omitempty"`
	MediumZoneLowerLimit int     `json:"mediumZoneLowerLimit,omitempty"`
	MediumZonePAI        float64 `json:"mediumZonePai,omitempty"`
	HighZoneMinutes      int     `json:"highZoneMinutes,omitempty"`
	HighZoneLowerLimit   int     `json:"highZoneLowerLimit,omitempty"`
	HighZonePAI          float64 `json:"highZonePai,omitempty"`
}

// WorkoutSummary is one workout from /v1/sport/run/history.json.
// TrackID is a UNIX timestamp (seconds). SportType 1 = running, 18 = cycling,
// etc. The actual sport mapping is in ZeppSportType below.
type WorkoutSummary struct {
	TrackID   string `json:"trackId"`
	Type      int    `json:"type,omitempty"`
	SportType int    `json:"sportType,omitempty"`
	StartTime int64  `json:"startTime,omitempty"`
	EndTime   int64  `json:"endTime,omitempty"`
	Duration  int    `json:"duration,omitempty"`
	Distance  int    `json:"distance,omitempty"`
	Calories  int    `json:"calories,omitempty"`
	AvgHR     int    `json:"avgHr,omitempty"`
	MaxHR     int    `json:"maxHr,omitempty"`
	MinHR     int    `json:"minHr,omitempty"`
	AvgPace   int    `json:"avgPace,omitempty"`
	MaxPace   int    `json:"maxPace,omitempty"`
	AvgPower  int    `json:"avgPower,omitempty"`
	MaxPower  int    `json:"maxPower,omitempty"`
	Steps     int    `json:"steps,omitempty"`
}

// WorkoutDetail is the full per-second data for one workout from
// /v1/sport/run/detail.json. Most numeric series are delta-encoded:
// the first value is absolute, every subsequent value is the delta
// from the previous one. Sum them to reconstruct the absolute series.
type WorkoutDetail struct {
	TrackID     string `json:"trackId"`
	SportType   int    `json:"sportType,omitempty"`
	StartTime   int64  `json:"startTime,omitempty"`
	EndTime     int64  `json:"endTime,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	Distance    int    `json:"distance,omitempty"`
	Calories    int    `json:"calories,omitempty"`
	AvgHR       int    `json:"avgHr,omitempty"`
	MaxHR       int    `json:"maxHr,omitempty"`
	MinHR       int    `json:"minHr,omitempty"`
	AvgPower    int    `json:"avgPower,omitempty"`
	MaxPower    int    `json:"maxPower,omitempty"`
	AvgPace     int    `json:"avgPace,omitempty"`
	MaxAltitude int    `json:"maxAltitude,omitempty"`
	MinAltitude int    `json:"minAltitude,omitempty"`
	Ascent      int    `json:"ascent,omitempty"`
	Descent     int    `json:"descent,omitempty"`
	Steps       int    `json:"steps,omitempty"`
	HRSeries    []int  `json:"hrSeries,omitempty"`
	PaceSeries  []int  `json:"paceSeries,omitempty"`
	AltSeries   []int  `json:"altitudeSeries,omitempty"`
	PowerSeries []int  `json:"powerSeries,omitempty"`
	StepSeries  []int  `json:"stepSeries,omitempty"`
}

// SportTypeName maps Zepp/Amazfit sport type integers to human-readable names.
// Source: reverse-engineered from huami-token and the Zepp app.
// Not exhaustive; falls back to "unknown" for unmapped values.
func SportTypeName(t int) string {
	sportNames := map[int]string{
		1: "running", 2: "walking", 3: "cycling", 4: "hiking",
		5: "swimming_pool", 6: "open_water_swim", 7: "elliptical", 8: "rowing",
		9: "climbing", 10: "treadmill", 11: "strength_training", 12: "yoga",
		13: "pilates", 14: "indoor_cycling", 15: "basketball", 16: "football",
		17: "tennis", 18: "badminton", 19: "table_tennis", 20: "golf",
		21: "skiing", 22: "snowboarding", 23: "jump_rope", 24: "dance",
	}
	if name, ok := sportNames[t]; ok {
		return name
	}
	return "unknown"
}

// V2EventPreset identifies a Zepp /v2/users/me/events eventType/subType pair.
// Presets are used by the generic `zepp events` command and as building blocks
// for dedicated wellness commands.
type V2EventPreset struct {
	Name      string
	EventType string
	SubType   string
}

// ZeppV2Event is the normalized output of the generic `zepp events` command.
// It captures the common fields shared by most V2 event items; any additional
// fields are preserved in Extra.
type ZeppV2Event struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Type      string         `json:"type"`
	SubType   string         `json:"subtype,omitempty"`
	Value     float64        `json:"value,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// v2EventPresets returns the registry of supported /v2/users/me/events presets.
func v2EventPresets() map[string]V2EventPreset {
	return map[string]V2EventPreset{
		"hrv-sdnn":       {Name: "hrv-sdnn", EventType: "hrv_sdnn", SubType: "real_data"},
		"hrv-rmssd":      {Name: "hrv-rmssd", EventType: "HRVRMSSD", SubType: "real_data"},
		"readiness":      {Name: "readiness", EventType: "readiness", SubType: "watch_score"},
		"body-battery":   {Name: "body-battery", EventType: "Charge", SubType: "real_data"},
		"stress-minute":  {Name: "stress-minute", EventType: "Charge", SubType: "stress_data"},
		"respiratory":    {Name: "respiratory", EventType: "RespiratoryRate", SubType: "real_data"},
		"daily-health":   {Name: "daily-health", EventType: "DailyHealth", SubType: "summary"},
		"blood-pressure": {Name: "blood-pressure", EventType: "blood_pressure", SubType: "real_data"},
		"emotion":        {Name: "emotion", EventType: "Emotion", SubType: "real_data"},
		"skin-temp":      {Name: "skin-temp", EventType: "skinTemp", SubType: "real_data"},
	}
}

// V2EventPresets returns a copy of the registered V2 event presets.
func V2EventPresets() map[string]V2EventPreset {
	presets := v2EventPresets()
	out := make(map[string]V2EventPreset, len(presets))
	for k, v := range presets {
		out[k] = v
	}

	return out
}

// V2EventPresetByName returns a preset by name, or ok=false if unknown.
func V2EventPresetByName(name string) (V2EventPreset, bool) {
	p, ok := v2EventPresets()[name]
	return p, ok
}

const (
	zeppTimestampKey = "timestamp"
	zeppSportRide    = "ride"
)

// SportNameToSegment returns the Zepp URL segment for a sport name.
func SportNameToSegment(name string) (string, bool) {
	switch name {
	case "run":
		return "run", true
	case "walking":
		return "walking", true
	case zeppSportRide, "cycling":
		return zeppSportRide, true
	case "swimming":
		return "swimming", true
	default:
		return "", false
	}
}

// DecodeV2Events normalizes a raw /v2/users/me/events response into a slice of
// ZeppV2Event. It extracts common fields (timestamp, type, subtype, value) and
// preserves any remaining item fields in Extra.
func DecodeV2Events(raw []byte) ([]ZeppV2Event, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode v2 events: %w", err)
	}

	events := make([]ZeppV2Event, 0, len(payload.Items))
	for _, item := range payload.Items {
		ev := ZeppV2Event{Extra: make(map[string]any)}

		if ts, ok := item[zeppTimestampKey].(float64); ok {
			ev.Timestamp = int64(ts)
			ev.Date = time.UnixMilli(ev.Timestamp).UTC().Format("2006-01-02")
		}

		if t, ok := item["type"].(string); ok {
			ev.Type = t
		} else if t, ok := item["eventType"].(string); ok {
			ev.Type = t
		}

		if st, ok := item["subType"].(string); ok {
			ev.SubType = st
		}

		if v, ok := item["value"]; ok {
			ev.Value = zeppAnyToFloat(v)
		}

		for k, v := range item {
			switch k {
			case zeppTimestampKey, "type", "eventType", "subType", "value":
				continue
			default:
				ev.Extra[k] = v
			}
		}

		events = append(events, ev)
	}

	return events, nil
}

// zeppAnyToFloat coerces a JSON value (number or numeric string) to float64.
func zeppAnyToFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f
		}
	}

	return 0
}

// decodeV2EventsAs maps a raw /v2/users/me/events response to a typed slice
// using the provided conversion function.
func decodeV2EventsAs[T any](raw []byte, fn func(ZeppV2Event) T) ([]T, error) {
	events, err := DecodeV2Events(raw)
	if err != nil {
		return nil, err
	}

	out := make([]T, 0, len(events))
	for _, ev := range events {
		out = append(out, fn(ev))
	}

	return out, nil
}

// ZeppHRVEvent is one nightly HRV reading from /v2/users/me/events.
type ZeppHRVEvent struct {
	Timestamp int64   `json:"timestamp"`
	Date      string  `json:"date"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
}

// DecodeHRV decodes a raw V2 events response into HRV events for the given metric.
func DecodeHRV(raw []byte, metric string) ([]ZeppHRVEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppHRVEvent {
		return ZeppHRVEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Metric:    metric,
			Value:     ev.Value,
		}
	})
}

// ZeppReadinessEvent is one daily readiness score.
type ZeppReadinessEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Score     float64        `json:"score"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeReadiness decodes a raw readiness V2 events response.
func DecodeReadiness(raw []byte) ([]ZeppReadinessEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppReadinessEvent {
		return ZeppReadinessEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Score:     ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppBodyBatteryEvent is one body-battery / Charge reading.
type ZeppBodyBatteryEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Level     float64        `json:"level"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeBodyBattery decodes a raw body-battery V2 events response.
func DecodeBodyBattery(raw []byte) ([]ZeppBodyBatteryEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppBodyBatteryEvent {
		return ZeppBodyBatteryEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Level:     ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppHealthSummaryEvent is one daily health summary from DailyHealth/summary.
type ZeppHealthSummaryEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeHealthSummary decodes a raw daily-health V2 events response.
func DecodeHealthSummary(raw []byte) ([]ZeppHealthSummaryEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppHealthSummaryEvent {
		return ZeppHealthSummaryEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Extra:     ev.Extra,
		}
	})
}

// ZeppMoodEvent is one mood / emotion reading.
type ZeppMoodEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Mood      float64        `json:"mood"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeMood decodes a raw emotion V2 events response.
func DecodeMood(raw []byte) ([]ZeppMoodEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppMoodEvent {
		return ZeppMoodEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Mood:      ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppSkinTempEvent is one skin temperature reading.
type ZeppSkinTempEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Delta     float64        `json:"delta"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeSkinTemp decodes a raw skinTemp V2 events response.
func DecodeSkinTemp(raw []byte) ([]ZeppSkinTempEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppSkinTempEvent {
		return ZeppSkinTempEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Delta:     ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppStressMinuteEvent is one per-minute stress reading from
// /v2/users/me/events?eventType=Charge&subType=stress_data.
type ZeppStressMinuteEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Stress    float64        `json:"stress"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeStressMinute decodes a raw per-minute stress V2 events response.
func DecodeStressMinute(raw []byte) ([]ZeppStressMinuteEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppStressMinuteEvent {
		return ZeppStressMinuteEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Stress:    ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppRespiratoryRateEvent is one overnight respiratory rate reading.
type ZeppRespiratoryRateEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Rate      float64        `json:"rate"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeRespiratoryRate decodes a raw respiratory rate V2 events response.
func DecodeRespiratoryRate(raw []byte) ([]ZeppRespiratoryRateEvent, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppRespiratoryRateEvent {
		return ZeppRespiratoryRateEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Rate:      ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppBloodPressureEvent is one blood-pressure reading.
type ZeppBloodPressureEvent struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Systolic  int            `json:"systolic"`
	Diastolic int            `json:"diastolic"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeBloodPressure decodes a raw blood_pressure V2 events response.
func DecodeBloodPressure(raw []byte) ([]ZeppBloodPressureEvent, error) {
	events, err := DecodeV2Events(raw)
	if err != nil {
		return nil, err
	}

	out := make([]ZeppBloodPressureEvent, 0, len(events))
	for _, ev := range events {
		bp := ZeppBloodPressureEvent{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
		}

		bp.Systolic = zeppIntFromExtra(ev.Extra, "systolic", "highPressure", "sys")
		bp.Diastolic = zeppIntFromExtra(ev.Extra, "diastolic", "lowPressure", "dia")

		bp.Extra = make(map[string]any)
		for k, v := range ev.Extra {
			switch k {
			case "systolic", "highPressure", "sys", "diastolic", "lowPressure", "dia":
				continue
			default:
				bp.Extra[k] = v
			}
		}

		out = append(out, bp)
	}

	return out, nil
}

// ZeppSportLoad is one daily training load reading from the watch's
// WatchSportStatistics/SPORT_LOAD endpoint.
type ZeppSportLoad struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Load      float64        `json:"load"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeSportLoad decodes a raw SPORT_LOAD response.
func DecodeSportLoad(raw []byte) ([]ZeppSportLoad, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppSportLoad {
		return ZeppSportLoad{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Load:      ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppVO2Max is one VO2 max estimate from the watch's
// WatchSportStatistics/VO2_MAX endpoint.
type ZeppVO2Max struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	VO2Max    float64        `json:"vo2max"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeVO2Max decodes a raw VO2_MAX response.
func DecodeVO2Max(raw []byte) ([]ZeppVO2Max, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppVO2Max {
		return ZeppVO2Max{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			VO2Max:    ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppHeartRateReading is one heart-rate reading from /users/{id}/heartRate.
type ZeppHeartRateReading struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	HR        float64        `json:"hr"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeHeartRateEndpoint decodes a raw /users/{id}/heartRate response.
func DecodeHeartRateEndpoint(raw []byte) ([]ZeppHeartRateReading, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppHeartRateReading {
		return ZeppHeartRateReading{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			HR:        ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppWeightRecord is one weight measurement from /users/{id}/members/-1/weightRecords.
type ZeppWeightRecord struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Weight    float64        `json:"weight"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeWeightRecords decodes a raw weight records response.
func DecodeWeightRecords(raw []byte) ([]ZeppWeightRecord, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppWeightRecord {
		return ZeppWeightRecord{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Weight:    ev.Value,
			Extra:     ev.Extra,
		}
	})
}

// ZeppManualDataEntry is one manually-entered wellness record from
// /v1/user/manualData.json.
type ZeppManualDataEntry struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeManualData decodes a raw manual data response.
func DecodeManualData(raw []byte) ([]ZeppManualDataEntry, error) {
	return decodeV2EventsAs(raw, func(ev ZeppV2Event) ZeppManualDataEntry {
		return ZeppManualDataEntry{
			Timestamp: ev.Timestamp,
			Date:      ev.Date,
			Extra:     ev.Extra,
		}
	})
}

// ZeppSecondHeartRateFile is one COS file index entry for per-second HR
// data from /users/me/fileInfo/events.
type ZeppSecondHeartRateFile struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	URL       string         `json:"url"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeSecondHeartRateFiles decodes a raw /users/me/fileInfo/events response.
func DecodeSecondHeartRateFiles(raw []byte) ([]ZeppSecondHeartRateFile, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode second heart rate files: %w", err)
	}

	out := make([]ZeppSecondHeartRateFile, 0, len(payload.Items))
	for _, item := range payload.Items {
		f := ZeppSecondHeartRateFile{Extra: make(map[string]any)}

		if ts, ok := item[zeppTimestampKey].(float64); ok {
			f.Timestamp = int64(ts)
			f.Date = time.UnixMilli(f.Timestamp).UTC().Format("2006-01-02")
		}

		if u, ok := item["url"].(string); ok {
			f.URL = u
		}

		for k, v := range item {
			if k == zeppTimestampKey || k == "url" {
				continue
			}

			f.Extra[k] = v
		}

		out = append(out, f)
	}

	return out, nil
}

// ZeppSpO2Window is one SpO2 ODI/OSA window from
// /users/{id}/events/dateString.
type ZeppSpO2Window struct {
	Timestamp int64          `json:"timestamp"`
	Date      string         `json:"date"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// DecodeSpO2Windows decodes a raw /users/{id}/events/dateString response.
func DecodeSpO2Windows(raw []byte) ([]ZeppSpO2Window, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode spo2 windows: %w", err)
	}

	out := make([]ZeppSpO2Window, 0, len(payload.Items))
	for _, item := range payload.Items {
		w := ZeppSpO2Window{Extra: make(map[string]any)}

		if ts, ok := item[zeppTimestampKey].(float64); ok {
			w.Timestamp = int64(ts)
			w.Date = time.UnixMilli(w.Timestamp).UTC().Format("2006-01-02")
		}

		for k, v := range item {
			if k == zeppTimestampKey {
				continue
			}

			w.Extra[k] = v
		}

		out = append(out, w)
	}

	return out, nil
}

// DecodeBloodPressureUser decodes the /users/me/bloodPressure response. It
// accepts the same field aliases as DecodeBloodPressure.
func DecodeBloodPressureUser(raw []byte) ([]ZeppBloodPressureEvent, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode blood pressure user: %w", err)
	}

	out := make([]ZeppBloodPressureEvent, 0, len(payload.Items))
	for _, item := range payload.Items {
		bp := ZeppBloodPressureEvent{Extra: make(map[string]any)}

		if ts, ok := item[zeppTimestampKey].(float64); ok {
			bp.Timestamp = int64(ts)
			bp.Date = time.UnixMilli(bp.Timestamp).UTC().Format("2006-01-02")
		}

		bp.Systolic = zeppIntFromExtra(item, "systolic", "highPressure", "sys")
		bp.Diastolic = zeppIntFromExtra(item, "diastolic", "lowPressure", "dia")

		for k, v := range item {
			switch k {
			case zeppTimestampKey, "systolic", "highPressure", "sys", "diastolic", "lowPressure", "dia":
				continue
			default:
				bp.Extra[k] = v
			}
		}

		out = append(out, bp)
	}

	return out, nil
}

// zeppIntFromExtra returns the first integer value found under the given keys.
func zeppIntFromExtra(extra map[string]any, keys ...string) int {
	for _, key := range keys {
		switch v := extra[key].(type) {
		case float64:
			return int(v)
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}

	return 0
}
