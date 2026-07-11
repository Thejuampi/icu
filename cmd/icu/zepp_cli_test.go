package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const (
	err404CLI              = `{"code":404}`
	errMissingEventTypeCLI = `{"code":0,"message":"missing eventType"}`
	eventsPathCLI          = "/users/u1/events"
	v2EventsPathCLI        = "/v2/users/me/events"
)

type zeppMockServer struct {
	server  *httptest.Server
	records []zeppMockRequest
}

type zeppMockRequest struct {
	Method string
	Path   string
	Query  string
}

func newZeppMockServer(handler func(req zeppMockRequest) (int, string)) *zeppMockServer {
	state := &zeppMockServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		state.records = append(state.records, zeppMockRequest{
			Method: request.Method,
			Path:   request.URL.Path,
			Query:  request.URL.RawQuery,
		})
		status, payload := handler(state.records[len(state.records)-1])

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(payload))
	})

	state.server = httptest.NewServer(mux)

	return state
}

func (s *zeppMockServer) Close() {
	s.server.Close()
}

func (s *zeppMockServer) URL() string {
	return s.server.URL
}

func (s *zeppMockServer) Last() zeppMockRequest {
	return s.records[len(s.records)-1]
}

func withZeppTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ZEPP_LOGIN_TOKEN", "tok")
	t.Setenv("ZEPP_APP_TOKEN", "test-app-token")
	t.Setenv("ZEPP_USER_ID", "u1")
	t.Setenv("ZEPP_COUNTRY_CODE", "US")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
}

func TestRunZeppLoginRequiresEmailAndPassword(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "login")

	err := cmd.Run(nil, map[string]string{}, nil)
	if err == nil {
		t.Fatalf("expected error when email missing")
	}

	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("error %q should mention email", err.Error())
	}
}

func TestRunZeppLoginRequiresPassword(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "login")

	err := cmd.Run(nil, map[string]string{"email": "u@example.com"}, nil)
	if err == nil {
		t.Fatalf("expected error when password missing")
	}

	if !strings.Contains(err.Error(), "password") {
		t.Fatalf("error %q should mention password", err.Error())
	}
}

func TestRunZeppLoginPersistsTokensOnSuccess(t *testing.T) {
	tokensSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://example.com/callback?access=atok&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	}))
	t.Cleanup(tokensSrv.Close)

	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":"ok","token_info":{"login_token":"lt","app_token":"at","user_id":"u1"}}`))
	}))
	t.Cleanup(loginSrv.Close)

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	withZeppTestEnv(t)
	t.Setenv("ZEPP_TOKENS_URL", tokensSrv.URL+"/v2/registrations/tokens")
	t.Setenv("ZEPP_LOGIN_URL", loginSrv.URL+"/v2/client/login")

	cmd, _ := registry.Lookup("zepp", "login")
	if err := cmd.Run(nil, map[string]string{"email": "u@example.com", "password": "secret"}, nil); err != nil {
		t.Fatalf("login: %v", err)
	}

	cfg, err := icu.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ZeppLoginToken != "lt" {
		t.Errorf("LoginToken = %q, want lt", cfg.ZeppLoginToken)
	}

	if cfg.ZeppAppToken != "at" {
		t.Errorf("AppToken = %q, want at", cfg.ZeppAppToken)
	}

	if cfg.ZeppUserID != "u1" {
		t.Errorf("UserID = %q, want u1", cfg.ZeppUserID)
	}
}

func TestRunZeppLogoutClearsAllZeppFields(t *testing.T) {
	withZeppTestEnv(t)
	t.Setenv("ZEPP_LOGIN_TOKEN", "tok-to-clear")

	if err := icu.SaveConfig(&icu.Config{
		ZeppLoginToken:  "tok-to-clear",
		ZeppAppToken:    "app-to-clear",
		ZeppUserID:      "user-to-clear",
		ZeppCountryCode: "US",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "logout")
	if err := cmd.Run(nil, map[string]string{}, nil); err != nil {
		t.Fatalf("logout: %v", err)
	}

	cfg, err := icu.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.ZeppLoginToken != "" {
		t.Errorf("LoginToken = %q, want empty", cfg.ZeppLoginToken)
	}

	if cfg.ZeppAppToken != "" {
		t.Errorf("AppToken = %q, want empty", cfg.ZeppAppToken)
	}

	if cfg.ZeppUserID != "" {
		t.Errorf("UserID = %q, want empty", cfg.ZeppUserID)
	}

	if cfg.ZeppCountryCode != "" {
		t.Errorf("CountryCode = %q, want empty", cfg.ZeppCountryCode)
	}
}

func TestRunZeppLogoutWithoutTokenIsNoop(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	withZeppTestEnv(t)

	if err := icu.SaveConfig(&icu.Config{}); err != nil {
		t.Fatalf("save: %v", err)
	}

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "logout")
	if err := cmd.Run(nil, map[string]string{}, nil); err != nil {
		t.Fatalf("logout: %v", err)
	}
}

func TestZeppProfileCommandHitsCorrectEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/huami.health.getUserInfo.json" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"code":1,"message":"success","data":{"userId":"u1","nickname":"ProfileTester"}}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "profile")
	if err := cmd.Run(nil, map[string]string{}, nil); err != nil {
		t.Fatalf("profile: %v", err)
	}

	last := srv.Last()
	if !strings.HasSuffix(last.Path, "/huami.health.getUserInfo.json") {
		t.Fatalf("last path = %q", last.Path)
	}
}

func TestZeppSummaryCommandHitsBandDataEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/v1/data/band_data.json" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"code":1,"message":"success","data":[]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "summary")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-07"}, nil); err != nil {
		t.Fatalf("summary: %v", err)
	}

	if !strings.HasSuffix(srv.Last().Path, "/v1/data/band_data.json") {
		t.Fatalf("last path = %q", srv.Last().Path)
	}
}

func TestZeppSleepCommandReturnsDecodedSleep(t *testing.T) {
	withZeppTestEnv(t)

	rawSummary := `{"slp":{"st":1717200000,"ed":1717230000,"dp":90,"lt":210}}`
	encodedSummary := base64.StdEncoding.EncodeToString([]byte(rawSummary))

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/v1/data/band_data.json" {
			return http.StatusNotFound, err404CLI
		}

		body := `{"code":1,"message":"success","data":[{"date_time":"2026-06-01","summary":"` + encodedSummary + `","data":"","data_hr":""}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "sleep")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-07"}, nil); err != nil {
		t.Fatalf("sleep: %v", err)
	}
}

func TestZeppHeartRateCommandDecodesHR(t *testing.T) {
	withZeppTestEnv(t)

	hrBytes := make([]byte, 0, 4)

	for _, bpm := range []uint16{72, 95} {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, bpm)
		hrBytes = append(hrBytes, buf...)
	}

	encodedHR := base64.StdEncoding.EncodeToString(hrBytes)

	srv := newZeppMockServer(func(_ zeppMockRequest) (int, string) {
		body := `{"code":1,"message":"success","data":[{"date_time":"2026-06-01","summary":"","data":"","data_hr":"` + encodedHR + `"}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "heart-rate")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("heart-rate: %v", err)
	}
}

func TestZeppSpO2CommandHitsEventsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != eventsPathCLI {
			return http.StatusNotFound, err404CLI
		}

		if !strings.Contains(req.Query, "eventType=blood_oxygen") {
			return http.StatusOK, errMissingEventTypeCLI
		}

		body := `{"items":[{"timestamp":1717200000000,"subType":"click","extra":"{\"spo2\":97}"}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "spo2")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("spo2: %v", err)
	}
}

func TestZeppPAICommandHitsEventsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != eventsPathCLI {
			return http.StatusNotFound, err404CLI
		}

		if !strings.Contains(req.Query, "eventType=PaiHealthInfo") {
			return http.StatusOK, errMissingEventTypeCLI
		}

		body := `{"items":[{"timestamp":1717200000000,"dailyPai":42.5,"totalPai":120.0,"maxHr":170,"restHr":52,"lowZoneMinutes":120,"mediumZoneMinutes":30,"highZoneMinutes":10}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "pai")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("pai: %v", err)
	}
}

func TestZeppStatusReportsValidTrue(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	withZeppTestEnv(t)

	if err := icu.SaveConfig(&icu.Config{
		ZeppLoginToken:  "tok",
		ZeppAppToken:    "at",
		ZeppUserID:      "u1",
		ZeppCountryCode: "US",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "status")
	if err := cmd.Run(nil, map[string]string{}, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestZeppStatusReportsNotValidWhenNoTokens(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	withZeppTestEnv(t)

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "status")
	if err := cmd.Run(nil, map[string]string{}, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestZeppStressCommandHitsEventsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != eventsPathCLI {
			return http.StatusNotFound, err404CLI
		}

		if !strings.Contains(req.Query, "eventType=all_day_stress") {
			return http.StatusOK, errMissingEventTypeCLI
		}

		body := `{"items":[{"timestamp":1717200000000,"minStress":20,"maxStress":80,"avgStress":45,"relaxProportion":40,"normalProportion":30,"mediumProportion":20,"highProportion":10}]}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "stress")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("stress: %v", err)
	}
}

func TestZeppWorkoutsCommandHitsSportEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/v1/sport/run/history.json" {
			return http.StatusNotFound, err404CLI
		}

		body := `{"code":1,"data":{"next":-1,"summary":[{"trackid":"1780272000","dis":"10000","calorie":"450","end_time":"1780275600","run_time":"3600","avg_pace":"0","avg_heart_rate":"150","type":1}]}}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "workouts")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-07"}, nil); err != nil {
		t.Fatalf("workouts: %v", err)
	}

	if !strings.HasSuffix(srv.Last().Path, "/v1/sport/run/history.json") {
		t.Fatalf("last path = %q", srv.Last().Path)
	}
}

func TestZeppWorkoutCommandHitsSportDetailEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/v1/sport/run/detail.json" {
			return http.StatusNotFound, err404CLI
		}

		if !strings.Contains(req.Query, "trackid=track1") {
			return http.StatusOK, `{"code":1,"message":"missing trackid"}`
		}

		body := `{"code":1,"message":"success","data":{"trackid":"track1","type":1,"avgHR":150}}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "workout")
	if err := cmd.Run([]string{"track1"}, map[string]string{}, nil); err != nil {
		t.Fatalf("workout: %v", err)
	}
}

func TestZeppWorkoutsCommandSupportsSportFlag(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/v1/sport/walking/history.json" {
			return http.StatusNotFound, err404CLI
		}

		body := `{"code":1,"data":{"next":-1,"summary":[]}}`

		return http.StatusOK, body
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "workouts")
	if err := cmd.Run(nil, map[string]string{"sport": "walking", "oldest": "2026-06-01", "newest": "2026-06-07"}, nil); err != nil {
		t.Fatalf("workouts walking: %v", err)
	}
}

func TestZeppWorkoutsCommandRejectsUnknownSport(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "workouts")
	if err := cmd.Run(nil, map[string]string{"sport": "quidditch"}, nil); err == nil {
		t.Fatalf("expected error for unknown sport")
	}
}

func TestZeppCommandWithoutTokenReturnsClearError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("ZEPP_LOGIN_TOKEN", "")
	t.Setenv("ZEPP_APP_TOKEN", "")
	t.Setenv("ZEPP_USER_ID", "")

	if err := icu.SaveConfig(&icu.Config{}); err != nil {
		t.Fatalf("save: %v", err)
	}

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "summary")

	err := cmd.Run(nil, map[string]string{}, nil)
	if err == nil {
		t.Fatalf("expected error for missing token")
	}

	if !strings.Contains(err.Error(), "zepp login") {
		t.Fatalf("error %q should mention zepp login", err.Error())
	}
}

func TestZeppCommandWithInvalidDateReturnsError(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	withZeppTestEnv(t)

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "summary")

	err := cmd.Run(nil, map[string]string{"oldest": "garbage"}, nil)
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestZeppStatusInvalidTokenReportsValidFalse(t *testing.T) {
	sentinel := "SENTINEL-INVALID-TOKEN-XYZ"

	withZeppTestEnv(t)
	t.Setenv("ZEPP_LOGIN_TOKEN", sentinel)

	srv := newZeppMockServer(func(_ zeppMockRequest) (int, string) {
		return http.StatusOK, `{"code":3001,"message":"token expired"}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	output, err := captureStdout(t, func() error {
		return runZeppStatus(map[string]string{})
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if strings.Contains(output, sentinel) {
		t.Fatalf("status output leaked the raw token: %s", output)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, output)
	}

	if payload["valid"] != false {
		t.Fatalf("valid = %v, want false", payload["valid"])
	}
}

func TestRunZeppProfileWithToken(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/huami.health.getUserInfo.json" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"code":1,"message":"success","data":{"userId":"u1","nickname":"ProfileNamer"}}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	output, err := captureStdout(t, runZeppProfile)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}

	if !strings.Contains(output, "ProfileNamer") {
		t.Fatalf("profile output missing nickname: %s", output)
	}
}

func TestZeppEventsCommandRequiresPreset(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "events")
	err := cmd.Run(nil, map[string]string{}, nil)
	if err == nil {
		t.Fatalf("expected error when preset missing")
	}

	if !strings.Contains(err.Error(), "preset") {
		t.Fatalf("error %q should mention preset", err.Error())
	}
}

func TestZeppEventsCommandRejectsUnknownPreset(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "events")
	err := cmd.Run(nil, map[string]string{"preset": "not-real"}, nil)
	if err == nil {
		t.Fatalf("expected error for unknown preset")
	}
}

func TestZeppEventsCommandHitsV2Endpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != v2EventsPathCLI {
			return http.StatusNotFound, err404CLI
		}

		if !strings.Contains(req.Query, "eventType=Charge") || !strings.Contains(req.Query, "subType=real_data") {
			return http.StatusBadRequest, errMissingEventTypeCLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"type":"Charge","subType":"real_data","value":85}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "events")
	if err := cmd.Run(nil, map[string]string{"preset": "body-battery", "oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("events: %v", err)
	}
}

func TestZeppV2WellnessCommandsHitCorrectEndpoint(t *testing.T) {
	cases := []struct {
		name      string
		cmdName   string
		flags     map[string]string
		eventType string
		subType   string
	}{
		{
			name:      "hrv-sdnn",
			cmdName:   "hrv",
			flags:     map[string]string{"metric": "sdnn", "oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "hrv_sdnn",
			subType:   "real_data",
		},
		{
			name:      "hrv-rmssd",
			cmdName:   "hrv",
			flags:     map[string]string{"metric": "rmssd", "oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "HRVRMSSD",
			subType:   "real_data",
		},
		{
			name:      "readiness",
			cmdName:   "readiness",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "readiness",
			subType:   "watch_score",
		},
		{
			name:      "body-battery",
			cmdName:   "body-battery",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "Charge",
			subType:   "real_data",
		},
		{
			name:      "hybridcharge",
			cmdName:   "hybridcharge",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "Charge",
			subType:   "insight_data",
		},
		{
			name:      "biocharge",
			cmdName:   "biocharge",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "Charge",
			subType:   "insight_data",
		},
		{
			name:      "health-summary",
			cmdName:   "health-summary",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "DailyHealth",
			subType:   "summary",
		},
		{
			name:      "mood",
			cmdName:   "mood",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "Emotion",
			subType:   "real_data",
		},
		{
			name:      "skin-temp",
			cmdName:   "skin-temp",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "skinTemp",
			subType:   "real_data",
		},
		{
			name:      "stress-minute",
			cmdName:   "stress-minute",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "Charge",
			subType:   "stress_data",
		},
		{
			name:      "respiratory-rate",
			cmdName:   "respiratory-rate",
			flags:     map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"},
			eventType: "RespiratoryRate",
			subType:   "real_data",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withZeppTestEnv(t)

			srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
				if req.Path != v2EventsPathCLI {
					return http.StatusNotFound, err404CLI
				}

				if !strings.Contains(req.Query, "eventType="+tc.eventType) || !strings.Contains(req.Query, "subType="+tc.subType) {
					return http.StatusBadRequest, errMissingEventTypeCLI
				}

				return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":42}]}`
			})
			t.Cleanup(srv.Close)
			t.Setenv("ZEPP_EVENTS_URL", srv.URL())

			registry := NewCommandRegistry()
			registerZeppCommands(registry)

			cmd, _ := registry.Lookup("zepp", tc.cmdName)

			if err := cmd.Run(nil, tc.flags, nil); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
		})
	}
}

func TestZeppWatchStatCommandsHitCorrectEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		cmdName string
		path    string
	}{
		{name: "sport-load", cmdName: "sport-load", path: "/v2/watch/users/u1/WatchSportStatistics/SPORT_LOAD"},
		{name: "vo2", cmdName: "vo2", path: "/v2/watch/users/u1/WatchSportStatistics/VO2_MAX"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withZeppTestEnv(t)

			srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
				if req.Path != tc.path {
					return http.StatusNotFound, err404CLI
				}

				return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":42}]}`
			})
			t.Cleanup(srv.Close)
			t.Setenv("ZEPP_BASE_URL", srv.URL())

			registry := NewCommandRegistry()
			registerZeppCommands(registry)

			cmd, _ := registry.Lookup("zepp", tc.cmdName)
			if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
		})
	}
}

func TestZeppBloodPressureCommandDefaultsToWatchSource(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != v2EventsPathCLI {
			return http.StatusNotFound, err404CLI
		}

		if !strings.Contains(req.Query, "eventType=blood_pressure") || !strings.Contains(req.Query, "subType=real_data") {
			return http.StatusBadRequest, errMissingEventTypeCLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"extra":{"systolic":120,"diastolic":80}}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "blood-pressure")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("blood-pressure watch: %v", err)
	}
}

func TestZeppBloodPressureCommandUserSource(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/users/me/bloodPressure" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"systolic":120,"diastolic":80}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "blood-pressure")
	if err := cmd.Run(nil, map[string]string{"source": "user", "oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("blood-pressure user: %v", err)
	}
}

func TestZeppBloodPressureCommandRejectsInvalidSource(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "blood-pressure")
	err := cmd.Run(nil, map[string]string{"source": "invalid"}, nil)
	if err == nil {
		t.Fatalf("expected error for invalid source")
	}
}

func TestZeppHRVCommandRejectsInvalidMetric(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "hrv")
	err := cmd.Run(nil, map[string]string{"metric": "invalid"}, nil)
	if err == nil {
		t.Fatalf("expected error for invalid metric")
	}
}

func TestZeppHeartRateCommandAppSource(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/users/u1/heartRate" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":68}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "heart-rate")
	if err := cmd.Run(nil, map[string]string{"source": "app", "oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("heart-rate app: %v", err)
	}
}

func TestZeppHeartRateCommandRejectsInvalidSource(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "heart-rate")
	if err := cmd.Run(nil, map[string]string{"source": "invalid"}, nil); err == nil {
		t.Fatalf("expected error for invalid source")
	}
}

func TestZeppWeightCommandHitsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/users/u1/members/-1/weightRecords" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":70.5}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "weight")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("weight: %v", err)
	}
}

func TestZeppManualDataCommandHitsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/v1/user/manualData.json" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"value":1}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "manual-data")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("manual-data: %v", err)
	}
}

func TestZeppSecondHeartRateCommandHitsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/users/me/fileInfo/events" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"url":"https://cos.example/1.zip"}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "second-heart-rate")
	if err := cmd.Run(nil, map[string]string{"oldest": "2026-06-01", "newest": "2026-06-01"}, nil); err != nil {
		t.Fatalf("second-heart-rate: %v", err)
	}
}

func TestZeppSpO2WindowsCommandHitsEndpoint(t *testing.T) {
	withZeppTestEnv(t)

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
		if req.Path != "/users/u1/events/dateString" {
			return http.StatusNotFound, err404CLI
		}

		return http.StatusOK, `{"items":[{"timestamp":1780272000000,"extra":{"value":98}}]}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "spo2-windows")
	if err := cmd.Run(nil, map[string]string{"date": "2026-06-07"}, nil); err != nil {
		t.Fatalf("spo2-windows: %v", err)
	}
}

func TestZeppSpO2WindowsCommandRequiresDate(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerZeppCommands(registry)

	cmd, _ := registry.Lookup("zepp", "spo2-windows")
	if err := cmd.Run(nil, map[string]string{}, nil); err == nil {
		t.Fatalf("expected error when date missing")
	}
}
