package icu_test

import (
	"context"
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
	zeppV2EventRaw = `{"items":[{"timestamp":1780272000000,"value":42.5}]}`
	zeppBPRaw      = `{"items":[{"timestamp":1780272000000,"systolic":120,"diastolic":80}]}`
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

	summaries, err := client.Workouts(t.Context(), "run", "2026-06-01", "2026-06-07")
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

	_, err := client.Workouts(t.Context(), "run", "2026-06-01", "2026-06-07")
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

	_, err := client.Workouts(t.Context(), "run", "2026-06-01", "2026-06-07")
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

	detail, err := client.Workout(t.Context(), "run", "1717200000")
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

	_, err := client.Workout(t.Context(), "run", "1717200000")
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

	_, err := client.Workout(t.Context(), "run", "1717200000")
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

func TestV2EventPresetsIncludesExpectedEntries(t *testing.T) {
	t.Parallel()

	presets := icu.V2EventPresets()
	if len(presets) == 0 {
		t.Fatalf("presets is empty")
	}

	if _, ok := presets["hrv-sdnn"]; !ok {
		t.Fatalf("missing hrv-sdnn preset")
	}
}

func TestV2EventPresetByNameReturnsKnownPreset(t *testing.T) {
	t.Parallel()

	var preset icu.V2EventPreset
	var ok bool
	preset, ok = icu.V2EventPresetByName("body-battery")
	if !ok {
		t.Fatalf("expected body-battery preset")
	}

	if preset.EventType != "Charge" || preset.SubType != "real_data" {
		t.Fatalf("preset = %+v", preset)
	}
}

func TestV2EventPresetByNameReturnsHybridChargePreset(t *testing.T) {
	t.Parallel()

	var preset icu.V2EventPreset
	var ok bool
	preset, ok = icu.V2EventPresetByName("hybridcharge")
	if !ok {
		t.Fatalf("expected hybridcharge preset")
	}

	if preset.EventType != "Charge" || preset.SubType != "insight_data" {
		t.Fatalf("preset = %+v", preset)
	}
}

func TestV2EventPresetByNameReturnsFalseForUnknown(t *testing.T) {
	t.Parallel()

	_, ok := icu.V2EventPresetByName("not-real")
	if ok {
		t.Fatalf("expected unknown preset to be false")
	}
}

func TestV2EventsURLBuildsCorrectPathAndQuery(t *testing.T) {
	t.Parallel()

	preset := icu.V2EventPreset{EventType: "hrv_sdnn", SubType: "real_data"}
	got, err := icu.V2EventsURL("https://api-mifit.huami.com", preset, "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("V2EventsURL: %v", err)
	}

	if !strings.Contains(got, "/v2/users/me/events?") {
		t.Fatalf("url missing v2 path: %s", got)
	}

	if !strings.Contains(got, "eventType=hrv_sdnn") {
		t.Fatalf("url missing eventType: %s", got)
	}

	if !strings.Contains(got, "subType=real_data") {
		t.Fatalf("url missing subType: %s", got)
	}

	if !strings.Contains(got, "limit=1000") {
		t.Fatalf("url missing limit: %s", got)
	}
}

func TestV2EventsURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	preset := icu.V2EventPreset{EventType: "hrv_sdnn", SubType: "real_data"}
	_, err := icu.V2EventsURL("https://api-mifit.huami.com", preset, "garbage", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestDecodeV2EventsExtractsCommonFields(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"type":"hrv_sdnn","subType":"real_data","value":"45.5","extra_field":"x"}]}`

	events, err := icu.DecodeV2Events([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeV2Events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1", len(events))
	}

	ev := events[0]
	if ev.Timestamp != 1780272000000 {
		t.Fatalf("timestamp = %d, want 1780272000000", ev.Timestamp)
	}

	if ev.Date != "2026-06-01" {
		t.Fatalf("date = %q, want 2026-06-01", ev.Date)
	}

	if ev.Type != "hrv_sdnn" {
		t.Fatalf("type = %q, want hrv_sdnn", ev.Type)
	}

	if ev.SubType != "real_data" {
		t.Fatalf("subtype = %q, want real_data", ev.SubType)
	}

	if ev.Value != 45.5 {
		t.Fatalf("value = %v, want 45.5", ev.Value)
	}

	if ev.Extra["extra_field"] != "x" {
		t.Fatalf("extra = %+v", ev.Extra)
	}
}

func TestDecodeV2EventsReturnsEmptyForEmptyInput(t *testing.T) {
	t.Parallel()

	events, err := icu.DecodeV2Events(nil)
	if err != nil {
		t.Fatalf("DecodeV2Events: %v", err)
	}

	if events != nil {
		t.Fatalf("events = %+v, want nil", events)
	}
}

func TestZeppClientFetchV2EventsHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v2/users/me/events" {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=Charge") {
			return http.StatusOK, `{"code":0,"message":"missing eventType"}`
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"type":"Charge","subType":"real_data","value":85}]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	preset := icu.V2EventPreset{EventType: "Charge", SubType: "real_data"}
	raw, err := client.FetchV2Events(t.Context(), preset, "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("FetchV2Events: %v", err)
	}

	events, err := icu.DecodeV2Events(raw)
	if err != nil {
		t.Fatalf("DecodeV2Events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1", len(events))
	}

	if events[0].Value != 85 {
		t.Fatalf("value = %v, want 85", events[0].Value)
	}
}

func TestZeppClientFetchV2EventsSendsApptokenHeader(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Header.Get("Apptoken") != "test-app-token" {
			return http.StatusUnauthorized, `{"code":3001,"message":"missing token"}`
		}

		return http.StatusOK, `{"items":[]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.FetchV2Events(t.Context(), icu.V2EventPreset{EventType: "Charge", SubType: "real_data"}, "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("FetchV2Events: %v", err)
	}
}

func TestZeppClientFetchV2EventsMissingAuthReturnsError(t *testing.T) {
	t.Parallel()

	auth := &icu.ZeppAuthResult{AppToken: "", UserID: ""}
	client := icu.NewZeppClientFromAuth(auth)

	_, err := client.FetchV2Events(t.Context(), icu.V2EventPreset{}, "2026-06-01", "2026-06-01")
	if !errors.Is(err, icu.ErrZeppNotAuthenticated) {
		t.Fatalf("expected ErrZeppNotAuthenticated, got %v", err)
	}
}

func TestZeppClientFetchV2EventsReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.FetchV2Events(t.Context(), icu.V2EventPreset{EventType: "Charge", SubType: "real_data"}, "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientFetchV2EventsUsesEventsHost(t *testing.T) {
	t.Parallel()

	dataServer := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path == "/v2/users/me/events" {
			return http.StatusTeapot, `{"code":418}`
		}

		return http.StatusNotFound, err404
	})
	t.Cleanup(dataServer.Close)

	eventsServer := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v2/users/me/events" {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=Charge") || !strings.Contains(record.Query, "subType=insight_data") {
			return http.StatusBadRequest, errMissingEventType
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":90}]}`
	})
	t.Cleanup(eventsServer.Close)

	auth := &icu.ZeppAuthResult{AppToken: "test-app-token", UserID: "u1", CountryCode: "US"}
	client := icu.NewZeppClientFromAuth(
		auth,
		icu.WithZeppBaseURL(dataServer.server.URL),
		icu.WithZeppEventsURL(eventsServer.server.URL),
	)

	days, err := client.HybridChargeDays(t.Context(), "2026-06-01", "2026-06-01")
	if err != nil {
		t.Fatalf("HybridChargeDays: %v", err)
	}

	if len(days) != 1 || days[0].Score != 90 {
		t.Fatalf("days = %+v, want one score 90 from events host", days)
	}
}

func TestDecodeHRV(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeHRV([]byte(raw), "sdnn")
	if err != nil {
		t.Fatalf("DecodeHRV: %v", err)
	}

	if len(events) != 1 || events[0].Metric != "sdnn" || events[0].Value != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeReadiness(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeReadiness([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeReadiness: %v", err)
	}

	if len(events) != 1 || events[0].Score != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeBodyBattery(t *testing.T) {
	t.Parallel()

	var raw string = zeppV2EventRaw

	var events []icu.ZeppBodyBatteryEvent
	var err error
	events, err = icu.DecodeBodyBattery([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeBodyBattery: %v", err)
	}

	if len(events) != 1 || events[0].Level != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeHybridCharge(t *testing.T) {
	t.Parallel()

	var raw string = `{"items":[{"timestamp":1780272000000,"value":90,"phase":"wake","source":"watch"}]}`

	var events []icu.ZeppHybridChargeEvent
	var err error
	events, err = icu.DecodeHybridCharge([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeHybridCharge: %v", err)
	}

	if len(events) != 1 || events[0].Score != 90 || events[0].Extra["phase"] != "wake" {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeHealthSummary(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"extra":"z"}]}`

	events, err := icu.DecodeHealthSummary([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeHealthSummary: %v", err)
	}

	if len(events) != 1 || events[0].Extra["extra"] != "z" {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeMood(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeMood([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeMood: %v", err)
	}

	if len(events) != 1 || events[0].Mood != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeSkinTemp(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeSkinTemp([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeSkinTemp: %v", err)
	}

	if len(events) != 1 || events[0].Delta != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func newZeppV2EventTestServer(t *testing.T, eventType, subType string) *icu.ZeppClient {
	t.Helper()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v2/users/me/events" {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType="+eventType) || !strings.Contains(record.Query, "subType="+subType) {
			return http.StatusOK, `{"code":0,"message":"missing eventType"}`
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":42}]}`
	})
	t.Cleanup(srv.Close)

	return newTestZeppClient(srv)
}

func TestZeppClientHRVSDNNDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "hrv_sdnn", "real_data")
	if _, err := client.HRVSDNNDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("HRVSDNNDays: %v", err)
	}
}

func TestZeppClientHRVRMSSDDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "HRVRMSSD", "real_data")
	if _, err := client.HRVRMSSDDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("HRVRMSSDDays: %v", err)
	}
}

func TestZeppClientReadinessDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "readiness", "watch_score")
	if _, err := client.ReadinessDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("ReadinessDays: %v", err)
	}
}

func TestZeppClientBodyBatteryDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	var client *icu.ZeppClient = newZeppV2EventTestServer(t, "Charge", "real_data")
	if _, err := client.BodyBatteryDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("BodyBatteryDays: %v", err)
	}
}

func TestZeppClientHybridChargeDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	var client *icu.ZeppClient = newZeppV2EventTestServer(t, "Charge", "insight_data")
	if _, err := client.HybridChargeDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("HybridChargeDays: %v", err)
	}
}

func TestZeppClientHealthSummaryDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "DailyHealth", "summary")
	if _, err := client.HealthSummaryDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("HealthSummaryDays: %v", err)
	}
}

func TestZeppClientMoodDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "Emotion", "real_data")
	if _, err := client.MoodDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("MoodDays: %v", err)
	}
}

func TestZeppClientSkinTempDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "skinTemp", "real_data")
	if _, err := client.SkinTempDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("SkinTempDays: %v", err)
	}
}

func TestDecodeStressMinute(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeStressMinute([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeStressMinute: %v", err)
	}

	if len(events) != 1 || events[0].Stress != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeRespiratoryRate(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeRespiratoryRate([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeRespiratoryRate: %v", err)
	}

	if len(events) != 1 || events[0].Rate != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeBloodPressure(t *testing.T) {
	t.Parallel()

	raw := zeppBPRaw

	events, err := icu.DecodeBloodPressure([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeBloodPressure: %v", err)
	}

	if len(events) != 1 || events[0].Systolic != 120 || events[0].Diastolic != 80 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeBloodPressureUser(t *testing.T) {
	t.Parallel()

	raw := zeppBPRaw

	events, err := icu.DecodeBloodPressureUser([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeBloodPressureUser: %v", err)
	}

	if len(events) != 1 || events[0].Systolic != 120 || events[0].Diastolic != 80 {
		t.Fatalf("events = %+v", events)
	}
}

func TestZeppClientStressMinuteDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "Charge", "stress_data")
	if _, err := client.StressMinuteDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("StressMinuteDays: %v", err)
	}
}

func TestZeppClientRespiratoryRateDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "RespiratoryRate", "real_data")
	if _, err := client.RespiratoryRateDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("RespiratoryRateDays: %v", err)
	}
}

func TestZeppClientBloodPressureDaysHitsV2Endpoint(t *testing.T) {
	t.Parallel()

	client := newZeppV2EventTestServer(t, "blood_pressure", "real_data")
	if _, err := client.BloodPressureDays(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("BloodPressureDays: %v", err)
	}
}

func TestZeppClientBloodPressureUserHitsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/users/me/bloodPressure" {
			return http.StatusNotFound, err404
		}

		return http.StatusOK, zeppBPRaw
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.BloodPressureUser(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("BloodPressureUser: %v", err)
	}
}

func TestZeppClientBloodPressureUserMissingAuthReturnsError(t *testing.T) {
	t.Parallel()

	auth := &icu.ZeppAuthResult{AppToken: "", UserID: ""}
	client := icu.NewZeppClientFromAuth(auth)

	_, err := client.BloodPressureUser(t.Context(), "2026-06-01", "2026-06-01")
	if !errors.Is(err, icu.ErrZeppNotAuthenticated) {
		t.Fatalf("expected ErrZeppNotAuthenticated, got %v", err)
	}
}

func TestZeppClientBloodPressureUserRejectsBadDate(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, zeppBPRaw
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	_, err := client.BloodPressureUser(t.Context(), "garbage", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestZeppClientBloodPressureUserRejectsBadNewestDate(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, zeppBPRaw
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	_, err := client.BloodPressureUser(t.Context(), "2026-06-01", "garbage")
	if err == nil {
		t.Fatalf("expected error for invalid newest date")
	}
}

func TestZeppClientBloodPressureUserReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	_, err := client.BloodPressureUser(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestWatchSportStatisticsURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.WatchSportStatisticsURL("https://api-mifit.huami.com", "u1", "SPORT_LOAD", "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("WatchSportStatisticsURL: %v", err)
	}

	if !strings.Contains(got, "/v2/watch/users/u1/WatchSportStatistics/SPORT_LOAD?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "from=") || !strings.Contains(got, "to=") {
		t.Fatalf("url missing query: %s", got)
	}
}

func TestDecodeSportLoad(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeSportLoad([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeSportLoad: %v", err)
	}

	if len(events) != 1 || events[0].Load != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeVO2Max(t *testing.T) {
	t.Parallel()

	raw := zeppV2EventRaw

	events, err := icu.DecodeVO2Max([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeVO2Max: %v", err)
	}

	if len(events) != 1 || events[0].VO2Max != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestZeppClientSportLoadHitsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v2/watch/users/u1/WatchSportStatistics/SPORT_LOAD" {
			return http.StatusNotFound, err404
		}

		return http.StatusOK, zeppV2EventRaw
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.SportLoad(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("SportLoad: %v", err)
	}
}

func TestZeppClientVO2MaxHitsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/v2/watch/users/u1/WatchSportStatistics/VO2_MAX" {
			return http.StatusNotFound, err404
		}

		return http.StatusOK, zeppV2EventRaw
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.VO2Max(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("VO2Max: %v", err)
	}
}

func TestWatchSportStatisticsURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.WatchSportStatisticsURL("https://api-mifit.huami.com", "u1", "SPORT_LOAD", "garbage", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestUserHeartRateURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.UserHeartRateURL("https://api-mifit.huami.com", "u1", "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("UserHeartRateURL: %v", err)
	}

	if !strings.Contains(got, "/users/u1/heartRate?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "startTime=") || !strings.Contains(got, "endTime=") {
		t.Fatalf("url missing query: %s", got)
	}
}

func TestUserHeartRateURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.UserHeartRateURL("https://api-mifit.huami.com", "u1", "garbage", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestWeightRecordsURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.WeightRecordsURL("https://api-mifit.huami.com", "u1", "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("WeightRecordsURL: %v", err)
	}

	if !strings.Contains(got, "/users/u1/members/-1/weightRecords?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "fromTime=") || !strings.Contains(got, "toTime=") {
		t.Fatalf("url missing query: %s", got)
	}
}

func TestWeightRecordsURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.WeightRecordsURL("https://api-mifit.huami.com", "u1", "2026-06-01", "garbage")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestManualDataURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.ManualDataURL("https://api-mifit.huami.com", "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("ManualDataURL: %v", err)
	}

	if !strings.Contains(got, "/v1/user/manualData.json?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "from=") || !strings.Contains(got, "to=") {
		t.Fatalf("url missing query: %s", got)
	}
}

func TestManualDataURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.ManualDataURL("https://api-mifit.huami.com", "not-a-date", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestBloodPressureUserURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.BloodPressureUserURL("https://api-mifit.huami.com", "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("BloodPressureUserURL: %v", err)
	}

	if !strings.Contains(got, "/users/me/bloodPressure?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "from=") || !strings.Contains(got, "to=") {
		t.Fatalf("url missing query: %s", got)
	}
}

func TestBloodPressureUserURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.BloodPressureUserURL("https://api-mifit.huami.com", "2026-06-01", "bad")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestSecondHeartRateFilesURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.SecondHeartRateFilesURL("https://api-mifit.huami.com", "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("SecondHeartRateFilesURL: %v", err)
	}

	if !strings.Contains(got, "/users/me/fileInfo/events?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "eventType=second_heart_rate") || !strings.Contains(got, "subType=real_data") {
		t.Fatalf("url missing event query: %s", got)
	}
}

func TestSecondHeartRateFilesURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.SecondHeartRateFilesURL("https://api-mifit.huami.com", "bad", "2026-06-07")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestSpO2WindowsURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got, err := icu.SpO2WindowsURL("https://api-mifit.huami.com", "u1", "2026-06-07", "UTC")
	if err != nil {
		t.Fatalf("SpO2WindowsURL: %v", err)
	}

	if !strings.Contains(got, "/users/u1/events/dateString?") {
		t.Fatalf("url missing path: %s", got)
	}

	if !strings.Contains(got, "eventType=blood_oxygen") || !strings.Contains(got, "subType=odi") {
		t.Fatalf("url missing event query: %s", got)
	}
}

func TestSpO2WindowsURLRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := icu.SpO2WindowsURL("https://api-mifit.huami.com", "u1", "not-a-date", "UTC")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestSpO2WindowsURLRejectsInvalidTimezone(t *testing.T) {
	t.Parallel()

	_, err := icu.SpO2WindowsURL("https://api-mifit.huami.com", "u1", "2026-06-07", "Mars/Phobos")
	if err == nil {
		t.Fatalf("expected error for invalid timezone")
	}
}

func TestSportHistoryURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got := icu.SportHistoryURL("https://api-mifit.huami.com", "walking")
	if got != "https://api-mifit.huami.com/v1/sport/walking/history.json" {
		t.Fatalf("url = %s", got)
	}
}

func TestSportDetailURLBuildsCorrectPath(t *testing.T) {
	t.Parallel()

	got := icu.SportDetailURL("https://api-mifit.huami.com", "ride")
	if got != "https://api-mifit.huami.com/v1/sport/ride/detail.json" {
		t.Fatalf("url = %s", got)
	}
}

func TestSportNameToSegmentMapsKnownSports(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want string
	}{
		{"run", "run"},
		{"walking", "walking"},
		{"cycling", "ride"},
		{"ride", "ride"},
		{"swimming", "swimming"},
	}

	for _, tc := range cases {
		segment, ok := icu.SportNameToSegment(tc.name)
		if !ok || segment != tc.want {
			t.Fatalf("%s = %s/%v, want %s", tc.name, segment, ok, tc.want)
		}
	}
}

func TestSportNameToSegmentReturnsFalseForUnknownSport(t *testing.T) {
	t.Parallel()

	_, ok := icu.SportNameToSegment("quidditch")
	if ok {
		t.Fatalf("expected false for unknown sport")
	}
}

func TestZeppClientSportLoadReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	_, err := client.SportLoad(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestZeppClientVO2MaxReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusInternalServerError, err500
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	_, err := client.VO2Max(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected error on 5xx")
	}
}

func TestDecodeBloodPressureUserRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := icu.DecodeBloodPressureUser([]byte("not-json"))
	if err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}

func TestDecodeBloodPressureUsesAliasFields(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"highPressure":130,"lowPressure":85}]}`

	events, err := icu.DecodeBloodPressure([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeBloodPressure: %v", err)
	}

	if len(events) != 1 || events[0].Systolic != 130 || events[0].Diastolic != 85 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeBloodPressureParsesStringValues(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"systolic":"125","diastolic":"82"}]}`

	events, err := icu.DecodeBloodPressure([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeBloodPressure: %v", err)
	}

	if len(events) != 1 || events[0].Systolic != 125 || events[0].Diastolic != 82 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeBloodPressureKeepsNonBPFieldsInExtra(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"systolic":120,"diastolic":80,"deviceId":"abc"}]}`

	events, err := icu.DecodeBloodPressure([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeBloodPressure: %v", err)
	}

	if len(events) != 1 || events[0].Extra["deviceId"] != "abc" {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeV2EventsHandlesEmptyItems(t *testing.T) {
	t.Parallel()

	events, err := icu.DecodeV2Events([]byte(`{"items":[]}`))
	if err != nil {
		t.Fatalf("DecodeV2Events: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("events = %+v, want empty", events)
	}
}

func TestDecodeHeartRateEndpoint(t *testing.T) {
	t.Parallel()

	events, err := icu.DecodeHeartRateEndpoint([]byte(zeppV2EventRaw))
	if err != nil {
		t.Fatalf("DecodeHeartRateEndpoint: %v", err)
	}

	if len(events) != 1 || events[0].HR != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeWeightRecords(t *testing.T) {
	t.Parallel()

	events, err := icu.DecodeWeightRecords([]byte(zeppV2EventRaw))
	if err != nil {
		t.Fatalf("DecodeWeightRecords: %v", err)
	}

	if len(events) != 1 || events[0].Weight != 42.5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeManualData(t *testing.T) {
	t.Parallel()

	events, err := icu.DecodeManualData([]byte(zeppV2EventRaw))
	if err != nil {
		t.Fatalf("DecodeManualData: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
}

func TestDecodeSecondHeartRateFiles(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"url":"https://cos.example/1.zip","size":1234}]}`

	files, err := icu.DecodeSecondHeartRateFiles([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeSecondHeartRateFiles: %v", err)
	}

	if len(files) != 1 || files[0].URL != "https://cos.example/1.zip" {
		t.Fatalf("files = %+v", files)
	}

	if files[0].Extra["size"] != float64(1234) {
		t.Fatalf("extra = %+v", files[0].Extra)
	}
}

func TestDecodeSpO2Windows(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1780272000000,"spo2":98,"duration":60}]}`

	windows, err := icu.DecodeSpO2Windows([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeSpO2Windows: %v", err)
	}

	if len(windows) != 1 {
		t.Fatalf("windows = %+v", windows)
	}
}

func TestZeppClientSecondHeartRateFilesHitsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/users/me/fileInfo/events" {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=second_heart_rate") || !strings.Contains(record.Query, "subType=real_data") {
			return http.StatusOK, errMissingEventType
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"url":"https://cos.example/1.zip"}]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.SecondHeartRateFiles(t.Context(), "2026-06-01", "2026-06-01"); err != nil {
		t.Fatalf("SecondHeartRateFiles: %v", err)
	}
}

func TestZeppClientSpO2WindowsHitsEndpoint(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		if record.Path != "/users/u1/events/dateString" {
			return http.StatusNotFound, err404
		}

		if !strings.Contains(record.Query, "eventType=blood_oxygen") || !strings.Contains(record.Query, "subType=odi") {
			return http.StatusOK, errMissingEventType
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"extra":{"value":98}}]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.SpO2Windows(t.Context(), "2026-06-07", "UTC"); err != nil {
		t.Fatalf("SpO2Windows: %v", err)
	}
}

func TestZeppClientSpO2WindowsRejectsInvalidTimezone(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		return http.StatusOK, `{"items":[]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.SpO2Windows(t.Context(), "2026-06-07", "invalid"); err == nil {
		t.Fatalf("expected error for invalid timezone")
	}
}

func TestZeppClientDataHostDateRangeMethodsHitEndpoints(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		call func(context.Context, *icu.ZeppClient, string, string) (any, error)
	}{
		{
			name: "HeartRateEndpoint",
			path: "/users/u1/heartRate",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.HeartRateEndpoint(ctx, o, n)
			},
		},
		{
			name: "WeightRecords",
			path: "/users/u1/members/-1/weightRecords",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.WeightRecords(ctx, o, n)
			},
		},
		{
			name: "ManualData",
			path: "/v1/user/manualData.json",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.ManualData(ctx, o, n)
			},
		},
		{
			name: "BloodPressureUser",
			path: "/users/me/bloodPressure",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.BloodPressureUser(ctx, o, n)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
				if record.Path != tc.path {
					return http.StatusNotFound, err404
				}

				return http.StatusOK, zeppV2EventRaw
			})
			t.Cleanup(srv.Close)

			client := newTestZeppClient(srv)
			if _, err := tc.call(t.Context(), client, "2026-06-01", "2026-06-01"); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
		})
	}
}

func TestZeppClientDataHostDateRangeMethodsRejectBadDates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		oldest string
		newest string
		call   func(context.Context, *icu.ZeppClient, string, string) (any, error)
	}{
		{
			name:   "HeartRateEndpointBadOldest",
			oldest: "garbage",
			newest: "2026-06-01",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.HeartRateEndpoint(ctx, o, n)
			},
		},
		{
			name:   "WeightRecordsBadNewest",
			oldest: "2026-06-01",
			newest: "garbage",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.WeightRecords(ctx, o, n)
			},
		},
		{
			name:   "ManualDataBadOldest",
			oldest: "not-a-date",
			newest: "2026-06-01",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.ManualData(ctx, o, n)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
				return http.StatusOK, zeppV2EventRaw
			})
			t.Cleanup(srv.Close)

			client := newTestZeppClient(srv)
			if _, err := tc.call(t.Context(), client, tc.oldest, tc.newest); err == nil {
				t.Fatalf("%s: expected error for invalid date", tc.name)
			}
		})
	}
}

func TestZeppClientDataHostDateRangeMethodsReturnErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(context.Context, *icu.ZeppClient, string, string) (any, error)
	}{
		{
			name: "HeartRateEndpoint",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.HeartRateEndpoint(ctx, o, n)
			},
		},
		{
			name: "WeightRecords",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.WeightRecords(ctx, o, n)
			},
		},
		{
			name: "ManualData",
			call: func(ctx context.Context, c *icu.ZeppClient, o, n string) (any, error) {
				return c.ManualData(ctx, o, n)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
				return http.StatusInternalServerError, err500
			})
			t.Cleanup(srv.Close)

			client := newTestZeppClient(srv)
			if _, err := tc.call(t.Context(), client, "2026-06-01", "2026-06-01"); err == nil {
				t.Fatalf("%s: expected error on 5xx", tc.name)
			}
		})
	}
}
