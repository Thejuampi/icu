package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Redirect mode (-redirect-sink) is a black-box check that secrets never
// follow a redirect to another host. It is off by default; with it on the
// mock starts a second loopback listener (the "sink", another port) and:
//
//	/_redirect/<mode>/<path>  answers <path> with a redirect instead:
//	    cross307, cross308  -> 307/308 to the sink, same <path> and query
//	    same307, same308    -> 307/308 to <path> on this host:port
//	  The Zepp tokens endpoint under the prefix is served normally (so login
//	  reaches the code-bearing login call) unless <mode> ends in "-tokens".
//	POST /v2/registrations/tokens  synthetic Zepp step 1: 303 with
//	    access=mockZeppAccessCode in the Location (never followed)
//	POST /v2/client/login          synthetic Zepp step 2: token_info when
//	    the form carries code=mockZeppAccessCode, else 401
//	GET  /_mock/leaks              what the sink received, secrets redacted
//
// The sink records every request (method, path, query keys, headers, body
// size) and reports booleans for an access code, an API key (Authorization)
// and an Apptoken; secret values are never echoed.
const (
	redirectPrefix      = "/_redirect/"
	mockZeppTokensPath  = "/v2/registrations/tokens"
	mockZeppLoginPath   = "/v2/client/login"
	mockZeppAccessCode  = "mock-access-code"
	tokensOnlySuffix    = "-tokens"
	redactedValue       = "[redacted]"
	mockZeppLoginResult = `{"result":"ok","token_info":{"login_token":"mock-login-token","app_token":"mock-app-token","user_id":"mock-user"}}`
)

// Leak is one request the sink received, with secret values redacted.
type Leak struct {
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	QueryKeys        []string          `json:"queryKeys,omitempty"`
	Headers          map[string]string `json:"headers"`
	BodyBytes        int               `json:"bodyBytes"`
	FormKeys         []string          `json:"formKeys,omitempty"`
	CodeReceived     bool              `json:"codeReceived"`
	APIKeyReceived   bool              `json:"apiKeyReceived"`
	AppTokenReceived bool              `json:"appTokenReceived"`
}

type redirectSink struct {
	mu    sync.Mutex
	url   string
	leaks []Leak
}

func newRedirectSink(sinkURL string) *redirectSink {
	return &redirectSink{url: strings.TrimRight(sinkURL, "/")}
}

// isSecretHeader reports headers redacted in /_mock/leaks (Referer can carry
// a query with a code).
func isSecretHeader(name string) bool {
	switch name {
	case "Authorization", "Apptoken", "Cookie", "Referer":
		return true
	}

	return false
}

func (k *redirectSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	form, _ := url.ParseQuery(string(body))
	query := r.URL.Query()

	leak := Leak{
		Method:           r.Method,
		Path:             r.URL.Path,
		QueryKeys:        sortedKeys(query),
		Headers:          map[string]string{},
		BodyBytes:        len(body),
		CodeReceived:     form.Get("code") != "" || query.Get("code") != "",
		APIKeyReceived:   r.Header.Get("Authorization") != "",
		AppTokenReceived: r.Header.Get("Apptoken") != "",
	}

	for name := range r.Header {
		leak.Headers[name] = r.Header.Get(name)
		if isSecretHeader(name) {
			leak.Headers[name] = redactedValue
		}
	}

	if len(body) > 0 {
		leak.FormKeys = sortedKeys(form)
	}

	k.mu.Lock()
	k.leaks = append(k.leaks, leak)
	k.mu.Unlock()

	// A decoy success: if a client ever followed the redirect, the login
	// would "work", so only the recorded leak shows what went wrong.
	writeJSON(w, http.StatusOK, []byte(mockZeppLoginResult))
}

func sortedKeys(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func (k *redirectSink) report() []byte {
	k.mu.Lock()
	defer k.mu.Unlock()

	report := map[string]any{
		"count":            len(k.leaks),
		"codeReceived":     false,
		"apiKeyReceived":   false,
		"appTokenReceived": false,
		"requests":         append([]Leak{}, k.leaks...),
	}

	for _, leak := range k.leaks {
		report["codeReceived"] = report["codeReceived"].(bool) || leak.CodeReceived             //nolint:forcetypeassert // set above
		report["apiKeyReceived"] = report["apiKeyReceived"].(bool) || leak.APIKeyReceived       //nolint:forcetypeassert // set above
		report["appTokenReceived"] = report["appTokenReceived"].(bool) || leak.AppTokenReceived //nolint:forcetypeassert // set above
	}

	body, _ := json.Marshal(report) //nolint:errchkjson // plain maps, slices and strings

	return body
}

func (k *redirectSink) reset() {
	k.mu.Lock()
	k.leaks = nil
	k.mu.Unlock()
}

// serveRedirectMode handles the redirect-mode routes; it reports false when
// the request is not one of them (or redirect mode is off).
func (s *server) serveRedirectMode(w http.ResponseWriter, r *http.Request) bool {
	if s.sink == nil {
		return false
	}

	switch {
	case r.URL.Path == "/_mock/leaks" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.sink.report())
	case strings.HasPrefix(r.URL.Path, redirectPrefix):
		s.handleRedirect(w, r)
	case r.URL.Path == mockZeppTokensPath && r.Method == http.MethodPost:
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, maxBodyBytes))
		w.Header().Set("Location", "https://mock.invalid/callback?access="+mockZeppAccessCode+"&refresh=mock-refresh&country_code=US")
		w.WriteHeader(http.StatusSeeOther)
	case r.URL.Path == mockZeppLoginPath && r.Method == http.MethodPost:
		if err := r.ParseForm(); err != nil || r.PostForm.Get("code") != mockZeppAccessCode {
			writeError(w, http.StatusUnauthorized, "bad or missing code")

			return true
		}

		writeJSON(w, http.StatusOK, []byte(mockZeppLoginResult))
	default:
		return false
	}

	return true
}

func redirectStatus(code string) int {
	switch code {
	case "307":
		return http.StatusTemporaryRedirect
	case "308":
		return http.StatusPermanentRedirect
	}

	return 0
}

func (s *server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	mode, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, redirectPrefix), "/")
	rest = "/" + rest
	kind, tokensOnly := strings.CutSuffix(mode, tokensOnlySuffix)

	var (
		target string
		status int
	)

	switch {
	case strings.HasPrefix(kind, "cross"):
		target, status = s.sink.url, redirectStatus(strings.TrimPrefix(kind, "cross"))
	case strings.HasPrefix(kind, "same"):
		status = redirectStatus(strings.TrimPrefix(kind, "same"))
	}

	if status == 0 {
		writeError(w, http.StatusNotFound, "unknown redirect mode "+mode+" (cross307, cross308, same307, same308, optional -tokens)")

		return
	}

	if rest == mockZeppTokensPath && !tokensOnly {
		r.URL.Path = rest
		s.ServeHTTP(w, r)

		return
	}

	target += rest
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	w.Header().Set("Location", target)
	w.WriteHeader(status)
}
