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

	login := []string{"icu", "zepp", "login", "--email", "fixture@example.invalid", "--zepp-base-url", mock.URL}
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
	args := []string{"icu", "zepp", "login", "--email", "fixture@example.invalid", "--password", "synthetic-password", "--zepp-base-url", mock.URL}
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
	args := []string{"icu", "zepp", "login", "--email", "fixture@example.invalid", "--password", "synthetic-password"}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "specify both") || hits.Load() != 0 {
		t.Fatalf("partial auth overrides should fail locally, code=%d stderr=%s hits=%d", code, &stderr, hits.Load())
	}
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
