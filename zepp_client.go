package icu

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// flexInt accepts both JSON number and JSON string during unmarshal.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if data[0] == '"' {
		var s string

		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("unmarshal flexInt string: %w", err)
		}

		v, err := strconv.Atoi(s)
		if err != nil {
			return nil //nolint:nilerr // invalid string defaults to zero
		}

		*f = flexInt(v)

		return nil
	}

	var v int

	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("unmarshal flexInt number: %w", err)
	}

	*f = flexInt(v)

	return nil
}

// flexFloat accepts both JSON number and JSON string during unmarshal.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if data[0] == '"' {
		var s string

		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("unmarshal flexFloat string: %w", err)
		}

		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil //nolint:nilerr // invalid string defaults to zero
		}

		*f = flexFloat(v)

		return nil
	}

	var v float64

	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("unmarshal flexFloat number: %w", err)
	}

	*f = flexFloat(v)

	return nil
}

func atoiOrZero(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return v
}

var (
	ErrZeppNotAuthenticated = errors.New("zepp not authenticated: run 'icu zepp login'")
	ErrZeppServer           = errors.New("zepp server error")
	ErrZeppUnknownSport     = errors.New("unknown zepp sport")
)

type ZeppClient struct {
	httpClient  *http.Client
	loginToken  string
	appToken    string
	userID      string
	countryCode string
	dataHost    string
	eventsBase  string
}

type ZeppClientOption func(*ZeppClient)

func WithZeppHTTPClient(httpClient *http.Client) ZeppClientOption {
	return func(client *ZeppClient) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithZeppBaseURL(baseURL string) ZeppClientOption {
	return func(client *ZeppClient) {
		if baseURL != "" {
			client.dataHost = baseURL
		}
	}
}

func WithZeppEventsURL(eventsURL string) ZeppClientOption {
	return func(client *ZeppClient) {
		if eventsURL != "" {
			client.eventsBase = eventsURL
		}
	}
}

func WithZeppCountryCode(countryCode string) ZeppClientOption {
	return func(client *ZeppClient) {
		if countryCode != "" {
			client.countryCode = countryCode
			client.dataHost = zeppDataHostFor(countryCode)
		}
	}
}

func NewZeppClientFromAuth(auth *ZeppAuthResult, options ...ZeppClientOption) *ZeppClient {
	client := &ZeppClient{
		httpClient:  &http.Client{},
		loginToken:  auth.LoginToken,
		appToken:    auth.AppToken,
		userID:      auth.UserID,
		countryCode: auth.CountryCode,
		dataHost:    zeppDataHostFor(auth.CountryCode),
		eventsBase:  zeppEventsHost,
	}

	for _, option := range options {
		option(client)
	}

	if client.dataHost == "" {
		client.dataHost = zeppDataHostGlobal
	}

	if client.eventsBase == "" {
		client.eventsBase = zeppEventsHost
	}

	return client
}

// ensureAuthenticated returns an error if the client has no auth tokens.
func (c *ZeppClient) ensureAuthenticated() error {
	if c.appToken == "" || c.userID == "" {
		return ErrZeppNotAuthenticated
	}

	return nil
}

// DataHostForTest returns the data host the client is using.
// Exposed only for tests; production code should not depend on this.
func (c *ZeppClient) DataHostForTest() string {
	return c.dataHost
}

// doGet performs a GET request to the data host with the apptoken header.
func (c *ZeppClient) doGet(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	endpoint := c.dataHost + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Apptoken", c.appToken)
	req.Header.Set("Appname", "com.xiaomi.hm.health")
	req.Header.Set("Appplatform", "web")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// BandData fetches the daily summary (steps, sleep, HR-by-minute) for the
// given date range. The response is decoded: each day's `summary` and `data_hr`
// base64-packed blobs are unpacked into typed structures.
func (c *ZeppClient) BandData(ctx context.Context, oldest, newest string) ([]BandDataDay, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	if _, err := parseDateToMillis(oldest); err != nil {
		return nil, fmt.Errorf("parse oldest date: %w", err)
	}

	if _, err := parseDateEndOfDayMillis(newest); err != nil {
		return nil, fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"query_type":  {"detail"},
		"device_type": {"android_phone"},
		"userid":      {c.userID},
		"from_date":   {oldest},
		"to_date":     {newest},
	}

	resp, err := c.doGet(ctx, zeppBandDataPath, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
		Data    []struct {
			UID      string `json:"uid"`
			DataType int    `json:"data_type"`
			DateTime string `json:"date_time"`
			Source   int    `json:"source"`
			UUID     string `json:"uuid"`
			Summary  string `json:"summary"`
			Data     string `json:"data"`
			DataHR   string `json:"data_hr"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if raw.Code != 1 && raw.Code != 0 {
		return nil, fmt.Errorf("%w: %s", ErrZeppServer, raw.Message)
	}

	days := make([]BandDataDay, 0, len(raw.Data))

	for i := range raw.Data {
		item := raw.Data[i]
		day := BandDataDay{
			Date:       item.DateTime,
			UID:        item.UID,
			DataType:   item.DataType,
			Source:     item.Source,
			UUID:       item.UUID,
			SummaryRaw: item.Summary,
			DataRaw:    item.Data,
			DataHRRaw:  item.DataHR,
		}

		if summary, err := decodeBandDataSummary(item.Summary, item.DateTime); err == nil {
			day.Summary = summary
		}

		if points, err := decodeBandDataHeartRate(item.DataHR, item.DateTime); err == nil {
			day.HeartRate = points
		}

		days = append(days, day)
	}

	return days, nil
}

// SleepDays fetches sleep data for the date range. Sleep is extracted from
// the decoded band_data summary; days with no sleep entry are omitted.
func (c *ZeppClient) SleepDays(ctx context.Context, oldest, newest string) ([]BandDataSleep, error) {
	days, err := c.BandData(ctx, oldest, newest)
	if err != nil {
		return nil, err
	}

	sleeps := make([]BandDataSleep, 0, len(days))

	for i := range days {
		if days[i].Summary == nil || days[i].Summary.Sleep == nil {
			continue
		}

		sleeps = append(sleeps, *days[i].Summary.Sleep)
	}

	return sleeps, nil
}

// HeartRateSeries fetches per-minute heart rate for the date range. Returns
// one slice per day in chronological order.
func (c *ZeppClient) HeartRateSeries(ctx context.Context, oldest, newest string) ([][]BandDataHeartPoint, error) {
	days, err := c.BandData(ctx, oldest, newest)
	if err != nil {
		return nil, err
	}

	series := make([][]BandDataHeartPoint, 0, len(days))
	for i := range days {
		series = append(series, days[i].HeartRate)
	}

	return series, nil
}

// StressDays fetches stress data from the Zepp events endpoint.
func (c *ZeppClient) StressDays(ctx context.Context, oldest, newest string) ([]StressDay, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	from, err := parseDateToMillis(oldest)
	if err != nil {
		return nil, fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return nil, fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"from":      {strconv.FormatInt(from, 10)},
		"to":        {strconv.FormatInt(to, 10)},
		"eventType": {"all_day_stress"},
		"limit":     {"1000"},
	}

	resp, err := c.doGetEvents(ctx, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Items []struct {
			Timestamp        int64           `json:"timestamp"`
			MinStress        flexInt         `json:"minStress"`
			MaxStress        flexInt         `json:"maxStress"`
			AvgStress        flexInt         `json:"avgStress"`
			RelaxProportion  flexInt         `json:"relaxProportion"`
			NormalProportion flexInt         `json:"normalProportion"`
			MediumProportion flexInt         `json:"mediumProportion"`
			HighProportion   flexInt         `json:"highProportion"`
			Data             json.RawMessage `json:"data,omitempty"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := make([]StressDay, 0, len(raw.Items))

	for _, item := range raw.Items {
		day := StressDay{
			RawTime:   item.Timestamp,
			Min:       int(item.MinStress),
			Max:       int(item.MaxStress),
			Average:   int(item.AvgStress),
			RelaxPct:  int(item.RelaxProportion),
			NormalPct: int(item.NormalProportion),
			MediumPct: int(item.MediumProportion),
			HighPct:   int(item.HighProportion),
			Date:      time.UnixMilli(item.Timestamp).UTC().Format("2006-01-02"),
		}

		if len(item.Data) > 0 {
			var points []struct {
				Time  int64 `json:"time"`
				Value int   `json:"value"`
			}

			if err := json.Unmarshal(item.Data, &points); err == nil {
				day.Points = make([]StressPoint, 0, len(points))
				for _, p := range points {
					day.Points = append(day.Points, StressPoint{Time: p.Time, Value: p.Value})
				}
			}
		}

		out = append(out, day)
	}

	return out, nil
}

// SpO2Readings fetches SpO2 (blood oxygen) events from the Zepp events endpoint.
func (c *ZeppClient) SpO2Readings(ctx context.Context, oldest, newest string) ([]SpO2Reading, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	from, err := parseDateToMillis(oldest)
	if err != nil {
		return nil, fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return nil, fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"from":      {strconv.FormatInt(from, 10)},
		"to":        {strconv.FormatInt(to, 10)},
		"eventType": {"blood_oxygen"},
		"limit":     {"1000"},
	}

	resp, err := c.doGetEvents(ctx, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Items []struct {
			Timestamp int64     `json:"timestamp"`
			SubType   string    `json:"subType"`
			ODI       flexFloat `json:"odi"`
			Score     flexFloat `json:"score"`
			Extra     string    `json:"extra,omitempty"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := make([]SpO2Reading, 0, len(raw.Items))

	for _, item := range raw.Items {
		reading := SpO2Reading{
			Timestamp: item.Timestamp,
			SubType:   item.SubType,
			ODI:       float64(item.ODI),
			ODIScore:  float64(item.Score),
			Date:      time.UnixMilli(item.Timestamp).UTC().Format("2006-01-02"),
		}

		if item.Extra != "" {
			var extra map[string]any

			if err := json.Unmarshal([]byte(item.Extra), &extra); err == nil {
				reading.Value = zeppExtraFloat(extra, "spo2")
				reading.SpO2Decrease = zeppExtraFloat(extra, "spo2Decrease")
			}
		}

		out = append(out, reading)
	}

	return out, nil
}

func zeppExtraFloat(extra map[string]any, key string) float64 {
	switch v := extra[key].(type) {
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}

	return 0
}

// PAIDays fetches Personal Activity Intelligence data.
func (c *ZeppClient) PAIDays(ctx context.Context, oldest, newest string) ([]PAIDay, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	from, err := parseDateToMillis(oldest)
	if err != nil {
		return nil, fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return nil, fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"from":      {strconv.FormatInt(from, 10)},
		"to":        {strconv.FormatInt(to, 10)},
		"eventType": {"PaiHealthInfo"},
		"limit":     {"1000"},
	}

	resp, err := c.doGetEvents(ctx, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Items []struct {
			Timestamp            int64     `json:"timestamp"`
			DailyPAI             flexFloat `json:"dailyPai"`
			TotalPAI             flexFloat `json:"totalPai"`
			MaxHR                flexInt   `json:"maxHr"`
			RestHR               flexInt   `json:"restHr"`
			LowZoneMinutes       flexInt   `json:"lowZoneMinutes"`
			LowZoneLowerLimit    flexInt   `json:"lowZoneLowerLimit"`
			LowZonePAI           flexFloat `json:"lowZonePai"`
			MediumZoneMinutes    flexInt   `json:"mediumZoneMinutes"`
			MediumZoneLowerLimit flexInt   `json:"mediumZoneLowerLimit"`
			MediumZonePAI        flexFloat `json:"mediumZonePai"`
			HighZoneMinutes      flexInt   `json:"highZoneMinutes"`
			HighZoneLowerLimit   flexInt   `json:"highZoneLowerLimit"`
			HighZonePAI          flexFloat `json:"highZonePai"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := make([]PAIDay, 0, len(raw.Items))

	for _, item := range raw.Items {
		day := PAIDay{
			Timestamp:            item.Timestamp,
			Date:                 time.UnixMilli(item.Timestamp).UTC().Format("2006-01-02"),
			DailyPAI:             float64(item.DailyPAI),
			TotalPAI:             float64(item.TotalPAI),
			MaxHR:                int(item.MaxHR),
			RestingHR:            int(item.RestHR),
			LowZoneMinutes:       int(item.LowZoneMinutes),
			LowZoneLowerLimit:    int(item.LowZoneLowerLimit),
			LowZonePAI:           float64(item.LowZonePAI),
			MediumZoneMinutes:    int(item.MediumZoneMinutes),
			MediumZoneLowerLimit: int(item.MediumZoneLowerLimit),
			MediumZonePAI:        float64(item.MediumZonePAI),
			HighZoneMinutes:      int(item.HighZoneMinutes),
			HighZoneLowerLimit:   int(item.HighZoneLowerLimit),
			HighZonePAI:          float64(item.HighZonePAI),
		}

		out = append(out, day)
	}

	return out, nil
}

// Workouts fetches all workouts. The API returns all workouts (pagination
// via trackid cursor). Date filtering is done client-side from the trackid
// (which is a Unix timestamp in seconds).
func (c *ZeppClient) Workouts(ctx context.Context, sport, oldest, newest string) ([]WorkoutSummary, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	segment, ok := SportNameToSegment(sport)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrZeppUnknownSport, sport)
	}

	// trackid is UTC epoch seconds, so parse dates as UTC.
	oldestSec, err := parseDateToSecondsUTC(oldest)
	if err != nil {
		return nil, fmt.Errorf("parse oldest date: %w", err)
	}

	newestSec, err := parseDateEndOfDaySecondsUTC(newest)
	if err != nil {
		return nil, fmt.Errorf("parse newest date: %w", err)
	}

	var summaries []WorkoutSummary

	nextID := int64(0)

	for {
		page, next, err := c.workoutsPage(ctx, segment, nextID)
		if err != nil {
			return nil, err
		}

		for i := range page {
			trackSec, err := strconv.ParseInt(page[i].TrackID, 10, 64)
			if err != nil {
				continue
			}

			if trackSec > newestSec {
				continue
			}

			if trackSec < oldestSec {
				return summaries, nil
			}

			summaries = append(summaries, page[i])
		}

		if next == -1 {
			break
		}

		nextID = next
	}

	return summaries, nil
}

func (c *ZeppClient) workoutsPage(ctx context.Context, sport string, trackID int64) ([]WorkoutSummary, int64, error) {
	query := url.Values{}

	if trackID > 0 {
		query.Set("trackid", strconv.FormatInt(trackID, 10))
	}

	resp, err := c.doGet(ctx, SportHistoryURL("", sport), query)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
		Data    struct {
			Next    int `json:"next"`
			Summary []struct {
				TrackID    string `json:"trackid"`
				WorkoutDis string `json:"dis"`
				Calorie    string `json:"calorie"`
				EndTime    string `json:"end_time"`
				RunTime    string `json:"run_time"`
				AvgPace    string `json:"avg_pace"`
				AvgHR      string `json:"avg_heart_rate"`
				SportType  int    `json:"type"`
			} `json:"summary"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, fmt.Errorf("decode response: %w", err)
	}

	if raw.Code != 1 && raw.Code != 0 {
		return nil, 0, fmt.Errorf("%w: %s", ErrZeppServer, raw.Message)
	}

	out := make([]WorkoutSummary, 0, len(raw.Data.Summary))

	for _, item := range raw.Data.Summary {
		endTime, _ := strconv.ParseInt(item.EndTime, 10, 64)
		distance, _ := strconv.Atoi(item.WorkoutDis)
		calories, _ := strconv.Atoi(item.Calorie)
		runTime, _ := strconv.Atoi(item.RunTime)
		avgPace, _ := strconv.Atoi(item.AvgPace)
		avgHR, _ := strconv.Atoi(item.AvgHR)

		summary := WorkoutSummary{
			TrackID:   item.TrackID,
			SportType: item.SportType,
			EndTime:   endTime,
			Duration:  runTime,
			Distance:  distance,
			Calories:  calories,
			AvgPace:   avgPace,
			AvgHR:     avgHR,
		}

		out = append(out, summary)
	}

	return out, int64(raw.Data.Next), nil
}

// Workout fetches the full per-second detail for a single workout.
// The returned `WorkoutDetail` includes the decoded (absolute) numeric series.
func (c *ZeppClient) Workout(ctx context.Context, sport, trackID string) (WorkoutDetail, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return WorkoutDetail{}, err
	}

	segment, ok := SportNameToSegment(sport)
	if !ok {
		return WorkoutDetail{}, fmt.Errorf("%w: %s", ErrZeppUnknownSport, sport)
	}

	query := url.Values{
		"trackid": {trackID},
		"source":  {segment + ".mifit.huami.com"},
	}

	resp, err := c.doGet(ctx, SportDetailURL("", segment), query)
	if err != nil {
		return WorkoutDetail{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WorkoutDetail{}, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
		Data    struct {
			TrackID     string `json:"trackid"`
			Type        int    `json:"type"`
			StartTime   int64  `json:"startTime"`
			EndTime     int64  `json:"endTime"`
			Duration    int    `json:"duration"`
			Distance    int    `json:"distance"`
			Calories    int    `json:"calories"`
			AvgHR       int    `json:"avgHR"`
			MaxHR       int    `json:"maxHR"`
			MinHR       int    `json:"minHR"`
			AvgPower    int    `json:"avgPower"`
			MaxPower    int    `json:"maxPower"`
			AvgPace     int    `json:"avgPace"`
			MaxAltitude int    `json:"maxAltitude"`
			MinAltitude int    `json:"minAltitude"`
			Ascent      int    `json:"rise"`
			Descent     int    `json:"decline"`
			Steps       int    `json:"step"`
			HRDelta     string `json:"hr_split"`
			PaceDelta   string `json:"pace_split"`
			AltDelta    string `json:"altitude_split"`
			PowerDelta  string `json:"power_split"`
			StepDelta   string `json:"step_split"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return WorkoutDetail{}, fmt.Errorf("decode response: %w", err)
	}

	if raw.Code != 1 && raw.Code != 0 {
		return WorkoutDetail{}, fmt.Errorf("%w: %s", ErrZeppServer, raw.Message)
	}

	workoutData := raw.Data

	detail := WorkoutDetail{
		TrackID:     workoutData.TrackID,
		SportType:   workoutData.Type,
		StartTime:   workoutData.StartTime,
		EndTime:     workoutData.EndTime,
		Duration:    workoutData.Duration,
		Distance:    workoutData.Distance,
		Calories:    workoutData.Calories,
		AvgHR:       workoutData.AvgHR,
		MaxHR:       workoutData.MaxHR,
		MinHR:       workoutData.MinHR,
		AvgPower:    workoutData.AvgPower,
		MaxPower:    workoutData.MaxPower,
		AvgPace:     workoutData.AvgPace,
		MaxAltitude: workoutData.MaxAltitude,
		MinAltitude: workoutData.MinAltitude,
		Ascent:      workoutData.Ascent,
		Descent:     workoutData.Descent,
		Steps:       workoutData.Steps,
	}

	if decoded, err := decodeDeltaEncodedShorts(workoutData.HRDelta); err == nil {
		detail.HRSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(workoutData.PaceDelta); err == nil {
		detail.PaceSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(workoutData.AltDelta); err == nil {
		detail.AltSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(workoutData.PowerDelta); err == nil {
		detail.PowerSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(workoutData.StepDelta); err == nil {
		detail.StepSeries = decoded
	}

	return detail, nil
}

// UserInfo fetches the Zepp user profile for the current login.
// NOTE: UserInfo is only used as a token-validity check; the regional
// endpoint varies. We attempt the regional host first and fall back to
// the global host.
func (c *ZeppClient) UserInfo(ctx context.Context) (ZeppUserInfo, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return ZeppUserInfo{}, err
	}

	info, err := c.userInfoFromHost(ctx, c.dataHost)
	if err == nil {
		return info, nil
	}

	if c.dataHost != zeppDataHostGlobal {
		return c.userInfoFromHost(ctx, zeppDataHostGlobal)
	}

	return ZeppUserInfo{}, err
}

func (c *ZeppClient) userInfoFromHost(ctx context.Context, host string) (ZeppUserInfo, error) {
	saved := c.dataHost
	c.dataHost = host

	defer func() { c.dataHost = saved }()

	query := url.Values{
		"r":           {randomRequestID()},
		"userid":      {c.userID},
		"appid":       {"428135909242707968"},
		"channel":     {"Normal"},
		"country":     {c.countryCode},
		"cv":          {"151689_9.12.5"},
		"device":      {"android_32"},
		"device_type": {"android_phone"},
		"lang":        {"en_US"},
		"timezone":    {"UTC"},
		"v":           {"2.0"},
	}

	resp, err := c.doGet(ctx, zeppUserInfoPath, query)
	if err != nil {
		return ZeppUserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ZeppUserInfo{}, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
		Data    struct {
			UserID   string `json:"userId"`
			Nickname string `json:"nickname"`
			Email    string `json:"email"`
			Gender   int    `json:"gender"`
			Height   int    `json:"height"`
			Weight   int    `json:"weight"`
			Birthday string `json:"birthday"`
			Region   string `json:"region"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return ZeppUserInfo{}, fmt.Errorf("decode response: %w", err)
	}

	if raw.Code != 1 && raw.Code != 0 && raw.Message != "success" {
		return ZeppUserInfo{}, fmt.Errorf("%w: %s", ErrZeppServer, raw.Message)
	}

	userData := raw.Data

	return ZeppUserInfo{
		UserID:   userData.UserID,
		Nickname: userData.Nickname,
		Email:    userData.Email,
		Gender:   userData.Gender,
		Height:   userData.Height,
		Weight:   userData.Weight,
		Birthday: userData.Birthday,
		Region:   userData.Region,
	}, nil
}

// FetchV2Events fetches raw bytes from /v2/users/me/events for a given preset
// and date range. The request goes to the regional data host (matching the
// behaviour observed in m4ary/zepp-health-cli).
func (c *ZeppClient) FetchV2Events(ctx context.Context, preset V2EventPreset, oldest, newest string) ([]byte, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	endpoint, err := V2EventsURL(c.dataHost, preset, oldest, newest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Apptoken", c.appToken)
	req.Header.Set("Appname", "com.xiaomi.hm.health")
	req.Header.Set("Appplatform", "web")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return body, nil
}

// HRVSDNNDays fetches nightly HRV SDNN values from /v2/users/me/events.
func (c *ZeppClient) HRVSDNNDays(ctx context.Context, oldest, newest string) ([]ZeppHRVEvent, error) {
	preset, _ := V2EventPresetByName("hrv-sdnn")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch hrv sdnn: %w", err)
	}

	return DecodeHRV(raw, "sdnn")
}

// HRVRMSSDDays fetches nightly HRV RMSSD values from /v2/users/me/events.
func (c *ZeppClient) HRVRMSSDDays(ctx context.Context, oldest, newest string) ([]ZeppHRVEvent, error) {
	preset, _ := V2EventPresetByName("hrv-rmssd")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch hrv rmssd: %w", err)
	}

	return DecodeHRV(raw, "rmssd")
}

// ReadinessDays fetches daily readiness scores from /v2/users/me/events.
func (c *ZeppClient) ReadinessDays(ctx context.Context, oldest, newest string) ([]ZeppReadinessEvent, error) {
	preset, _ := V2EventPresetByName("readiness")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch readiness: %w", err)
	}

	return DecodeReadiness(raw)
}

// BodyBatteryDays fetches daily body-battery levels from /v2/users/me/events.
func (c *ZeppClient) BodyBatteryDays(ctx context.Context, oldest, newest string) ([]ZeppBodyBatteryEvent, error) {
	preset, _ := V2EventPresetByName("body-battery")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch body battery: %w", err)
	}

	return DecodeBodyBattery(raw)
}

// HealthSummaryDays fetches daily health summaries from /v2/users/me/events.
func (c *ZeppClient) HealthSummaryDays(ctx context.Context, oldest, newest string) ([]ZeppHealthSummaryEvent, error) {
	preset, _ := V2EventPresetByName("daily-health")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch health summary: %w", err)
	}

	return DecodeHealthSummary(raw)
}

// MoodDays fetches mood / emotion readings from /v2/users/me/events.
func (c *ZeppClient) MoodDays(ctx context.Context, oldest, newest string) ([]ZeppMoodEvent, error) {
	preset, _ := V2EventPresetByName("emotion")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch mood: %w", err)
	}

	return DecodeMood(raw)
}

// SkinTempDays fetches skin temperature readings from /v2/users/me/events.
func (c *ZeppClient) SkinTempDays(ctx context.Context, oldest, newest string) ([]ZeppSkinTempEvent, error) {
	preset, _ := V2EventPresetByName("skin-temp")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch skin temp: %w", err)
	}

	return DecodeSkinTemp(raw)
}

// StressMinuteDays fetches per-minute stress readings from /v2/users/me/events.
func (c *ZeppClient) StressMinuteDays(ctx context.Context, oldest, newest string) ([]ZeppStressMinuteEvent, error) {
	preset, _ := V2EventPresetByName("stress-minute")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch stress minute: %w", err)
	}

	return DecodeStressMinute(raw)
}

// RespiratoryRateDays fetches overnight respiratory rate readings.
func (c *ZeppClient) RespiratoryRateDays(ctx context.Context, oldest, newest string) ([]ZeppRespiratoryRateEvent, error) {
	preset, _ := V2EventPresetByName("respiratory")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch respiratory rate: %w", err)
	}

	return DecodeRespiratoryRate(raw)
}

// BloodPressureDays fetches blood-pressure readings from /v2/users/me/events.
func (c *ZeppClient) BloodPressureDays(ctx context.Context, oldest, newest string) ([]ZeppBloodPressureEvent, error) {
	preset, _ := V2EventPresetByName("blood-pressure")
	raw, err := c.FetchV2Events(ctx, preset, oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch blood pressure: %w", err)
	}

	return DecodeBloodPressure(raw)
}

// SportLoad fetches daily training load from the watch's sport statistics.
func (c *ZeppClient) SportLoad(ctx context.Context, oldest, newest string) ([]ZeppSportLoad, error) {
	raw, err := c.watchSportStatistics(ctx, "SPORT_LOAD", oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch sport load: %w", err)
	}

	return DecodeSportLoad(raw)
}

// VO2Max fetches VO2 max estimates from the watch's sport statistics.
func (c *ZeppClient) VO2Max(ctx context.Context, oldest, newest string) ([]ZeppVO2Max, error) {
	raw, err := c.watchSportStatistics(ctx, "VO2_MAX", oldest, newest)
	if err != nil {
		return nil, fmt.Errorf("fetch vo2 max: %w", err)
	}

	return DecodeVO2Max(raw)
}

// SecondHeartRateFiles fetches the per-second heart-rate COS file index from
// /users/me/fileInfo/events. Only the file metadata is returned; the actual
// blobs are not downloaded.
func (c *ZeppClient) SecondHeartRateFiles(ctx context.Context, oldest, newest string) ([]ZeppSecondHeartRateFile, error) {
	endpoint, err := SecondHeartRateFilesURL(c.dataHost, oldest, newest)
	if err != nil {
		return nil, err
	}

	raw, err := c.fetchDataHostURL(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch second heart rate files: %w", err)
	}

	return DecodeSecondHeartRateFiles(raw)
}

// SpO2Windows fetches SpO2 ODI windows for a single day from
// /users/{id}/events/dateString.
func (c *ZeppClient) SpO2Windows(ctx context.Context, date, tz string) ([]ZeppSpO2Window, error) {
	endpoint, err := SpO2WindowsURL(c.dataHost, c.userID, date, tz)
	if err != nil {
		return nil, err
	}

	raw, err := c.fetchDataHostURL(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch spo2 windows: %w", err)
	}

	return DecodeSpO2Windows(raw)
}

func (c *ZeppClient) watchSportStatistics(ctx context.Context, statType, oldest, newest string) ([]byte, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	endpoint, err := WatchSportStatisticsURL(c.dataHost, c.userID, statType, oldest, newest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Apptoken", c.appToken)
	req.Header.Set("Appname", "com.xiaomi.hm.health")
	req.Header.Set("Appplatform", "web")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return body, nil
}

// HeartRateEndpoint fetches heart-rate readings from /users/{id}/heartRate.
func (c *ZeppClient) HeartRateEndpoint(ctx context.Context, oldest, newest string) ([]ZeppHeartRateReading, error) {
	endpoint, err := UserHeartRateURL(c.dataHost, c.userID, oldest, newest)
	if err != nil {
		return nil, err
	}

	raw, err := c.fetchDataHostURL(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch heart rate endpoint: %w", err)
	}

	return DecodeHeartRateEndpoint(raw)
}

// WeightRecords fetches weight measurements from /users/{id}/members/-1/weightRecords.
func (c *ZeppClient) WeightRecords(ctx context.Context, oldest, newest string) ([]ZeppWeightRecord, error) {
	endpoint, err := WeightRecordsURL(c.dataHost, c.userID, oldest, newest)
	if err != nil {
		return nil, err
	}

	raw, err := c.fetchDataHostURL(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch weight records: %w", err)
	}

	return DecodeWeightRecords(raw)
}

// ManualData fetches manually entered wellness records from /v1/user/manualData.json.
func (c *ZeppClient) ManualData(ctx context.Context, oldest, newest string) ([]ZeppManualDataEntry, error) {
	endpoint, err := ManualDataURL(c.dataHost, oldest, newest)
	if err != nil {
		return nil, err
	}

	raw, err := c.fetchDataHostURL(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch manual data: %w", err)
	}

	return DecodeManualData(raw)
}

// BloodPressureUser fetches blood-pressure readings from /users/me/bloodPressure.
func (c *ZeppClient) BloodPressureUser(ctx context.Context, oldest, newest string) ([]ZeppBloodPressureEvent, error) {
	endpoint, err := BloodPressureUserURL(c.dataHost, oldest, newest)
	if err != nil {
		return nil, err
	}

	raw, err := c.fetchDataHostURL(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch blood pressure user: %w", err)
	}

	return DecodeBloodPressureUser(raw)
}

func (c *ZeppClient) fetchDataHostURL(ctx context.Context, endpoint string) ([]byte, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Apptoken", c.appToken)
	req.Header.Set("Appname", "com.xiaomi.hm.health")
	req.Header.Set("Appplatform", "web")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return body, nil
}

// doGetEvents performs a GET to the Zepp events endpoint (api-mifit.zepp.com).
// Unlike the data endpoints, this host is regional-agnostic.
func (c *ZeppClient) doGetEvents(ctx context.Context, query url.Values) (*http.Response, error) {
	endpoint := c.eventsBase + "/users/" + c.userID + "/events"
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Apptoken", c.appToken)

	return c.httpClient.Do(req) //nolint:wrapcheck // caller wraps the error
}

// decodeBandDataSummary decodes the base64-packed JSON summary for one day.
// The summary may include stp (steps), slp (sleep), goal, sn (serial), and
// sync (last-sync epoch). Returns an error only if the base64 decode itself
// fails; missing or malformed inner fields are tolerated.
var errEmptySummary = errors.New("empty summary")

func decodeBandDataSummary(encoded, date string) (*BandDataSummary, error) {
	if encoded == "" {
		return nil, errEmptySummary
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	var rawSummary struct {
		Stp  json.RawMessage `json:"stp,omitempty"`
		Slp  json.RawMessage `json:"slp,omitempty"`
		Goal int             `json:"goal,omitempty"`
		SN   string          `json:"sn,omitempty"`
		Sync int64           `json:"sync,omitempty"`
	}

	if err := json.Unmarshal(raw, &rawSummary); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}

	summary := &BandDataSummary{
		Goal:     rawSummary.Goal,
		Serial:   rawSummary.SN,
		LastSync: rawSummary.Sync,
	}

	if len(rawSummary.Stp) > 0 {
		steps, err := decodeSteps(rawSummary.Stp)
		if err == nil {
			summary.Steps = steps
		}
	}

	if len(rawSummary.Slp) > 0 {
		sleep, err := decodeSleep(rawSummary.Slp, date)
		if err == nil {
			summary.Sleep = sleep
		}
	}

	return summary, nil
}

func decodeSteps(raw json.RawMessage) (*BandDataSteps, error) {
	var stepData struct {
		Total    int             `json:"ttl,omitempty"`
		Calories int             `json:"cal,omitempty"`
		Distance int             `json:"dis,omitempty"`
		RunDist  int             `json:"runDist,omitempty"`
		Stages   []BandDataStage `json:"stage,omitempty"`
	}

	if err := json.Unmarshal(raw, &stepData); err != nil {
		return nil, fmt.Errorf("unmarshal steps: %w", err)
	}

	return &BandDataSteps{
		Total:    stepData.Total,
		Calories: stepData.Calories,
		Distance: stepData.Distance,
		RunDist:  stepData.RunDist,
		Stages:   stepData.Stages,
	}, nil
}

func decodeSleep(raw json.RawMessage, date string) (*BandDataSleep, error) {
	var sleepRaw struct {
		StartEpoch   int64                `json:"st,omitempty"`
		EndEpoch     int64                `json:"ed,omitempty"`
		DeepMinutes  int                  `json:"dp,omitempty"`
		LightMinutes int                  `json:"lt,omitempty"`
		Stages       []BandDataSleepStage `json:"stage,omitempty"`
	}

	if err := json.Unmarshal(raw, &sleepRaw); err != nil {
		return nil, fmt.Errorf("unmarshal sleep: %w", err)
	}

	midnightSec, err := parseDateToSecondsUTC(date)
	if err != nil {
		return nil, fmt.Errorf("parse sleep date: %w", err)
	}

	const secondsPerMinute = 60

	for i := range sleepRaw.Stages {
		sleepRaw.Stages[i].StartEpoch = midnightSec + int64(sleepRaw.Stages[i].Start*secondsPerMinute)
		sleepRaw.Stages[i].EndEpoch = midnightSec + int64(sleepRaw.Stages[i].Stop*secondsPerMinute)
	}

	return &BandDataSleep{
		StartEpoch:   sleepRaw.StartEpoch,
		EndEpoch:     sleepRaw.EndEpoch,
		DeepMinutes:  sleepRaw.DeepMinutes,
		LightMinutes: sleepRaw.LightMinutes,
		Stages:       sleepRaw.Stages,
	}, nil
}

// decodeBandDataHeartRate decodes the base64-packed binary blob of 2-byte
// little-endian shorts. Each short is one minute of heart rate; the day
// starts at midnight (local time at the watch). Values 254/255 are sentinels
// for "no read" and "not required" and are preserved as-is.
// dateStr is the ISO date (e.g. "2026-06-03") used to compute per-minute timestamps.
func decodeBandDataHeartRate(encoded, dateStr string) ([]BandDataHeartPoint, error) {
	if encoded == "" {
		return nil, errors.New("empty data_hr")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("data_hr length %d is not a multiple of 2", len(raw))
	}

	midnight := int64(0)

	if dateStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC); err == nil {
			midnight = t.UnixMilli()
		}
	}

	const sentinel = 254
	const millisPerMinute = 60_000

	points := make([]BandDataHeartPoint, 0, len(raw)/2)

	for i := 0; i < len(raw); i += 2 {
		bpm := int(binary.LittleEndian.Uint16(raw[i : i+2]))
		if bpm >= sentinel {
			points = append(points, BandDataHeartPoint{BPM: 0})

			continue
		}

		ts := midnight + int64(i/2)*millisPerMinute
		points = append(points, BandDataHeartPoint{Timestamp: ts, BPM: bpm})
	}

	return points, nil
}

// decodeDeltaEncodedShorts decodes a base64-packed delta-encoded series of
// 2-byte little-endian shorts. The first value is absolute; each subsequent
// value is the delta from the previous value. The result is the cumulative
// sum of deltas (the absolute series).
func decodeDeltaEncodedShorts(encoded string) ([]int, error) {
	if encoded == "" {
		return nil, errors.New("empty delta series")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("delta series length %d is not a multiple of 2", len(raw))
	}

	if len(raw) < 2 {
		return []int{}, nil
	}

	out := make([]int, 0, len(raw)/2)
	cumulative := 0
	first := true

	for i := 0; i < len(raw); i += 2 {
		delta := int(int16(binary.LittleEndian.Uint16(raw[i : i+2]))) //nolint:gosec // intentional signed overflow for delta decoding
		if first {
			cumulative = delta
			first = false
		} else {
			cumulative += delta
		}

		out = append(out, cumulative)
	}

	return out, nil
}

// randomRequestID returns a fresh UUID v4 for use as the `r` query parameter
// required by some Zepp endpoints. We don't need cryptographic strength here.
func randomRequestID() string {
	return generateUUID()
}
