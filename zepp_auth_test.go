package icu_test

import (
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func binaryPutUint16(buf []byte, v int) {
	binary.LittleEndian.PutUint16(buf, uint16(v)) //nolint:gosec // test helper, v is always small
}

func base64EncodeForTest(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// TestZeppLoginHappyPath exercises the full auth flow against mock servers.
// The auth flow is two-step:
//  1. POST to /v2/registrations/tokens with AES-CBC encrypted body → 303 redirect
//  2. POST to /v2/client/login with form data → 200 with token_info
func TestZeppLoginHappyPath(t *testing.T) {
	t.Parallel()

	var tokensRequests, loginRequests int

	tokensSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokensRequests++

		if r.URL.Path != "/v2/registrations/tokens" {
			t.Errorf("tokens path = %q, want /v2/registrations/tokens", r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("tokens method = %q, want POST", r.Method)
		}

		w.Header().Set("Location", "https://example.com/callback?access=fake-access-token&refresh=fake-refresh&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	}))
	t.Cleanup(tokensSrv.Close)

	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginRequests++

		if r.URL.Path != "/v2/client/login" {
			t.Errorf("login path = %q, want /v2/client/login", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"ok","token_info":{"login_token":"lt","app_token":"at","user_id":"u1"}}`))
	}))
	t.Cleanup(loginSrv.Close)

	auth, err := icu.ZeppLoginWithURLs(tokensSrv.URL+"/v2/registrations/tokens", loginSrv.URL+"/v2/client/login", "u@example.com", "secret")
	if err != nil {
		t.Fatalf("ZeppLogin: %v", err)
	}

	if auth.LoginToken != "lt" {
		t.Errorf("LoginToken = %q, want lt", auth.LoginToken)
	}

	if auth.AppToken != "at" {
		t.Errorf("AppToken = %q, want at", auth.AppToken)
	}

	if auth.UserID != "u1" {
		t.Errorf("UserID = %q, want u1", auth.UserID)
	}

	if auth.CountryCode != "US" {
		t.Errorf("CountryCode = %q, want US", auth.CountryCode)
	}

	if tokensRequests != 1 {
		t.Errorf("tokens requests = %d, want 1", tokensRequests)
	}

	if loginRequests != 1 {
		t.Errorf("login requests = %d, want 1", loginRequests)
	}
}

func TestZeppLoginRejectsNonRedirectResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := icu.ZeppLoginWithURLs(srv.URL+"/v2/registrations/tokens", srv.URL+"/v2/client/login", "u@example.com", "secret")
	if err == nil {
		t.Fatalf("expected error for non-303 response")
	}

	if !strings.Contains(err.Error(), "303") {
		t.Errorf("error %q should mention 303", err.Error())
	}
}

func TestZeppLoginRejectsLoginFailure(t *testing.T) {
	t.Parallel()

	tokensSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://example.com/callback?access=foo&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	}))
	t.Cleanup(tokensSrv.Close)

	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(loginSrv.Close)

	_, err := icu.ZeppLoginWithURLs(tokensSrv.URL+"/v2/registrations/tokens", loginSrv.URL+"/v2/client/login", "u@example.com", "secret")
	if err == nil {
		t.Fatalf("expected error for failed login")
	}
}

func TestPKCS7PadIsCorrectForBlockAlignedInput(t *testing.T) {
	t.Parallel()

	padded, err := icu.PKCS7PadForTest([]byte("hello"), 16)
	if err != nil {
		t.Fatalf("PKCS7Pad: %v", err)
	}

	if len(padded) != 16 {
		t.Fatalf("padded length = %d, want 16", len(padded))
	}

	if padded[len(padded)-1] != 11 {
		t.Errorf("last byte = %d, want 11 (16 - 5)", padded[len(padded)-1])
	}
}

func TestZeppLoginEncryptsAndPostsPayload(t *testing.T) {
	t.Parallel()

	tokensSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)

		if n == 0 {
			t.Errorf("empty body sent to tokens endpoint")
		}

		if n < 16 {
			t.Errorf("body too short (%d bytes) to be AES-CBC encrypted", n)
		}

		w.Header().Set("Location", "https://example.com/callback?access=tok&country_code=DE")
		w.WriteHeader(http.StatusSeeOther)
	}))
	t.Cleanup(tokensSrv.Close)

	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		if r.Form.Get("country_code") != "DE" {
			t.Errorf("country_code = %q, want DE", r.Form.Get("country_code"))
		}

		_, _ = w.Write([]byte(`{"result":"ok","token_info":{"login_token":"lt","app_token":"at","user_id":"u1"}}`))
	}))
	t.Cleanup(loginSrv.Close)

	auth, err := icu.ZeppLoginWithURLs(tokensSrv.URL+"/v2/registrations/tokens", loginSrv.URL+"/v2/client/login", "u@example.com", "secret")
	if err != nil {
		t.Fatalf("ZeppLogin: %v", err)
	}

	if auth.CountryCode != "DE" {
		t.Errorf("CountryCode = %q, want DE", auth.CountryCode)
	}
}

func TestZeppClientRejectsZeroLengthHRSentinel(t *testing.T) {
	t.Parallel()

	hrBytes := make([]byte, 4)
	binaryPutUint16(hrBytes, 254)
	binaryPutUint16(hrBytes[2:], 72)

	encoded := base64EncodeForTest(hrBytes)

	srv := newZeppTestServer(func(_ zeppRequestRecord) (int, string) {
		body := `{"code":1,"message":"success","data":[{"date_time":"2026-06-01","summary":"","data":"","data_hr":"` + encoded + `"}]}`

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
	if len(hr) != 2 {
		t.Fatalf("hr length = %d, want 2", len(hr))
	}

	if hr[0].BPM != 0 {
		t.Errorf("hr[0] = %d, want 0 (sentinel 254 mapped to 0)", hr[0].BPM)
	}

	if hr[1].BPM != 72 {
		t.Errorf("hr[1] = %d, want 72", hr[1].BPM)
	}
}

func TestPKCS7PadForTestRejectsInvalidBlockSize(t *testing.T) {
	t.Parallel()

	_, err := icu.PKCS7PadForTest([]byte("data"), 0)
	if err == nil {
		t.Fatal("expected error for block size 0")
	}

	_, err = icu.PKCS7PadForTest([]byte("data"), 256)
	if err == nil {
		t.Fatal("expected error for block size 256")
	}
}

func TestZeppClientWithZeppCountryCode(t *testing.T) {
	t.Parallel()

	auth := &icu.ZeppAuthResult{AppToken: "tok", UserID: "u1"}
	client := icu.NewZeppClientFromAuth(auth, icu.WithZeppCountryCode("DE"))

	if got := client.DataHostForTest(); got != "https://api-mifit-de.huami.com" {
		t.Errorf("dataHost = %q, want https://api-mifit-de.huami.com", got)
	}
}
