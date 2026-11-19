package icu

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestZeppWebLoginExchangesWebAccess(t *testing.T) {
	const (
		email    = "fixture+web@example.invalid"
		password = "synthetic-web-password"
		access   = "synthetic-web-access"
	)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /registrations/{email}/tokens", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PathValue("email") != email ||
			r.Header.Get("App_name") != "com.huami.webapp" ||
			r.PostForm.Get("password") != password || r.PostForm.Get("client_id") != "HuaMi" ||
			r.PostForm.Get("app_name") != "com.huami.webapp" || r.PostForm.Get("token") != "access" ||
			r.PostForm.Get("state") != "REDIRECTION" || r.PostForm.Get("json_response") != "true" ||
			r.PostForm.Get("redirect_uri") == "" || r.PostForm.Has("country_code") ||
			r.Header.Get("X-Hm-Ekv") != "" {
			t.Error("request did not use the web registration form")
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		writeZeppWebFixture(t, w, `{"access":"synthetic-web-access","country_code":"de","region":"us-west-2"}`)
	})
	mux.HandleFunc("POST /v2/client/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Header.Get("App_name") != "com.huami.webapp" ||
			r.PostForm.Get("app_name") != "com.huami.webapp" || r.PostForm.Get("device_model") != "web" ||
			r.PostForm.Get("device_id") != "02:00:00:00:00:00" || r.PostForm.Get("app_version") != "4.3.0" ||
			r.PostForm.Get("country_code") != "DE" || r.PostForm.Get("grant_type") != "access_token" ||
			r.PostForm.Get("third_name") != "huami" || r.PostForm.Get("allow_registration") != "false" ||
			r.PostForm.Get("code") != access || r.PostForm.Has("password") {
			t.Error("request did not exchange the access code as a web session")
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		writeZeppWebFixture(t, w, `{"token_info":{"login_token":"fixture-login","app_token":"fixture-app","user_id":"fixture-user"}}`)
	})
	mock := httptest.NewServer(mux)
	t.Cleanup(mock.Close)

	got, err := ZeppWebLogin(email, password, mock.URL+"/")
	want := ZeppAuthResult{LoginToken: "fixture-login", AppToken: "fixture-app", UserID: "fixture-user", CountryCode: "DE"}
	if err != nil || got == nil || *got != want {
		t.Fatalf("web login did not return the mock session and account country, err=%v", err)
	}
}

func writeZeppWebFixture(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, body); err != nil {
		t.Error(err)
	}
}

func TestZeppWebLoginStopsOnRejectedRegistration(t *testing.T) {
	const email = "fixture@example.invalid"
	cases := []struct {
		name, body, location, want string
		status                     int
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: "expected status 200, got 401"},
		{name: "malformed JSON", status: http.StatusOK, body: `{`, want: "invalid registration JSON"},
		{name: "missing access", status: http.StatusOK, body: `{"message":"synthetic-secret"}`, want: "missing access code"},
		{name: "redirect", status: http.StatusTemporaryRedirect, location: "/sink", want: "expected status 200, got 307"},
		{name: "malformed redirect", status: http.StatusSeeOther, location: "/%zz/synthetic-secret", want: "request failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var unexpected atomic.Int32
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/registrations/"+email+"/tokens" {
					unexpected.Add(1)
					w.WriteHeader(http.StatusNotFound)

					return
				}
				w.Header().Set("Location", tc.location)
				w.WriteHeader(tc.status)
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(mock.Close)

			got, err := ZeppWebLogin(email, "synthetic-secret", mock.URL)
			if got != nil || err == nil || !strings.Contains(err.Error(), tc.want) ||
				strings.Contains(err.Error(), email) || strings.Contains(err.Error(), "synthetic-secret") || unexpected.Load() != 0 {
				t.Fatalf("expected a sanitized registration error and no further request, unexpected requests=%d", unexpected.Load())
			}
		})
	}
}

func TestZeppWebBaseURL(t *testing.T) {
	for _, tc := range []struct {
		base  string
		valid bool
	}{
		{base: "https://account.huami.com/path", valid: true},
		{base: "http://[::1]:443", valid: true},
		{base: "http://localhost:4321", valid: true},
		{base: "http://example.invalid"},
		{base: "https://fixture@example.invalid"},
		{base: "http://127.0.0.1:4321?trailing=bad"},
		{base: "http://127.0.0.1:4321#ignored"},
		{base: "%not-a-url"},
	} {
		if err := validateZeppWebBaseURL(tc.base); (err == nil) != tc.valid {
			t.Errorf("web auth URL %q returned valid=%t, want %t", tc.base, err == nil, tc.valid)
		}
	}
}

func TestZeppWebRegistrationCountryUsesRegionWhenCountryAbsent(t *testing.T) {
	for _, tc := range []struct{ region, want string }{
		{region: "eu-central-1", want: "DE"},
		{region: "de", want: "DE"},
		{region: "cn-north-1", want: "CN"},
		{region: "ru", want: "RU"},
		{region: "sg", want: "SG"},
		{region: "us-west-2", want: "US"},
		{region: "", want: "US"},
	} {
		t.Run(tc.region, func(t *testing.T) {
			got := (zeppWebRegistration{Region: tc.region}).country()
			if got != tc.want {
				t.Fatalf("country for region %q = %q, want %q", tc.region, got, tc.want)
			}
		})
	}
}
