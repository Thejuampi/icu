package icu

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type backfillHistoryTransport func(*http.Request) (*http.Response, error)

func (f backfillHistoryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBackfillHistoryFailClosed(t *testing.T) {
	const done = `{"code":1,"data":{"next":-1,"summary":[]}}`
	const first = `{"code":1,"data":{"next":100,"summary":[{"trackid":"100","end_time":"110"}]}}`
	const later = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"200","end_time":"210"}]}}`
	const mismatchedSource = `{"code":1,"data":{"next":-1,"summary":[{"trackid":"100","end_time":"110","source":"different"}]}}`
	tests := []struct {
		name    string
		status  int
		bodies  []string
		want    error
		records int
		calls   int
	}{
		{"complete out of order", 200, []string{first, later}, nil, 2, 2},
		{"explicit empty", 200, []string{done}, nil, 0, 1},
		{"missing code", 200, []string{`{"data":{"next":-1,"summary":[]}}`}, errBackfillHistoryIncomplete, 0, 1},
		{"missing data", 200, []string{`{"code":0}`}, errBackfillHistoryIncomplete, 0, 1},
		{"missing summary", 200, []string{`{"code":1,"data":{"next":-1}}`}, errBackfillHistoryIncomplete, 0, 1},
		{"bad record", 200, []string{`{"code":1,"data":{"next":-1,"summary":[{"trackid":"100","end_time":"99"}]}}`}, errBackfillHistoryIncomplete, 0, 1},
		{"cycle no followup", 200, []string{first, first}, errBackfillHistoryIncomplete, 0, 2},
		{"conflicting track sources", 200, []string{first, mismatchedSource}, errBackfillHistoryIncomplete, 0, 2},
		{"401 no followup", 401, []string{first}, errBackfillHistoryUnauthorized, 0, 1},
		{"403 no followup", 403, []string{first}, errBackfillHistoryUnauthorized, 0, 1},
		{"logical auth", 200, []string{`{"code":401}`}, errBackfillHistoryUnauthorized, 0, 1},
		{"redirect", 307, []string{first}, errBackfillHistoryIncomplete, 0, 1},
		{"upstream down", 500, []string{first}, errBackfillHistoryIncomplete, 0, 1},
		{"trailing", 200, []string{done + " {}"}, errBackfillHistoryIncomplete, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			transport := backfillHistoryTransport(func(r *http.Request) (*http.Response, error) {
				if calls >= len(tt.bodies) {
					t.Fatal("unexpected follow-up")
				}
				if r.Method != http.MethodGet || r.URL.Path != "/v1/sport/run/history.json" || r.Header.Get("Appplatform") != "web" {
					t.Fatal("not the existing web read flow")
				}
				if calls == 0 && r.URL.Query().Get("trackid") != "" || calls == 1 && r.URL.Query().Get("trackid") != "100" {
					t.Fatal("wrong cursor")
				}
				body := tt.bodies[calls]
				calls++
				return &http.Response{StatusCode: tt.status, Header: http.Header{"Location": {"/unexpected"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			c := NewZeppClientFromAuth(&ZeppAuthResult{LoginToken: "mock", AppToken: "mock", UserID: "1"}, WithZeppHTTPClient(&http.Client{Transport: transport}))
			result, err := c.readZeppBackfillHistory(context.Background())
			if !errors.Is(err, tt.want) || len(result) != tt.records || calls != tt.calls {
				t.Fatalf("got error %v count %d calls %d", err, len(result), calls)
			}
		})
	}
}

func TestBackfillHistoryOverlap(t *testing.T) {
	existing := []zeppBackfillInterval{{Start: 100, End: 110}}
	for _, x := range []struct {
		r    zeppBackfillInterval
		want bool
	}{
		{zeppBackfillInterval{Start: 90, End: 99}, false},
		{zeppBackfillInterval{Start: 90, End: 100}, true},
		{zeppBackfillInterval{Start: 100, End: 111}, true},
		{zeppBackfillInterval{Start: 104, End: 105}, true},
		{zeppBackfillInterval{Start: 105, End: 120}, true},
		{zeppBackfillInterval{Start: 111, End: 120}, false},
	} {
		if got := zeppBackfillOverlap(x.r, existing); got != x.want {
			t.Errorf("%+v: got overlap %v", x.r, got)
		}
	}
}

func TestPlanRideBackfill(t *testing.T) {
	activities := []Activity{
		{ID: "i8", Type: "VirtualRide", StartDate: "2026-11-19T10:00:00Z", ElapsedTime: 300},
		{ID: "i9", Type: "Ride", StartDate: "2026-11-19T10:04:00Z", ElapsedTime: 300},
		{ID: "i7", Type: "Ride", StartDate: "2026-11-19T09:00:00-05:00", ElapsedTime: 360},
		{ID: "i6", Type: "Ride", StartDate: "2026-11-19T08:00:00Z", ElapsedTime: 360},
		{ID: "i5", Type: "GravelRide", StartDate: "invalid", ElapsedTime: 180},
		{ID: "i4", Type: "Run", StartDate: "2026-11-19T08:00:00Z", ElapsedTime: 360},
	}
	at, _ := time.Parse(time.RFC3339, "2026-11-19T08:02:00Z")
	got := planRideBackfill(activities, []zeppBackfillInterval{{Start: at.Unix(), End: at.Unix() + 30}})
	want := []string{"invalid-source", "overlap", "source-conflict", "source-conflict", "not-in-feed"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Status != want[i] {
			t.Errorf("got item %+v, want %s", got[i], want[i])
		}
	}
	if got[4].TrackID == 0 || got[4].EndTime-got[4].TrackID != 360 {
		t.Errorf("invalid parsed interval %+v", got[4])
	}
}
