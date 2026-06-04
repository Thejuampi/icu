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

func TestRunZeppLogoutClearsToken(t *testing.T) {
	withZeppTestEnv(t)
	t.Setenv("ZEPP_LOGIN_TOKEN", "tok-to-clear")

	if err := icu.SaveConfig(&icu.Config{ZeppLoginToken: "tok-to-clear"}); err != nil {
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
}

func TestRunZeppLogoutWithoutTokenIsNoop(t *testing.T) {
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
			return http.StatusNotFound, `{"code":404}`
		}

		return http.StatusOK, `{"code":1,"message":"success","data":{"userId":"u1","nickname":"ProfileTester"}}`
	})
	t.Cleanup(srv.Close)
	t.Setenv("ZEPP_BASE_URL", srv.URL())
	t.Setenv("ZEPP_EVENTS_URL", srv.URL())

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
			return http.StatusNotFound, `{"code":404}`
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
			return http.StatusNotFound, `{"code":404}`
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

	srv := newZeppMockServer(func(req zeppMockRequest) (int, string) {
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
		if req.Path != "/users/u1/events" {
			return http.StatusNotFound, `{"code":404}`
		}

		if !strings.Contains(req.Query, "eventType=blood_oxygen") {
			return http.StatusOK, `{"code":0,"message":"missing eventType"}`
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
		if req.Path != "/users/u1/events" {
			return http.StatusNotFound, `{"code":404}`
		}

		if !strings.Contains(req.Query, "eventType=PaiHealthInfo") {
			return http.StatusOK, `{"code":0,"message":"missing eventType"}`
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

func TestZeppStatusReportsValidTrue(t *testing.T) {
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

func TestZeppStatusReportsNotValidWhenNoTokens(t *testing.T) {
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
		if req.Path != "/users/u1/events" {
			return http.StatusNotFound, `{"code":404}`
		}

		if !strings.Contains(req.Query, "eventType=all_day_stress") {
			return http.StatusOK, `{"code":0,"message":"missing eventType"}`
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
			return http.StatusNotFound, `{"code":404}`
		}

		body := `{"code":1,"message":"success","data":[{"trackid":"t1","type":1}]}`
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
			return http.StatusNotFound, `{"code":404}`
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

func TestZeppCommandWithInvalidDateReturnsError(t *testing.T) {
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
			return http.StatusNotFound, `{"code":404}`
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
