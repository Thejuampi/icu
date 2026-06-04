package icu

import "encoding/json"

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
