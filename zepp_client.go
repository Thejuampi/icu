package icu

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
			return err
		}

		v, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}

		*f = flexInt(v)

		return nil
	}

	var v int

	if err := json.Unmarshal(data, &v); err != nil {
		return err
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
			return err
		}

		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}

		*f = flexFloat(v)

		return nil
	}

	var v float64

	if err := json.Unmarshal(data, &v); err != nil {
		return err
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
	if query != nil && len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("apptoken", c.appToken)
	req.Header.Set("appname", "com.xiaomi.hm.health")
	req.Header.Set("appplatform", "web")

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

	if resp.StatusCode != 200 {
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
	for _, d := range raw.Data {
		day := BandDataDay{
			Date:       d.DateTime,
			UID:        d.UID,
			DataType:   d.DataType,
			Source:     d.Source,
			UUID:       d.UUID,
			SummaryRaw: d.Summary,
			DataRaw:    d.Data,
			DataHRRaw:  d.DataHR,
		}

		if summary, err := decodeBandDataSummary(d.Summary); err == nil {
			day.Summary = summary
		}

		if points, err := decodeBandDataHeartRate(d.DataHR, d.DateTime); err == nil {
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
	for _, d := range days {
		if d.Summary == nil || d.Summary.Sleep == nil {
			continue
		}

		sleeps = append(sleeps, *d.Summary.Sleep)
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
	for _, d := range days {
		series = append(series, d.HeartRate)
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

	if resp.StatusCode != 200 {
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

	if resp.StatusCode != 200 {
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
		r := SpO2Reading{
			Timestamp: item.Timestamp,
			SubType:   item.SubType,
			ODI:       float64(item.ODI),
			ODIScore:  float64(item.Score),
			Date:      time.UnixMilli(item.Timestamp).UTC().Format("2006-01-02"),
		}

		if item.Extra != "" {
			var extra map[string]any
			if err := json.Unmarshal([]byte(item.Extra), &extra); err == nil {
				if v, ok := extra["spo2"].(float64); ok {
					r.Value = v
				}

				if v, ok := extra["spo2_decrease"].(float64); ok {
					r.SpO2Decrease = v
				}
			}
		}

		out = append(out, r)
	}

	return out, nil
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

	if resp.StatusCode != 200 {
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

// Workouts fetches the list of workouts in the date range. The result is
// the first page only; if the API returns a `next` cursor, callers can pass
// it to WorkoutsPage for the next page.
func (c *ZeppClient) Workouts(ctx context.Context, oldest, newest string) ([]WorkoutSummary, error) {
	items, _, err := c.WorkoutsPage(ctx, oldest, newest, "", "")
	if err != nil {
		return nil, err
	}

	return items, nil
}

// WorkoutsPage fetches a single page of workouts. startTrackID/stopTrackID
// can be passed from a previous page's `next` cursor to page backwards.
func (c *ZeppClient) WorkoutsPage(ctx context.Context, oldest, newest, startTrackID, stopTrackID string) ([]WorkoutSummary, string, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, "", err
	}

	from, err := parseDateToMillis(oldest)
	if err != nil {
		return nil, "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return nil, "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"query_type":  {"summary"},
		"device_type": {"android_phone"},
		"userid":      {c.userID},
		"from_date":   {oldest},
		"to_date":     {newest},
		"from":        {strconv.FormatInt(from/1000, 10)},
		"to":          {strconv.FormatInt(to/1000, 10)},
		"source":      {"run.mifit.huami.com"},
	}

	if startTrackID != "" {
		query.Set("startTrackId", startTrackID)
	}

	if stopTrackID != "" {
		query.Set("stopTrackId", stopTrackID)
	}

	resp, err := c.doGet(ctx, zeppSportHistoryPath, query)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("%w: status %d", ErrZeppServer, resp.StatusCode)
	}

	var raw struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
		Data    []struct {
			TrackID   string `json:"trackid"`
			Date      string `json:"date"`
			Type      int    `json:"type"`
			StartTime int64  `json:"startTime"`
			EndTime   int64  `json:"endTime"`
			Duration  int    `json:"duration"`
			Distance  int    `json:"distance"`
			Calories  int    `json:"calories"`
			AvgHR     int    `json:"avgHR"`
			MaxHR     int    `json:"maxHR"`
			MinHR     int    `json:"minHR"`
			AvgPace   int    `json:"avgPace"`
			MaxPace   int    `json:"maxPace"`
			AvgPower  int    `json:"avgPower"`
			MaxPower  int    `json:"maxPower"`
			Steps     int    `json:"step"`
			Next      string `json:"next"`
		} `json:"data"`
		Next string `json:"next"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	if raw.Code != 1 && raw.Code != 0 {
		return nil, "", fmt.Errorf("%w: %s", ErrZeppServer, raw.Message)
	}

	out := make([]WorkoutSummary, 0, len(raw.Data))
	for _, item := range raw.Data {
		summary := WorkoutSummary{
			TrackID:   item.TrackID,
			SportType: item.Type,
			StartTime: item.StartTime,
			EndTime:   item.EndTime,
			Duration:  item.Duration,
			Distance:  item.Distance,
			Calories:  item.Calories,
			AvgHR:     item.AvgHR,
			MaxHR:     item.MaxHR,
			MinHR:     item.MinHR,
			AvgPace:   item.AvgPace,
			MaxPace:   item.MaxPace,
			AvgPower:  item.AvgPower,
			MaxPower:  item.MaxPower,
			Steps:     item.Steps,
		}

		out = append(out, summary)
	}

	next := raw.Next
	if next == "" && len(raw.Data) > 0 {
		next = raw.Data[len(raw.Data)-1].Next
	}

	return out, next, nil
}

// Workout fetches the full per-second detail for a single workout.
// The returned `WorkoutDetail` includes the decoded (absolute) numeric series.
func (c *ZeppClient) Workout(ctx context.Context, trackID string) (WorkoutDetail, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return WorkoutDetail{}, err
	}

	query := url.Values{
		"trackid": {trackID},
		"source":  {"run.mifit.huami.com"},
	}

	resp, err := c.doGet(ctx, zeppSportDetailPath, query)
	if err != nil {
		return WorkoutDetail{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

	d := raw.Data

	detail := WorkoutDetail{
		TrackID:     d.TrackID,
		SportType:   d.Type,
		StartTime:   d.StartTime,
		EndTime:     d.EndTime,
		Duration:    d.Duration,
		Distance:    d.Distance,
		Calories:    d.Calories,
		AvgHR:       d.AvgHR,
		MaxHR:       d.MaxHR,
		MinHR:       d.MinHR,
		AvgPower:    d.AvgPower,
		MaxPower:    d.MaxPower,
		AvgPace:     d.AvgPace,
		MaxAltitude: d.MaxAltitude,
		MinAltitude: d.MinAltitude,
		Ascent:      d.Ascent,
		Descent:     d.Descent,
		Steps:       d.Steps,
	}

	if decoded, err := decodeDeltaEncodedShorts(d.HRDelta); err == nil {
		detail.HRSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(d.PaceDelta); err == nil {
		detail.PaceSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(d.AltDelta); err == nil {
		detail.AltSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(d.PowerDelta); err == nil {
		detail.PowerSeries = decoded
	}

	if decoded, err := decodeDeltaEncodedShorts(d.StepDelta); err == nil {
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

	if resp.StatusCode != 200 {
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

	d := raw.Data

	return ZeppUserInfo{
		UserID:   d.UserID,
		Nickname: d.Nickname,
		Email:    d.Email,
		Gender:   d.Gender,
		Height:   d.Height,
		Weight:   d.Weight,
		Birthday: d.Birthday,
		Region:   d.Region,
	}, nil
}

// doGetEvents performs a GET to the Zepp events endpoint (api-mifit.zepp.com).
// Unlike the data endpoints, this host is regional-agnostic.
func (c *ZeppClient) doGetEvents(ctx context.Context, query url.Values) (*http.Response, error) {
	endpoint := c.eventsBase + "/users/" + c.userID + "/events"
	if query != nil && len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("apptoken", c.appToken)

	return c.httpClient.Do(req)
}

// decodeBandDataSummary decodes the base64-packed JSON summary for one day.
// The summary may include stp (steps), slp (sleep), goal, sn (serial), and
// sync (last-sync epoch). Returns an error only if the base64 decode itself
// fails; missing or malformed inner fields are tolerated.
func decodeBandDataSummary(encoded string) (*BandDataSummary, error) {
	if encoded == "" {
		return nil, errors.New("empty summary")
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
		sleep, err := decodeSleep(rawSummary.Slp)
		if err == nil {
			summary.Sleep = sleep
		}
	}

	return summary, nil
}

func decodeSteps(raw json.RawMessage) (*BandDataSteps, error) {
	var s struct {
		Total    int             `json:"ttl,omitempty"`
		Calories int             `json:"cal,omitempty"`
		Distance int             `json:"dis,omitempty"`
		RunDist  int             `json:"runDist,omitempty"`
		Stages   []BandDataStage `json:"stage,omitempty"`
	}

	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}

	return &BandDataSteps{
		Total:    s.Total,
		Calories: s.Calories,
		Distance: s.Distance,
		RunDist:  s.RunDist,
		Stages:   s.Stages,
	}, nil
}

func decodeSleep(raw json.RawMessage) (*BandDataSleep, error) {
	var s struct {
		StartEpoch   int64                `json:"st,omitempty"`
		EndEpoch     int64                `json:"ed,omitempty"`
		DeepMinutes  int                  `json:"dp,omitempty"`
		LightMinutes int                  `json:"lt,omitempty"`
		Stages       []BandDataSleepStage `json:"stage,omitempty"`
	}

	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}

	return &BandDataSleep{
		StartEpoch:   s.StartEpoch,
		EndEpoch:     s.EndEpoch,
		DeepMinutes:  s.DeepMinutes,
		LightMinutes: s.LightMinutes,
		Stages:       s.Stages,
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
		if t, err := time.ParseInLocation("2006-01-02", dateStr, time.Local); err == nil {
			midnight = t.UnixMilli()
		}
	}

	const sentinel = 254
	points := make([]BandDataHeartPoint, 0, len(raw)/2)

	for i := 0; i < len(raw); i += 2 {
		bpm := int(binary.LittleEndian.Uint16(raw[i : i+2]))
		if bpm >= sentinel {
			points = append(points, BandDataHeartPoint{BPM: 0})
			continue
		}

		ts := midnight + int64(i/2)*60_000
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
		delta := int(int16(binary.LittleEndian.Uint16(raw[i : i+2])))
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

func parseDateToMillis(dateStr string) (int64, error) {
	t, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return 0, err
	}

	return t.UnixMilli(), nil
}

// ParseZeppDateToMillisForTest is a test-friendly alias of parseDateToMillis.
// It exists so that the public test package can verify date parsing.
func ParseZeppDateToMillisForTest(dateStr string) (int64, error) {
	return parseDateToMillis(dateStr)
}

func parseDateEndOfDayMillis(dateStr string) (int64, error) {
	t, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return 0, err
	}

	return t.Add(24*time.Hour - time.Millisecond).UnixMilli(), nil
}

// randomRequestID returns a fresh UUID v4 for use as the `r` query parameter
// required by some Zepp endpoints. We don't need cryptographic strength here.
func randomRequestID() string {
	return generateUUID()
}

var _ = strings.TrimSpace
