package icu_test

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const (
	err404              = `{"code":404}`
	err500              = `{"code":500}`
	errTokenExpired     = `{"code":3001,"message":"token expired"}` //nolint:gosec // test fixture, not a real credential
	errMissingEventType = `{"code":0,"message":"missing eventType"}`
	eventsPath          = "/users/u1/events"
)

type zeppTestServer struct {
	server  *httptest.Server
	records []zeppRequestRecord
}

type zeppRequestRecord struct {
	Method string
	Path   string
	Query  string
	Header http.Header
}

func newZeppTestServer(handler func(record zeppRequestRecord) (status int, body string)) *zeppTestServer {
	state := &zeppTestServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		record := zeppRequestRecord{
			Method: request.Method,
			Path:   request.URL.Path,
			Query:  request.URL.RawQuery,
			Header: request.Header.Clone(),
		}
		state.records = append(state.records, record)
		status, payload := handler(record)

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(payload))
	})

	state.server = httptest.NewServer(mux)

	return state
}

func (s *zeppTestServer) Close() {
	s.server.Close()
}

func (s *zeppTestServer) Last() zeppRequestRecord {
	return s.records[len(s.records)-1]
}

func newTestZeppClient(srv *zeppTestServer) *icu.ZeppClient {
	auth := &icu.ZeppAuthResult{
		LoginToken:  "tok-abc",
		AppToken:    "test-app-token",
		UserID:      "u1",
		CountryCode: "US",
	}

	return icu.NewZeppClientFromAuth(
		auth,
		icu.WithZeppBaseURL(srv.server.URL),
		icu.WithZeppEventsURL(srv.server.URL),
		icu.WithZeppHTTPClient(srv.server.Client()),
	)
}

func TestZeppClientUserInfoHitsCorrectPath(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/huami.health.getUserInfo.json" {
			return http.StatusNotFound, err404
		}

		if record.Method != http.MethodGet {
			return http.StatusMethodNotAllowed, `{"code":405}`
		}

		return http.StatusOK, `{"code":1,"message":"success","data":{"userId":"u1","nickname":"Tester"}}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	info, err := client.UserInfo(t.Context())
	if err != nil {
		t.Fatalf("UserInfo: %v", err)
	}

	if info.UserID != "u1" || info.Nickname != "Tester" {
		t.Fatalf("info = %+v, want u1/Tester", info)
	}
}

func TestZeppClientUserInfoMissingAuthReturnsError(t *testing.T) {
	t.Parallel()

	auth := &icu.ZeppAuthResult{AppToken: "", UserID: ""}
	client := icu.NewZeppClientFromAuth(auth)

	_, err := client.UserInfo(t.Context())
	if !errors.Is(err, icu.ErrZeppNotAuthenticated) {
		t.Fatalf("expected ErrZeppNotAuthenticated, got %v", err)
	}
}

func TestZeppClientUserInfoSendsApptokenHeader(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Header.Get("Apptoken") != "test-app-token" {
			return http.StatusOK, `{"code":0,"message":"missing token"}`
		}

		return http.StatusOK, `{"code":1,"message":"success","data":{"userId":"u1"}}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	if _, err := client.UserInfo(t.Context()); err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
}

func TestZeppClientBandDataHitsV1Endpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v1/data/band_data.json" {
			return http.StatusNotFound, err404
		}

		if record.Method != http.MethodGet {
			return http.StatusMethodNotAllowed, `{"code":405}`
		}

		if !strings.Contains(record.Query, "from_date=2026-06-01") {
			return http.StatusOK, `{"code":1,"message":"missing from_date"}`
		}

		return http.StatusOK, `{"code":1,"message":"success","data":[]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	records, err := client.BandData(t.Context(), "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("BandData: %v", err)
	}

	if len(records) != 0 {
		t.Fatalf("records = %+v, want empty", records)
	}
}

func TestZeppClientBandDataDecodesSummary(t *testing.T) { //nolint:gocyclo,cyclop // many assertions validate decoded band data
	t.Parallel()

	rawSummary := `{"stp":{"ttl":8500,"cal":2100,"dis":6200,"stage":[{"start":0,"stop":30,"mode":3,"step":1200,"cal":30,"dis":800}]},"slp":{"st":1717200000,"ed":1717230000,"dp":90,"lt":210,"stage":[{"start":0,"stop":30,"mode":5},{"start":30,"stop":120,"mode":4}]},"goal":8000,"sn":"ABC123","sync":1717250000}` //nolint:lll // long JSON fixture

	encodedSummary := base64.StdEncoding.EncodeToString([]byte(rawSummary))

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		body := `{"code":1,"message":"success","data":[{"uid":"u1","data_type":0,"date_time":"2026-06-01","source":256,"uuid":"null","summary":"` + encodedSummary + `","data":"","data_hr":""}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	records, err := client.BandData(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("BandData: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("records length = %d, want 1", len(records))
	}

	day := records[0]

	if day.Date != "2026-06-01" {
		t.Fatalf("date = %q, want 2026-06-01", day.Date)
	}

	if day.SummaryRaw != encodedSummary {
		t.Fatalf("summaryRaw = %q, want %q", day.SummaryRaw, encodedSummary)
	}

	if day.Summary == nil {
		t.Fatalf("summary not decoded")
	}

	if day.Summary.Steps == nil || day.Summary.Steps.Total != 8500 {
		t.Fatalf("steps = %+v, want total=8500", day.Summary.Steps)
	}

	if day.Summary.Sleep == nil {
		t.Fatalf("sleep not decoded")
	}

	if day.Summary.Sleep.DeepMinutes != 90 || day.Summary.Sleep.LightMinutes != 210 {
		t.Fatalf("sleep = %+v, want dp=90 lt=210", day.Summary.Sleep)
	}

	// start/stop in stages are minutes since midnight; decodeSleep converts
	// them to epoch seconds using the day's date (2026-06-01 = midnight 1780272000).
	if len(day.Summary.Sleep.Stages) != 2 {
		t.Fatalf("sleep stages length = %d, want 2", len(day.Summary.Sleep.Stages))
	}

	// midnight 2026-06-01 UTC = 1780272000
	const midnight2026_06_01 int64 = 1780272000

	if day.Summary.Sleep.Stages[0].Start != 0 || day.Summary.Sleep.Stages[0].Stop != 30 {
		t.Fatalf("stage0 minutes = %d-%d, want 0-30", day.Summary.Sleep.Stages[0].Start, day.Summary.Sleep.Stages[0].Stop)
	}

	if day.Summary.Sleep.Stages[0].StartEpoch != midnight2026_06_01 {
		t.Fatalf("stage0 startEpoch = %d, want %d", day.Summary.Sleep.Stages[0].StartEpoch, midnight2026_06_01)
	}

	if day.Summary.Sleep.Stages[0].EndEpoch != midnight2026_06_01+30*60 {
		t.Fatalf("stage0 endEpoch = %d, want %d", day.Summary.Sleep.Stages[0].EndEpoch, midnight2026_06_01+30*60)
	}

	if day.Summary.Sleep.Stages[1].StartEpoch != midnight2026_06_01+30*60 {
		t.Fatalf("stage1 startEpoch = %d, want %d", day.Summary.Sleep.Stages[1].StartEpoch, midnight2026_06_01+30*60)
	}

	if day.Summary.Sleep.Stages[1].EndEpoch != midnight2026_06_01+120*60 {
		t.Fatalf("stage1 endEpoch = %d, want %d", day.Summary.Sleep.Stages[1].EndEpoch, midnight2026_06_01+120*60)
	}

	if day.Summary.Goal != 8000 {
		t.Fatalf("goal = %d, want 8000", day.Summary.Goal)
	}

	if day.Summary.Serial != "ABC123" {
		t.Fatalf("serial = %q, want ABC123", day.Summary.Serial)
	}
}

func TestZeppClientBandDataDecodesHeartRate(t *testing.T) {
	t.Parallel()

	hrBytes := make([]byte, 0, 6)

	for _, bpm := range []uint16{72, 95, 110} {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, bpm)
		hrBytes = append(hrBytes, buf...)
	}

	encodedHR := base64.StdEncoding.EncodeToString(hrBytes)

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		body := `{"code":1,"message":"success","data":[{"uid":"u1","date_time":"2026-06-01","summary":"","data":"","data_hr":"` + encodedHR + `"}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	records, err := client.BandData(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("BandData: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("records length = %d, want 1", len(records))
	}

	hr := records[0].HeartRate
	if len(hr) != 3 {
		t.Fatalf("hr length = %d, want 3", len(hr))
	}

	if hr[0].BPM != 72 || hr[1].BPM != 95 || hr[2].BPM != 110 {
		t.Fatalf("hr = %+v, want [72,95,110]", hr)
	}
}

func TestZeppClientBandDataRejectsBadDate(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, `{"code":0,"data":[]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.BandData(t.Context(), "garbage", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for invalid oldest date")
	}
}

func TestZeppClientSleepDaysExtractsFromBandData(t *testing.T) {
	t.Parallel()

	rawSummary := `{"slp":{"st":1717200000,"ed":1717230000,"dp":90,"lt":210}}`
	encodedSummary := base64.StdEncoding.EncodeToString([]byte(rawSummary))

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		body := `{"code":1,"message":"success","data":[{"date_time":"2026-06-01","summary":"` + encodedSummary + `","data":"","data_hr":""}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	sleeps, err := client.SleepDays(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("SleepDays: %v", err)
	}

	if len(sleeps) != 1 {
		t.Fatalf("sleeps length = %d, want 1", len(sleeps))
	}

	if sleeps[0].DeepMinutes != 90 || sleeps[0].LightMinutes != 210 {
		t.Fatalf("sleep = %+v, want dp=90 lt=210", sleeps[0])
	}
}

func TestZeppClientHeartRateSeriesExtractsFromBandData(t *testing.T) {
	t.Parallel()

	hrBytes := make([]byte, 0, 4)

	for _, bpm := range []uint16{60, 75} {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, bpm)
		hrBytes = append(hrBytes, buf...)
	}

	encodedHR := base64.StdEncoding.EncodeToString(hrBytes)

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		body := `{"code":1,"message":"success","data":[{"date_time":"2026-06-01","summary":"","data":"","data_hr":"` + encodedHR + `"}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	series, err := client.HeartRateSeries(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("HeartRateSeries: %v", err)
	}

	if len(series) != 1 || len(series[0]) != 2 {
		t.Fatalf("series = %+v, want 1 day with 2 points", series)
	}

	if series[0][0].BPM != 60 || series[0][1].BPM != 75 {
		t.Fatalf("hr points = %+v, want [60,75]", series[0])
	}
}

func TestZeppClientStressDaysHitsEventsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != eventsPath {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=all_day_stress") {
			return http.StatusOK, errMissingEventType
		}

		body := `{"items":[{"timestamp":1717200000000,"minStress":"20","maxStress":"80","avgStress":"45","relaxProportion":"40","normalProportion":"30","mediumProportion":"20","highProportion":"10","data":[{"time":1717200000000,"value":30}]}]}` //nolint:lll // long JSON fixture

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	days, err := client.StressDays(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("StressDays: %v", err)
	}

	if len(days) != 1 {
		t.Fatalf("days length = %d, want 1", len(days))
	}

	if days[0].Average != 45 || days[0].Min != 20 || days[0].Max != 80 {
		t.Fatalf("day = %+v, want avg=45 min=20 max=80", days[0])
	}

	if len(days[0].Points) != 1 || days[0].Points[0].Value != 30 {
		t.Fatalf("points = %+v, want 1 point with value=30", days[0].Points)
	}
}

func TestZeppClientSpO2ReadingsHitsEventsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != eventsPath {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=blood_oxygen") {
			return http.StatusOK, errMissingEventType
		}

		body := `{"items":[{"timestamp":1717200000000,"subType":"click","extra":"{\"spo2\":\"97\"}"}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	readings, err := client.SpO2Readings(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("SpO2Readings: %v", err)
	}

	if len(readings) != 1 {
		t.Fatalf("readings length = %d, want 1", len(readings))
	}

	if readings[0].Value != 97 {
		t.Fatalf("value = %v, want 97", readings[0].Value)
	}
}

func TestZeppClientPAIDaysHitsEventsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != eventsPath {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=PaiHealthInfo") {
			return http.StatusOK, errMissingEventType
		}

		body := `{"items":[{"timestamp":1717200000000,"dailyPai":"42.5","totalPai":"120.0","maxHr":"165","restHr":"58","lowZoneMinutes":"30","lowZoneLowerLimit":"1","lowZonePai":"5.0","mediumZoneMinutes":"15","mediumZoneLowerLimit":"50","mediumZonePai":"12.5","highZoneMinutes":"5","highZoneLowerLimit":"85","highZonePai":"25.0"}]}` //nolint:lll // long JSON fixture

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	days, err := client.PAIDays(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("PAIDays: %v", err)
	}

	if len(days) != 1 {
		t.Fatalf("days length = %d, want 1", len(days))
	}

	if days[0].DailyPAI != 42.5 {
		t.Fatalf("dailyPai = %v, want 42.5", days[0].DailyPAI)
	}
}

func TestZeppClientWorkoutsHitsV1SportEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v1/sport/run/history.json" {
			return http.StatusNotFound, err404
		}

		body := `{"code":1,"data":{"next":-1,"summary":[{"trackid":"1780272000","dis":"10000","calorie":"450","end_time":"1780275600","run_time":"3600","avg_pace":"0","avg_heart_rate":"150","type":1}]}}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	summaries, err := client.Workouts(t.Context(), "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("Workouts: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("summaries length = %d, want 1", len(summaries))
	}

	if summaries[0].TrackID != "1780272000" || summaries[0].SportType != 1 {
		t.Fatalf("summary = %+v", summaries[0])
	}
}

func TestZeppClientWorkoutsReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.Workouts(t.Context(), "2026-06-01", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientWorkoutsReturnsErrorOnNonZeroCode(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, errTokenExpired
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.Workouts(t.Context(), "2026-06-01", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for non-zero code")
	}
}

func TestZeppClientWorkoutDecodesDeltaSeries(t *testing.T) {
	t.Parallel()

	hrDelta := make([]byte, 0, 6)

	for _, v := range []int16{150, 10, -5} {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(v)) //nolint:gosec // test helper, v is always small
		hrDelta = append(hrDelta, buf...)
	}

	encodedHR := base64.StdEncoding.EncodeToString(hrDelta)

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v1/sport/run/detail.json" {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "trackid=1717200000") {
			return http.StatusOK, `{"code":1,"message":"missing trackid"}`
		}

		body := `{"code":1,"message":"success","data":{"trackid":"1717200000","type":1,"startTime":1717200000,"endTime":1717203600,"duration":3600,"distance":10000,"calories":450,"avgHR":150,"maxHR":175,"minHR":120,"hr_split":"` + encodedHR + `"}}` //nolint:lll // long JSON fixture

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	detail, err := client.Workout(t.Context(), "1717200000")
	if err != nil {
		t.Fatalf("Workout: %v", err)
	}

	if detail.TrackID != "1717200000" {
		t.Fatalf("trackId = %q, want 1717200000", detail.TrackID)
	}

	wantHR := []int{150, 160, 155}

	if len(detail.HRSeries) != 3 {
		t.Fatalf("hrSeries length = %d, want 3", len(detail.HRSeries))
	}

	for i, want := range wantHR {
		if detail.HRSeries[i] != want {
			t.Fatalf("hrSeries[%d] = %d, want %d", i, detail.HRSeries[i], want)
		}
	}
}

func TestZeppClientNonZeroCodeReturnsError(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, errTokenExpired
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.BandData(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error for non-zero code")
	}

	if !strings.Contains(err.Error(), "token expired") {
		t.Fatalf("error %q does not contain server message", err.Error())
	}
}

func TestZeppClientNon2xxStatusReturnsError(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, `{"code":500,"message":"boom"}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.BandData(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientCountryCodeSelectsRegionalHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		countryCode string
		wantHost    string
	}{
		{"US", "https://api-mifit.huami.com"},
		{"DE", "https://api-mifit-de.huami.com"},
		{"FR", "https://api-mifit-de.huami.com"},
		{"CN", "https://api-mifit-cn.huami.com"},
		{"", "https://api-mifit.huami.com"},
	}

	for _, tc := range cases {
		t.Run(tc.countryCode, func(t *testing.T) {
			t.Parallel()

			auth := &icu.ZeppAuthResult{
				AppToken:    "tok",
				UserID:      "u1",
				CountryCode: tc.countryCode,
			}

			client := icu.NewZeppClientFromAuth(auth)
			if got := client.DataHostForTest(); got != tc.wantHost {
				t.Fatalf("dataHost = %q, want %q", got, tc.wantHost)
			}
		})
	}
}

func TestZeppClientWorkoutReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.Workout(t.Context(), "1717200000")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientWorkoutReturnsErrorOnNonZeroCode(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, errTokenExpired
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.Workout(t.Context(), "1717200000")
	if err == nil {
		t.Fatalf("expected error for non-zero code")
	}
}

func TestZeppClientUserInfoReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.UserInfo(t.Context())
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientSpO2ReadingsReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.SpO2Readings(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientPAIDaysReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.PAIDays(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}
