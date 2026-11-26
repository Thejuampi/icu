package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestZeppLoginReusesInstallationIdentity(t *testing.T) {
	withZeppTestEnv(t)
	var tokenRequests atomic.Int32
	var rejectLogin atomic.Bool
	deviceIDs := make(chan string, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/registrations/tokens", func(w http.ResponseWriter, _ *http.Request) {
		tokenRequests.Add(1)
		w.Header().Set("Location", "https://mock.invalid/callback?access=synthetic-code&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	})
	mux.HandleFunc("POST /v2/client/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("code") != "synthetic-code" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		deviceIDs <- r.PostForm.Get("device_id")
		if rejectLogin.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if _, err := io.WriteString(w, `{"token_info":{"login_token":"synthetic-login","app_token":"synthetic-app","user_id":"synthetic-user"}}`); err != nil {
			t.Error(err)
		}
	})
	mock := httptest.NewServer(mux)
	t.Cleanup(mock.Close)

	login := []string{"icu", "zepp", "login", "--android-unsafe", "--email", "fixture@example.invalid", "--zepp-base-url", mock.URL}
	loginWithPassword := slices.Concat(login, []string{"--password", "synthetic-password"})
	runZeppSuccess(t, loginWithPassword)
	firstID := nextZeppDeviceID(t, deviceIDs)
	if valid, err := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, firstID); err != nil || !valid {
		t.Fatal("first login did not send a UUID v4")
	}
	before, err := icu.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if before.ZeppDeviceID != firstID {
		t.Fatal("first login did not persist the device ID it sent")
	}

	var stderr bytes.Buffer
	if code := run(login, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "--reauthenticate") ||
		strings.Contains(stderr.String(), "Password:") || tokenRequests.Load() != 1 || len(deviceIDs) != 0 {
		t.Fatalf("repeated login should stop before prompting or calling Zepp, code=%d stderr=%s", code, &stderr)
	}

	rejectLogin.Store(true)
	reauthenticate := slices.Concat(loginWithPassword, []string{"--reauthenticate"})
	if code := run(reauthenticate, io.Discard, io.Discard); code == 0 || nextZeppDeviceID(t, deviceIDs) != firstID {
		t.Fatal("failed explicit login did not reuse the installation identity")
	}
	after, err := icu.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if *after != *before {
		t.Fatal("failed login changed the stored credentials or installation identity")
	}

	rejectLogin.Store(false)
	runZeppSuccess(t, reauthenticate)
	if nextZeppDeviceID(t, deviceIDs) != firstID {
		t.Fatal("explicit login changed the installation identity")
	}

	runZeppSuccess(t, []string{"icu", "zepp", "logout"})
	runZeppSuccess(t, loginWithPassword)
	if nextZeppDeviceID(t, deviceIDs) != firstID {
		t.Fatal("local logout discarded the installation identity")
	}
}

func TestZeppLoginProtectsLegacyConfig(t *testing.T) {
	withZeppTestEnv(t)
	if err := icu.SaveConfig(&icu.Config{ZeppLoginToken: "legacy-login", ZeppAppToken: "legacy-app", ZeppUserID: "legacy-user"}); err != nil {
		t.Fatal(err)
	}

	var hits atomic.Int32
	mock := newZeppAuthMock(t, &hits)
	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--android-unsafe", "--email", "fixture@example.invalid", "--password", "synthetic-password", "--zepp-base-url", mock.URL}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "--reauthenticate") || hits.Load() != 0 {
		t.Fatalf("stored legacy credentials should block password login, code=%d stderr=%s hits=%d", code, &stderr, hits.Load())
	}

	runZeppSuccess(t, slices.Concat(args, []string{"--reauthenticate"}))
	cfg, err := icu.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ZeppDeviceID == "" || cfg.ZeppUserID != "u1" {
		t.Fatal("successful explicit login did not migrate the legacy config")
	}
}

func TestZeppLoginRejectsPartialURLOverrides(t *testing.T) {
	withZeppTestEnv(t)
	var hits atomic.Int32
	mock := newZeppAuthMock(t, &hits)
	t.Setenv("ZEPP_TOKENS_URL", mock.URL+"/v2/registrations/tokens")
	t.Setenv("ZEPP_LOGIN_URL", "")

	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--android-unsafe", "--email", "fixture@example.invalid", "--password", "synthetic-password"}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "specify both") || hits.Load() != 0 {
		t.Fatalf("partial auth overrides should fail locally, code=%d stderr=%s hits=%d", code, &stderr, hits.Load())
	}
}

func TestZeppWebLoginKeepsExistingSessionUntilSuccessfulReauthentication(t *testing.T) {
	withZeppTestEnv(t)
	before := icu.Config{
		ZeppDeviceID: "29894371-a279-4fbb-b7a3-37790e992c29", ZeppLoginToken: "previous-login",
		ZeppAppToken: "previous-app", ZeppUserID: "previous-user", ZeppCountryCode: "US",
	}
	if err := icu.SaveConfig(&before); err != nil {
		t.Fatal(err)
	}

	var registrations atomic.Int32
	var allowExchange atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("POST /registrations/fixture@example.invalid/tokens", func(w http.ResponseWriter, _ *http.Request) {
		registrations.Add(1)
		if _, err := io.WriteString(w, `{"access":"fixture-code","country_code":"DE"}`); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("POST /v2/client/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("app_name") != "com.huami.webapp" ||
			r.PostForm.Get("code") != "fixture-code" {
			t.Error("CLI did not select the web exchange")
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if !allowExchange.Load() {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}
		if _, err := io.WriteString(w, `{"token_info":{"login_token":"web-login","app_token":"web-app","user_id":"web-user"}}`); err != nil {
			t.Error(err)
		}
	})
	mock := httptest.NewServer(mux)
	t.Cleanup(mock.Close)

	args := []string{"icu", "zepp", "login", "--web", "--email", "fixture@example.invalid", "--zepp-base-url", mock.URL}
	var stderr bytes.Buffer
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "--reauthenticate") ||
		strings.Contains(stderr.String(), "Password:") || registrations.Load() != 0 {
		t.Fatal("saved session did not block the web login before prompting or making a request")
	}

	reauthenticate := slices.Concat(args, []string{"--reauthenticate", "--password", "synthetic-password"})
	if code := run(reauthenticate, io.Discard, io.Discard); code == 0 || registrations.Load() != 1 {
		t.Fatal("expected one web login and a failed exchange")
	}
	after, err := icu.LoadConfig()
	if err != nil || after == nil || *after != before {
		t.Fatalf("failed web login changed saved credentials or UUID, err=%v", err)
	}

	allowExchange.Store(true)
	runZeppSuccess(t, reauthenticate)
	after, err = icu.LoadConfig()
	if err != nil || after == nil || after.ZeppLoginToken != "web-login" || after.ZeppAppToken != "web-app" ||
		after.ZeppUserID != "web-user" || after.ZeppCountryCode != "DE" || after.ZeppDeviceID != before.ZeppDeviceID ||
		registrations.Load() != 2 {
		t.Fatalf("successful web login did not save the new session and retain the Android UUID, err=%v", err)
	}
}

func TestZeppWebLoginAsksForBaseURLWhenAuthOverridesAreSet(t *testing.T) {
	withZeppTestEnv(t)
	t.Setenv("ZEPP_TOKENS_URL", "http://127.0.0.1:1/v2/registrations/tokens")
	t.Setenv("ZEPP_LOGIN_URL", "http://127.0.0.1:1/v2/client/login")

	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--web", "--email", "fixture@example.invalid"}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "use --zepp-base-url") ||
		strings.Contains(stderr.String(), "Password:") {
		t.Fatal("web login with Android auth overrides should stop before prompting or using the default hosts")
	}
}

func TestZeppLoginRejectsInvalidWebFlagBeforeSendingCredentials(t *testing.T) {
	withZeppTestEnv(t)
	var hits atomic.Int32
	mock := newZeppAuthMock(t, &hits)
	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--web=typo", "--email", "fixture@example.invalid", "--zepp-base-url", mock.URL, "--password", "synthetic-password"}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "--web must be a boolean") || hits.Load() != 0 {
		t.Fatal("invalid web flag should fail locally")
	}
}

func TestZeppLoginDefaultUsesWebEndpoints(t *testing.T) {
	assertZeppWebLoginRequest(t, nil, "")
}

func TestZeppLoginWebAliasUsesWebEndpoints(t *testing.T) {
	stderr := assertZeppWebLoginRequest(t, []string{"--web"}, "")
	if !strings.Contains(stderr, zeppWebAliasMessage) {
		t.Fatalf("expected --web deprecation note, stderr=%s", stderr)
	}
}

func TestZeppLoginReauthenticateWorksWithDefault(t *testing.T) {
	withZeppTestEnv(t)
	registrations, logins, mock := newZeppWebLoginMock(t)
	login := []string{"icu", "zepp", "login", "--email", "fixture@example.invalid", "--zepp-base-url", mock.URL}
	loginWithPassword := slices.Concat(login, []string{"--password", "synthetic-password"})
	runZeppSuccess(t, loginWithPassword)
	if registrations.Load() != 1 || logins.Load() != 1 {
		t.Fatalf("first default login hits: registrations=%d logins=%d", registrations.Load(), logins.Load())
	}

	var stderr bytes.Buffer
	if code := run(login, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "--reauthenticate") ||
		strings.Contains(stderr.String(), "Password:") || registrations.Load() != 1 || logins.Load() != 1 {
		t.Fatalf("saved session should block a second default login, code=%d stderr=%s hits=%d/%d", code, &stderr, registrations.Load(), logins.Load())
	}

	runZeppSuccess(t, slices.Concat(loginWithPassword, []string{"--reauthenticate"}))
	if registrations.Load() != 2 || logins.Load() != 2 {
		t.Fatalf("--reauthenticate with the default login should repeat the web exchange, hits=%d/%d", registrations.Load(), logins.Load())
	}
}

func TestZeppLoginAndroidUnsafeUsesAndroidPath(t *testing.T) {
	withZeppTestEnv(t)
	var tokens, logins atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/registrations/tokens", func(w http.ResponseWriter, _ *http.Request) {
		tokens.Add(1)
		w.Header().Set("Location", "https://mock.invalid/callback?access=synthetic-code&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	})
	mux.HandleFunc("POST /registrations/fixture@example.invalid/tokens", func(http.ResponseWriter, *http.Request) {
		t.Error("android-unsafe login hit the web registration path")
	})
	mux.HandleFunc("POST /v2/client/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("code") != "synthetic-code" ||
			r.PostForm.Get("app_name") != "com.huami.midong" || r.PostForm.Get("device_model") != "android_phone" ||
			r.PostForm.Get("grant_type") != "access_token" || r.PostForm.Get("device_id") == "" ||
			r.PostForm.Get("device_id") == "02:00:00:00:00:00" {
			t.Errorf("android-unsafe login form = %v", r.PostForm)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		logins.Add(1)
		if _, err := io.WriteString(w, `{"token_info":{"login_token":"android-login","app_token":"android-app","user_id":"android-user"}}`); err != nil {
			t.Error(err)
		}
	})
	mock := httptest.NewServer(mux)
	t.Cleanup(mock.Close)

	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--android-unsafe", "--email", "fixture@example.invalid", "--password", "synthetic-password", "--zepp-base-url", mock.URL}
	if code := run(args, io.Discard, &stderr); code != 0 {
		t.Fatalf("android-unsafe login exited %d, stderr=%s", code, &stderr)
	}
	if !strings.Contains(stderr.String(), zeppAndroidUnsafeWarning) {
		t.Fatalf("expected android-unsafe warning, stderr=%s", &stderr)
	}
	if tokens.Load() != 1 || logins.Load() != 1 {
		t.Fatalf("android-unsafe hits: tokens=%d logins=%d", tokens.Load(), logins.Load())
	}
}

func TestZeppLoginRejectsWebAndAndroidUnsafeTogether(t *testing.T) {
	withZeppTestEnv(t)
	var hits atomic.Int32
	mock := newZeppAuthMock(t, &hits)
	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--web", "--android-unsafe", "--email", "fixture@example.invalid", "--password", "synthetic-password", "--zepp-base-url", mock.URL}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "cannot combine --web and --android-unsafe") || hits.Load() != 0 {
		t.Fatalf("combined flags should fail locally, code=%d stderr=%s hits=%d", code, &stderr, hits.Load())
	}
}

func TestZeppLoginAndroidUnsafeWithWebFalseUsesAndroidPath(t *testing.T) {
	withZeppTestEnv(t)
	var tokens, logins atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/registrations/tokens", func(w http.ResponseWriter, _ *http.Request) {
		tokens.Add(1)
		w.Header().Set("Location", "https://mock.invalid/callback?access=synthetic-code&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	})
	mux.HandleFunc("POST /registrations/fixture@example.invalid/tokens", func(http.ResponseWriter, *http.Request) {
		t.Error("--web=false with --android-unsafe hit the web registration path")
	})
	mux.HandleFunc("POST /v2/client/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("app_name") != "com.huami.midong" ||
			r.PostForm.Get("device_model") != "android_phone" {
			t.Errorf("android form = %v", r.PostForm)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		logins.Add(1)
		if _, err := io.WriteString(w, `{"token_info":{"login_token":"android-login","app_token":"android-app","user_id":"android-user"}}`); err != nil {
			t.Error(err)
		}
	})
	mock := httptest.NewServer(mux)
	t.Cleanup(mock.Close)

	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--android-unsafe", "--web=false", "--email", "fixture@example.invalid", "--password", "synthetic-password", "--zepp-base-url", mock.URL}
	if code := run(args, io.Discard, &stderr); code != 0 {
		t.Fatalf("android-unsafe with --web=false exited %d, stderr=%s", code, &stderr)
	}
	if !strings.Contains(stderr.String(), zeppAndroidUnsafeWarning) {
		t.Fatalf("expected android-unsafe warning, stderr=%s", &stderr)
	}
	if strings.Contains(stderr.String(), zeppWebAliasMessage) {
		t.Fatalf("did not expect --web alias note with --web=false, stderr=%s", &stderr)
	}
	if tokens.Load() != 1 || logins.Load() != 1 {
		t.Fatalf("expected the Android path, tokens=%d logins=%d", tokens.Load(), logins.Load())
	}
}

func TestZeppLoginWebFalseAloneUsesWebPath(t *testing.T) {
	stderr := assertZeppWebLoginRequest(t, []string{"--web=false"}, "")
	if strings.Contains(stderr, zeppWebAliasMessage) {
		t.Fatalf("--web=false is not the alias and should not print the note, stderr=%s", stderr)
	}
	if strings.Contains(stderr, zeppAndroidUnsafeWarning) {
		t.Fatalf("--web=false alone must not use the Android path, stderr=%s", stderr)
	}
}

func TestZeppTokenDefaultUsesWebEndpoints(t *testing.T) {
	assertZeppWebLoginRequest(t, nil, "token")
}

func TestZeppLoginRejectsInvalidAndroidUnsafeFlagBeforeSendingCredentials(t *testing.T) {
	withZeppTestEnv(t)
	var hits atomic.Int32
	mock := newZeppAuthMock(t, &hits)
	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "login", "--android-unsafe=typo", "--email", "fixture@example.invalid", "--zepp-base-url", mock.URL, "--password", "synthetic-password"}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "--android-unsafe must be a boolean") || hits.Load() != 0 {
		t.Fatal("invalid android-unsafe flag should fail locally")
	}
}

func assertZeppWebLoginRequest(t *testing.T, extra []string, action string) string {
	t.Helper()
	if action == "" {
		action = "login"
	}
	withZeppTestEnv(t)
	registrations, logins, mock := newZeppWebLoginMock(t)
	args := slices.Concat([]string{"icu", "zepp", action, "--email", "fixture@example.invalid", "--password", "synthetic-password", "--zepp-base-url", mock.URL}, extra)
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("%s %v exited %d, stderr=%s", action, extra, code, &stderr)
	}
	if registrations.Load() != 1 || logins.Load() != 1 {
		t.Fatalf("%s %v did not use the web endpoints, registrations=%d logins=%d", action, extra, registrations.Load(), logins.Load())
	}
	if action == "login" {
		cfg, err := icu.LoadConfig()
		if err != nil || cfg.ZeppLoginToken != "web-login" || cfg.ZeppAppToken != "web-app" || cfg.ZeppUserID != "web-user" {
			t.Fatalf("web login did not persist the mock session, cfg=%+v err=%v", cfg, err)
		}
	}
	if action == "token" && !strings.Contains(stdout.String(), `"loginToken": "web-login"`) {
		t.Fatalf("web token output = %q", stdout.String())
	}

	return stderr.String()
}

func newZeppWebLoginMock(t *testing.T) (*atomic.Int32, *atomic.Int32, *httptest.Server) {
	t.Helper()
	var registrations, logins atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/registrations/tokens", func(http.ResponseWriter, *http.Request) {
		t.Error("web login hit the Android registration path")
	})
	mux.HandleFunc("POST /registrations/fixture@example.invalid/tokens", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("app_name") != "com.huami.webapp" ||
			r.Header.Get("App_name") != "com.huami.webapp" || r.PostForm.Get("json_response") != "true" {
			t.Errorf("web registration form=%v headers=%v", r.PostForm, r.Header)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		registrations.Add(1)
		if _, err := io.WriteString(w, `{"access":"fixture-code","country_code":"DE"}`); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("POST /v2/client/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("app_name") != "com.huami.webapp" ||
			r.PostForm.Get("third_name") != "huami" || r.PostForm.Get("grant_type") != "access_token" ||
			r.PostForm.Get("device_model") != "web" || r.PostForm.Get("device_id") != "02:00:00:00:00:00" ||
			r.PostForm.Get("code") != "fixture-code" {
			t.Errorf("web login form = %v", r.PostForm)
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		logins.Add(1)
		if _, err := io.WriteString(w, `{"token_info":{"login_token":"web-login","app_token":"web-app","user_id":"web-user"}}`); err != nil {
			t.Error(err)
		}
	})
	mock := httptest.NewServer(mux)
	t.Cleanup(mock.Close)

	return &registrations, &logins, mock
}

func runZeppSuccess(t *testing.T, args []string) {
	t.Helper()
	var stderr bytes.Buffer
	if code := run(args, io.Discard, &stderr); code != 0 {
		t.Fatalf("zepp command exited %d, stderr=%s", code, &stderr)
	}
}

func nextZeppDeviceID(t *testing.T, ids <-chan string) string {
	t.Helper()
	select {
	case id := <-ids:
		return id
	default:
		t.Fatal("mock received no login device ID")
		return ""
	}
}
