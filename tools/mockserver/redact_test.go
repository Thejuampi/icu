package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// No form value (password, access code, anything) may reach /_mock/writes,
// on any path variant, with or without a form Content-Type.
func TestMockWritesNeverKeepFormValues(t *testing.T) {
	t.Parallel()
	plain, _ := newRedirectTestServer(t)
	servers := map[string]string{"default": newTestServer(t).URL, "redirect-sink": plain.URL}
	secrets := []string{"RCPASSWORD-x1", mockZeppAccessCode, "user-secret@example.com", "SECRETTOKEN"}
	form := url.Values{"password": {secrets[0]}, "code": {secrets[1]}, "username": {secrets[2]}, "token": {secrets[3]}}.Encode()
	paths := []string{
		mockZeppTokensPath, mockZeppLoginPath,
		mockZeppTokensPath + "/", "/" + mockZeppTokensPath, "/v2/registrations/./tokens",
		mockZeppLoginPath + "/", "/" + mockZeppLoginPath, "/v2/registrations/x/../tokens",
		"/api/v1/athlete/0/anything",
	}
	for name, base := range servers {
		for _, p := range paths {
			for _, ct := range []string{"application/x-www-form-urlencoded", "application/x-www-form-urlencoded; charset=utf-8", "text/plain"} {
				if ct == "text/plain" && p == "/api/v1/athlete/0/anything" {
					continue // non-form body on a non-auth path is kept as-is by design
				}
				send(t, noFollowClient(), http.MethodPost, base+p, form, map[string]string{"Content-Type": ct})
			}
		}
		resp, err := http.Get(base + "/_mock/writes") //nolint:noctx // test
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		for _, s := range secrets {
			if strings.Contains(string(body), s) || strings.Contains(string(body), url.QueryEscape(s)) {
				t.Fatalf("%s: secret %q in /_mock/writes: %s", name, s, body)
			}
		}
		if !strings.Contains(string(body), "[redacted]") {
			t.Fatalf("%s: expected redacted entries, got %s", name, body)
		}
	}
}
