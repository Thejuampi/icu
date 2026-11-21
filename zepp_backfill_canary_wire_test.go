package icu

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type backfillCanaryTestTransport func(*http.Request) (*http.Response, error)

func (f backfillCanaryTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBackfillCanaryWireSequence(t *testing.T) {
	var got []string
	client := &ZeppClient{
		dataHost: zeppDataHostGlobal,
		appToken: "synthetic-web-token",
		userID:   "synthetic-user",
		httpClient: &http.Client{
			Transport: backfillCanaryTestTransport(func(req *http.Request) (*http.Response, error) {
				got = append(got, req.Method+" "+req.URL.Path)

				if req.Header.Get("Apptoken") != "synthetic-web-token" ||
					req.Header.Get("Appname") != "com.xiaomi.hm.health" ||
					req.Header.Get("Appplatform") != "web" || req.GetBody != nil || !req.Close {
					t.Fatal("wrong web headers or replay was not disabled")
				}

				switch req.Method {
				case http.MethodPost:
					if req.URL.RawQuery != "" || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
						t.Fatal("wrong POST form transport")
					}
					if err := req.ParseForm(); err != nil || req.PostForm.Get("trackid") != "123" ||
						req.PostForm.Get("source") != backfillCanaryWireSource {
						t.Fatal("wrong POST body")
					}
				case http.MethodDelete:
					if req.URL.Query().Get("trackid") != "123" ||
						req.URL.Query().Get("source") != backfillCanaryWireSource || req.Body != http.NoBody {
						t.Fatal("wrong DELETE identity or unexpected body")
					}
				default:
					t.Fatal("unexpected method")
				}

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"code":"1"}`))}, nil
			}),
		},
	}
	if err := client.postBackfillCanaryDetail(t.Context(), url.Values{
		"trackid": {"123"}, "source": {backfillCanaryWireSource}, "version": {"16"}, "time": {"0;1"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := client.postBackfillCanarySummary(t.Context(), url.Values{
		"trackid": {"123"}, "source": {backfillCanaryWireSource}, "version": {"16"}, "type": {"10"}, "end_time": {"125"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := client.deleteBackfillCanarySummary(t.Context(), url.Values{
		"trackid": {"123"}, "source": {backfillCanaryWireSource},
	}); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"POST /v1/sport/run/detail.json",
		"POST /v1/sport/run/summary.json",
		"DELETE /v1/sport/run/summary.json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected requests: %v", got)
	}
}

func TestBackfillCanaryWireRejectsRedirectAndUnsafeInput(t *testing.T) {
	requests := 0
	client := &ZeppClient{
		dataHost: zeppDataHostGlobal,
		appToken: "synthetic-web-token",
		userID:   "synthetic-user",
		httpClient: &http.Client{
			Transport: backfillCanaryTestTransport(func(_ *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": {"https://other.invalid/"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":1}`)),
				}, nil
			}),
		},
	}

	identity := url.Values{"trackid": {"123"}, "source": {backfillCanaryWireSource}}
	if err := client.deleteBackfillCanarySummary(t.Context(), identity); !errors.Is(err, errBackfillCanaryWriteUnconfirmed) {
		t.Fatalf("wanted unconfirmed redirect, got %v", err)
	}

	for _, test := range []struct {
		origin string
		method string
		path   string
		supp   string
	}{
		{zeppDataHostGlobal + ".other.invalid", http.MethodDelete, backfillCanaryWireSummaryPath, ""},
		{zeppDataHostGlobal, http.MethodDelete, backfillCanaryWireDetailPath, ""},
		{zeppDataHostGlobal, http.MethodPut, backfillCanaryWireSummaryPath, ""},
		{zeppDataHostGlobal, http.MethodDelete, backfillCanaryWireSummaryPath, "apptoken"},
		{zeppDataHostGlobal, http.MethodDelete, backfillCanaryWireSummaryPath, "userid"},
	} {
		client.dataHost = test.origin
		input := url.Values{"trackid": {"123"}, "source": {backfillCanaryWireSource}}
		if test.supp != "" {
			input.Set(test.supp, "not-allowed")
		}
		if err := client.doBackfillCanaryWrite(t.Context(), test.method, test.path, input); !errors.Is(err, errBackfillCanaryWireInput) {
			t.Fatalf("wanted invalid request, got %v", err)
		}
	}
	if requests != 1 {
		t.Fatalf("expected exactly one attempted request, got %d", requests)
	}
}

func TestBackfillCanaryWireRequiresExplicitSuccess(t *testing.T) {
	for _, test := range []struct {
		status int
		body   string
		want   error
	}{
		{http.StatusOK, `{"code":1}`, nil},
		{http.StatusOK, `{"code":"1"}`, nil},
		{http.StatusUnauthorized, "secret from server", errBackfillHistoryUnauthorized},
		{http.StatusForbidden, "", errBackfillHistoryUnauthorized},
		{http.StatusOK, `{"code":"0108","message":"secret from server"}`, errBackfillHistoryUnauthorized},
		{http.StatusBadRequest, `{"code":401,"message":{"secret":"from server"}}`, errBackfillHistoryUnauthorized},
		{http.StatusOK, `{"code":27,"message":"Token expired"}`, errBackfillHistoryUnauthorized},
		{http.StatusOK, `{"code":0}`, errBackfillCanaryWriteUnconfirmed},
		{http.StatusOK, `{"code":1}garbage`, errBackfillCanaryWriteUnconfirmed},
		{http.StatusOK, "secret from server", errBackfillCanaryWriteUnconfirmed},
		{http.StatusOK, strings.Repeat(" ", backfillCanaryWireMaxReply+1), errBackfillCanaryWriteUnconfirmed},
		{http.StatusServiceUnavailable, `{"code":1}`, errBackfillCanaryWriteUnconfirmed},
	} {
		err := parseBackfillCanaryWriteReply(test.status, strings.NewReader(test.body))
		if !errors.Is(err, test.want) {
			t.Fatalf("status %d: wanted %v, got %v", test.status, test.want, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatal("server response leaked in an error")
		}
	}
}
