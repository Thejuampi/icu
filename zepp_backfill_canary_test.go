package icu

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	backfillCanaryTestTrack   = "631066600"
	backfillCanaryTestSource  = "run.fit.huami.com"
	backfillCanaryTestToken   = "canary-synthetic"
	backfillCanaryHistoryPath = "/v1/sport/run/history.json"
	backfillCanaryDetailPath  = "/v1/sport/run/detail.json"
	backfillCanarySummaryPath = "/v1/sport/run/summary.json"
)

type canaryMockStep struct {
	method string
	path   string
	status int
	body   string
}

func TestRideBackfillCanary(t *testing.T) {
	const (
		empty       = `{"code":1,"data":{"next":-1,"summary":[]}}`
		present     = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"631066600","end_time":"631066608","source":"run.fit.huami.com"}]}}`
		otherSource = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"631066600","end_time":"631066608","source":"other"}]}}`
		absentSrc   = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"631066600","end_time":"631066608"}]}}`
		wrongEnd    = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"631066600","end_time":"631066607","source":"run.fit.huami.com"}]}}`
		two         = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"631066600","end_time":"631066608","source":"run.fit.huami.com"},{"trackid":"631066601","end_time":"631066609"}]}}`
		duplicate   = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"631066600","end_time":"631066608","source":"run.fit.huami.com"},{"trackid":"631066600","end_time":"631066608","source":"run.fit.huami.com"}]}}`
		writeOK     = `{"code":1}`
		upstreamErr = "upstream response must not leak"
		trackID     = 631066600
	)
	pre := canaryMockStep{http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, empty}
	detail := canaryMockStep{http.MethodPost, backfillCanaryDetailPath, http.StatusOK, writeOK}
	summary := canaryMockStep{http.MethodPost, backfillCanarySummaryPath, http.StatusOK, writeOK}
	verify := canaryMockStep{http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, present}
	undo := canaryMockStep{http.MethodDelete, backfillCanarySummaryPath, http.StatusOK, writeOK}

	for _, tt := range []struct {
		name  string
		steps []canaryMockStep
		id    int64
		want  error
	}{
		{"one upload readback and undo", []canaryMockStep{pre, detail, summary, verify, undo, pre}, trackID, nil},
		{"preflight overlap refuses", []canaryMockStep{verify}, 0, errBackfillCanaryOverlap},
		{"preflight unauthorized stops", []canaryMockStep{{http.MethodGet, backfillCanaryHistoryPath, http.StatusUnauthorized, ""}}, 0, errBackfillHistoryUnauthorized},
		{"detail unauthorized stops", []canaryMockStep{pre, {http.MethodPost, backfillCanaryDetailPath, http.StatusUnauthorized, ""}}, trackID, errBackfillHistoryUnauthorized},
		{"summary forbidden stops", []canaryMockStep{pre, detail, {http.MethodPost, backfillCanarySummaryPath, http.StatusForbidden, ""}}, trackID, errBackfillHistoryUnauthorized},
		{"readback logical auth stops", []canaryMockStep{pre, detail, summary, {http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, `{"code":401}`}}, trackID, errBackfillHistoryUnauthorized},
		{"undo unauthorized stops", []canaryMockStep{pre, detail, summary, verify, {http.MethodDelete, backfillCanarySummaryPath, http.StatusUnauthorized, ""}}, trackID, errBackfillHistoryUnauthorized},
		{"undo verification forbidden stops", []canaryMockStep{pre, detail, summary, verify, undo, {http.MethodGet, backfillCanaryHistoryPath, http.StatusForbidden, ""}}, trackID, errBackfillHistoryUnauthorized},
		{"detail unknown stops", []canaryMockStep{pre, {http.MethodPost, backfillCanaryDetailPath, http.StatusBadGateway, upstreamErr}}, trackID, errBackfillCanaryUncertain},
		{"summary unknown stops", []canaryMockStep{pre, detail, {http.MethodPost, backfillCanarySummaryPath, http.StatusBadGateway, upstreamErr}}, trackID, errBackfillCanaryUncertain},
		{"undo unknown stops", []canaryMockStep{pre, detail, summary, verify, {http.MethodDelete, backfillCanarySummaryPath, http.StatusBadGateway, upstreamErr}}, trackID, errBackfillCanaryUncertain},
		{"missing summary never deletes", []canaryMockStep{pre, detail, summary, pre}, trackID, errBackfillCanaryUncertain},
		{"different source never deletes", []canaryMockStep{pre, detail, summary, {http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, otherSource}}, trackID, errBackfillCanaryUncertain},
		{"missing source never deletes", []canaryMockStep{pre, detail, summary, {http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, absentSrc}}, trackID, errBackfillCanaryUncertain},
		{"wrong end never deletes", []canaryMockStep{pre, detail, summary, {http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, wrongEnd}}, trackID, errBackfillCanaryUncertain},
		{"second overlap never deletes", []canaryMockStep{pre, detail, summary, {http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, two}}, trackID, errBackfillCanaryUncertain},
		{"duplicate attribution never deletes", []canaryMockStep{pre, detail, summary, {http.MethodGet, backfillCanaryHistoryPath, http.StatusOK, duplicate}}, trackID, errBackfillCanaryUncertain},
		{"undo not reflected stops", []canaryMockStep{pre, detail, summary, verify, undo, verify}, trackID, errBackfillCanaryUncertain},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, calls := canaryMockClient(t, tt.steps)
			got, err := client.TestRideBackfill(t.Context(), bytes.NewReader(zeppFITFixture(t)))
			if got != tt.id || !errors.Is(err, tt.want) || *calls != len(tt.steps) {
				t.Fatalf("got track %d, err %v and %d calls; want %d, %v and %d", got, err, *calls, tt.id, tt.want, len(tt.steps))
			}
			if err != nil && strings.Contains(err.Error(), upstreamErr) {
				t.Fatal("error includes an upstream body")
			}
		})
	}
}

func TestRideBackfillCanaryInvalidFITMakesNoRequests(t *testing.T) {
	client, calls := canaryMockClient(t, nil)
	if got, err := client.TestRideBackfill(t.Context(), strings.NewReader("not FIT")); got != 0 || err == nil || *calls != 0 {
		t.Fatalf("got track %d, err %v and %d requests", got, err, *calls)
	}
}

func TestRideBackfillCanaryRejectsUntrustedOriginBeforeReading(t *testing.T) {
	client, calls := canaryMockClient(t, nil)
	client.dataHost = "https://example.invalid"
	got, err := client.TestRideBackfill(t.Context(), bytes.NewReader(zeppFITFixture(t)))
	if got != 0 || !errors.Is(err, errBackfillCanaryWireInput) || *calls != 0 {
		t.Fatalf("got track %d, err %v and %d requests", got, err, *calls)
	}
}

func canaryMockClient(t *testing.T, steps []canaryMockStep) (*ZeppClient, *int) {
	t.Helper()
	calls := 0
	transport := backfillHistoryTransport(func(req *http.Request) (*http.Response, error) {
		if calls == len(steps) {
			t.Fatalf("unexpected call %s %s", req.Method, req.URL.Path)
		}
		step := steps[calls]
		calls++
		if req.Method != step.method || req.URL.Path != step.path || !strings.HasPrefix(req.URL.String(), zeppDataHostGlobal+"/") ||
			req.Header.Get("Apptoken") != backfillCanaryTestToken || req.Header.Get("Appplatform") != "web" {
			t.Fatalf("request %d: unexpected method, destination or test session", calls)
		}
		assertCanaryMockRequest(t, req)
		return &http.Response{StatusCode: step.status, Body: io.NopCloser(strings.NewReader(step.body))}, nil
	})
	client := NewZeppClientFromAuth(&ZeppAuthResult{AppToken: backfillCanaryTestToken, UserID: "1"},
		WithZeppHTTPClient(&http.Client{Transport: transport}), WithZeppBaseURL(zeppDataHostGlobal))
	return client, &calls
}

func assertCanaryMockRequest(t *testing.T, req *http.Request) {
	t.Helper()
	switch req.Method {
	case http.MethodPost:
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.PostForm.Get("trackid") != backfillCanaryTestTrack || req.PostForm.Get("source") != backfillCanaryTestSource {
			t.Fatal("wrong upload identity")
		}
		switch req.URL.Path {
		case backfillCanaryDetailPath:
			if req.PostForm.Get("time") != "2;1;3" || req.PostForm.Get("power_meter") != "2,230;1,0;3,250;" {
				t.Fatal("missing FIT samples")
			}
		case backfillCanarySummaryPath:
			if req.PostForm.Get("end_time") != "631066608" || req.PostForm.Get("type") != "10" || req.PostForm.Get("dis") != "25" {
				t.Fatal("missing FIT summary")
			}
		}
	case http.MethodDelete:
		if q := req.URL.Query(); q.Get("trackid") != backfillCanaryTestTrack || q.Get("source") != backfillCanaryTestSource || q.Has("apptoken") {
			t.Fatal("wrong deletion identity")
		}
	}
}
