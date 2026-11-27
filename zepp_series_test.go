package icu_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Thejuampi/icu"
)

func TestZeppLegacyEventsSendAppHeadersUserIDAndFollowNext(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		q, _ := url.ParseQuery(record.Query)
		if record.Path != "/users/u1/events" || q.Get("userId") != "u1" || q.Get("limit") != "200" ||
			q.Get("eventType") != "all_day_stress" || record.Header.Get("Appname") == "" ||
			record.Header.Get("Appplatform") == "" || record.Header.Get("Apptoken") == "" {
			return http.StatusInternalServerError, `{"code":0}`
		}

		if q.Get("from") == "1780272000000" {
			return http.StatusOK, `{"items":[{"timestamp":1780315200000,"avgStress":40}],"next":{"timestamp":1780401600000}}`
		}

		if q.Get("from") == "1780401600000" {
			// The next page repeats nothing new after its first item.
			return http.StatusOK, `{"items":[{"timestamp":1780401600000,"avgStress":41}],"next":{"timestamp":1780401600000}}`
		}

		return http.StatusInternalServerError, `{"code":0}`
	})
	t.Cleanup(srv.Close)

	days, err := newTestZeppClient(srv).StressDays(t.Context(), "2026-06-01", "2026-06-02")
	if err != nil {
		t.Fatalf("StressDays: %v", err)
	}

	if len(days) != 2 || len(srv.records) != 2 {
		t.Fatalf("days = %+v, requests = %d, want 2 and 2", days, len(srv.records))
	}
}

func TestZeppLegacyEventsNameTheSeriesOnRejection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		fetch func(*icu.ZeppClient) error
	}{
		{"stress", func(c *icu.ZeppClient) error {
			_, err := c.StressDays(t.Context(), "2026-06-01", "2026-06-01")
			return err
		}},
		{"spo2", func(c *icu.ZeppClient) error {
			_, err := c.SpO2Readings(t.Context(), "2026-06-01", "2026-06-01")
			return err
		}},
		{"pai", func(c *icu.ZeppClient) error {
			_, err := c.PAIDays(t.Context(), "2026-06-01", "2026-06-01")
			return err
		}},
	}

	srv := newZeppTestServer(func(zeppRequestRecord) (int, string) { return http.StatusInternalServerError, `{}` })
	t.Cleanup(srv.Close)

	for _, tc := range cases {
		err := tc.fetch(newTestZeppClient(srv))
		if !errors.Is(err, icu.ErrZeppSeriesRejected) || !errors.Is(err, icu.ErrZeppServer) {
			t.Fatalf("%s: err = %v, want ErrZeppSeriesRejected", tc.name, err)
		}

		msg := err.Error()
		if !strings.Contains(msg, "zepp "+tc.name+":") || !strings.Contains(msg, "HTTP 500") ||
			!strings.Contains(msg, "likely cause") {
			t.Fatalf("%s: message %q lacks series, status or cause", tc.name, msg)
		}
	}
}

func TestZeppSeriesKeepUnauthorized(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(zeppRequestRecord) (int, string) { return http.StatusUnauthorized, `{}` })
	t.Cleanup(srv.Close)

	_, err := newTestZeppClient(srv).SportLoad(t.Context(), "2026-06-01", "2026-06-01")
	if !errors.Is(err, icu.ErrZeppUnauthorized) || errors.Is(err, icu.ErrZeppSeriesRejected) {
		t.Fatalf("err = %v, want ErrZeppUnauthorized only", err)
	}
}

func TestZeppSportStatisticsSendStartAndEndDay(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		q, _ := url.ParseQuery(record.Query)
		if q.Get("startDay") != "2026-06-01" || q.Get("endDay") != "2026-06-02" || q.Has("from") ||
			record.Header.Get("Appname") == "" {
			return http.StatusInternalServerError, `{}`
		}

		return http.StatusOK, `{"items":[]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)
	if _, err := client.SportLoad(t.Context(), "2026-06-01", "2026-06-02"); err != nil {
		t.Fatalf("SportLoad: %v", err)
	}

	if _, err := client.VO2Max(t.Context(), "2026-06-01", "2026-06-02"); err != nil {
		t.Fatalf("VO2Max: %v", err)
	}

	if !strings.HasSuffix(srv.Last().Path, "/WatchSportStatistics/VO2_MAX") {
		t.Fatalf("path = %s", srv.Last().Path)
	}
}

func TestZeppSportStatisticsNameTheSeriesOnRejection(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(zeppRequestRecord) (int, string) { return http.StatusInternalServerError, `{}` })
	t.Cleanup(srv.Close)

	_, err := newTestZeppClient(srv).VO2Max(t.Context(), "2026-06-01", "2026-06-01")
	if !errors.Is(err, icu.ErrZeppSeriesRejected) || !strings.Contains(err.Error(), "zepp vo2:") {
		t.Fatalf("err = %v", err)
	}
}

func TestZeppManualDataSendsUserIDAndTypeAndFiltersByDate(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		q, _ := url.ParseQuery(record.Query)
		if q.Get("userid") != "u1" || q.Get("type") != "sleep" || q.Has("from") {
			return http.StatusBadRequest, `{}`
		}

		return http.StatusOK, `{"items":[{"timestamp":1780315200000},{"timestamp":1790000000000}]}`
	})
	t.Cleanup(srv.Close)

	entries, err := newTestZeppClient(srv).ManualData(t.Context(), "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("ManualData: %v", err)
	}

	if len(entries) != 1 || entries[0].Timestamp != 1780315200000 {
		t.Fatalf("entries = %+v, want only the in-range entry", entries)
	}
}

func TestZeppManualDataRejections(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(zeppRequestRecord) (int, string) { return http.StatusBadRequest, `{}` })
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	_, err := client.ManualDataType(t.Context(), "weight", "2026-06-01", "2026-06-01")
	if !errors.Is(err, icu.ErrZeppSeriesRejected) || !strings.Contains(err.Error(), `record type "weight"`) {
		t.Fatalf("err = %v", err)
	}

	if _, err := client.ManualDataType(t.Context(), "", "2026-06-02", "2026-06-01"); !errors.Is(err, icu.ErrZeppDateRange) {
		t.Fatalf("inverted range err = %v", err)
	}

	if _, err := client.ManualDataType(t.Context(), "", "bad", "2026-06-01"); err == nil {
		t.Fatalf("expected a date error")
	}
}

func TestZeppBloodPressureUserSendsDaysAndToDate(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		q, _ := url.ParseQuery(record.Query)
		if q.Get("days") != "2" || q.Get("toDate") != "2026-06-02" || q.Get("sourceArrayStr") == "" {
			return http.StatusBadRequest, `{}`
		}

		return http.StatusOK, `{"items":[{"timestamp":1780300800000,"systolic":120,"diastolic":80}]}`
	})
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	got, err := client.BloodPressureUser(t.Context(), "2026-06-01", "2026-06-02")
	if err != nil || len(got) != 1 || got[0].Systolic != 120 {
		t.Fatalf("got %+v, err %v", got, err)
	}

	_, err = client.BloodPressureUser(t.Context(), "2026-06-01", "2026-06-01")
	if !errors.Is(err, icu.ErrZeppSeriesRejected) || !strings.Contains(err.Error(), "zepp blood-pressure-user:") {
		t.Fatalf("err = %v", err)
	}
}

func workoutServer(detail string) *zeppTestServer {
	return newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		q, _ := url.ParseQuery(record.Query)

		switch record.Path {
		case "/v1/sport/run/history.json":
			if q.Get("trackid") == "" {
				return http.StatusOK, `{"code":1,"data":{"next":1717000000,"summary":[` +
					`{"trackid":"1717300000","source":"other.source"}]}}`
			}

			return http.StatusOK, `{"code":1,"data":{"next":-1,"summary":[` +
				`{"trackid":1717200000,"source":"hist.source"},{"trackid":"1717100000"}]}}`
		case "/v1/sport/run/detail.json":
			if q.Get("source") != "hist.source" && q.Get("source") != icu.ZeppWorkoutDefaultSource {
				return http.StatusOK, `{"code":1,"data":{}}`
			}

			return http.StatusOK, detail
		}

		return http.StatusNotFound, err404
	})
}

func TestZeppWorkoutLooksUpTheHistorySource(t *testing.T) {
	t.Parallel()

	srv := workoutServer(`{"code":1,"data":{"trackid":1717200000,"source":"hist.source"}}`)
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	detail, err := client.Workout(t.Context(), "ride", "1717200000")
	if err != nil {
		t.Fatalf("Workout: %v", err)
	}

	if detail.TrackID != "1717200000" {
		t.Fatalf("trackId = %q", detail.TrackID)
	}

	source, err := client.WorkoutSource(t.Context(), "1717100000")
	if err != nil || source != icu.ZeppWorkoutDefaultSource {
		t.Fatalf("source = %q, err %v, want the default source", source, err)
	}
}

func TestZeppWorkoutErrors(t *testing.T) {
	t.Parallel()

	srv := workoutServer(`{"code":1,"data":{}}`)
	t.Cleanup(srv.Close)

	client := newTestZeppClient(srv)

	if _, err := client.Workout(t.Context(), "run", "1700000000"); !errors.Is(err, icu.ErrZeppWorkoutNotFound) {
		t.Fatalf("missing trackid err = %v", err)
	}

	if _, err := client.Workout(t.Context(), "nope", "1717200000"); !errors.Is(err, icu.ErrZeppUnknownSport) {
		t.Fatalf("unknown sport err = %v", err)
	}

	if _, err := client.WorkoutSource(t.Context(), "abc"); err == nil {
		t.Fatalf("expected a trackid parse error")
	}

	_, err := client.WorkoutWithSource(t.Context(), "1717200000", "")
	if !errors.Is(err, icu.ErrZeppWorkoutEmpty) || !strings.Contains(err.Error(), `source "run.mi.com"`) {
		t.Fatalf("empty detail err = %v", err)
	}

	_, err = client.WorkoutWithSource(t.Context(), "1717200000", "wrong.source")
	if !errors.Is(err, icu.ErrZeppWorkoutEmpty) {
		t.Fatalf("wrong source err = %v", err)
	}
}

func TestZeppWorkoutDetailRejectionNamesTheSource(t *testing.T) {
	t.Parallel()

	srv := newZeppTestServer(func(zeppRequestRecord) (int, string) { return http.StatusBadRequest, `{}` })
	t.Cleanup(srv.Close)

	_, err := newTestZeppClient(srv).WorkoutWithSource(t.Context(), "1717200000", "x.source")
	if !errors.Is(err, icu.ErrZeppSeriesRejected) || !strings.Contains(err.Error(), `source "x.source"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestZeppLegacyEventsErrorsWhenPageCapIsHit(t *testing.T) {
	t.Parallel()

	pages := 0
	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		pages++
		q, _ := url.ParseQuery(record.Query)
		cur, _ := strconv.ParseInt(q.Get("from"), 10, 64)
		next := cur + 1

		return http.StatusOK, fmt.Sprintf(`{"items":[{"timestamp":%d,"avgStress":40}],"next":{"timestamp":%d}}`, cur, next)
	})
	t.Cleanup(srv.Close)

	_, err := newTestZeppClient(srv).StressDays(t.Context(), "2026-06-01", "2026-06-07")
	if !errors.Is(err, icu.ErrZeppTooManyPages) {
		t.Fatalf("err = %v, want ErrZeppTooManyPages", err)
	}

	if pages != 100 {
		t.Fatalf("pages = %d, want 100", pages)
	}

	if msg := err.Error(); !strings.Contains(msg, "zepp stress") || !strings.Contains(msg, "all_day_stress") || strings.Contains(msg, "u1") {
		t.Fatalf("message %q should name the series and omit the user id", msg)
	}
}

func TestZeppLegacyEventsSucceedsAtExactlyThePageCap(t *testing.T) {
	t.Parallel()

	pages := 0
	srv := newZeppTestServer(func(record zeppRequestRecord) (int, string) {
		pages++
		q, _ := url.ParseQuery(record.Query)
		cur, _ := strconv.ParseInt(q.Get("from"), 10, 64)
		item := fmt.Sprintf(`{"timestamp":%d,"avgStress":%d}`, cur, pages)
		if pages == 100 {
			return http.StatusOK, `{"items":[` + item + `]}`
		}

		return http.StatusOK, fmt.Sprintf(`{"items":[%s],"next":{"timestamp":%d}}`, item, cur+1)
	})
	t.Cleanup(srv.Close)

	days, err := newTestZeppClient(srv).StressDays(t.Context(), "2026-06-01", "2026-06-07")
	if err != nil {
		t.Fatalf("StressDays: %v", err)
	}

	if pages != 100 || len(days) != 100 {
		t.Fatalf("pages = %d, days = %d, want 100 and 100", pages, len(days))
	}
}

type failRoundTrip struct{}

func (failRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestZeppTransportErrorOmitsUserID(t *testing.T) {
	t.Parallel()

	const uid = "mock-user-42"
	auth := &icu.ZeppAuthResult{
		LoginToken:  "tok-abc",
		AppToken:    "test-app-token",
		UserID:      uid,
		CountryCode: "US",
	}
	client := icu.NewZeppClientFromAuth(
		auth,
		icu.WithZeppBaseURL("https://zepp.example.invalid"),
		icu.WithZeppEventsURL("https://zepp.example.invalid"),
		icu.WithZeppHTTPClient(&http.Client{Transport: failRoundTrip{}}),
	)

	_, err := client.StressDays(t.Context(), "2026-06-01", "2026-06-01")
	if err == nil {
		t.Fatalf("expected a transport error")
	}

	msg := err.Error()
	if strings.Contains(msg, uid) || strings.Contains(msg, "userId=") || strings.Contains(msg, "/users/"+uid) {
		t.Fatalf("transport error leaked the user id: %q", msg)
	}

	if !strings.Contains(msg, "/users/{id}/events") {
		t.Fatalf("transport error %q should keep the path shape", msg)
	}

	if !strings.Contains(msg, "connection refused") {
		t.Fatalf("transport error %q should keep the cause", msg)
	}
}

func TestZeppTransportPreservesContextErrorsAndOmitsUserID(t *testing.T) {
	t.Parallel()

	const uid = "mock-user-42"
	srv := newZeppTestServer(func(zeppRequestRecord) (int, string) {
		t.Error("the canceled client should not have issued a request")

		return http.StatusOK, `{"items":[]}`
	})
	t.Cleanup(srv.Close)

	auth := &icu.ZeppAuthResult{
		LoginToken:  "tok-abc",
		AppToken:    "test-app-token",
		UserID:      uid,
		CountryCode: "US",
	}
	client := icu.NewZeppClientFromAuth(
		auth,
		icu.WithZeppBaseURL(srv.server.URL),
		icu.WithZeppEventsURL(srv.server.URL),
		icu.WithZeppHTTPClient(srv.server.Client()),
	)

	type callFunc func(context.Context) error
	calls := []struct {
		name string
		kind string
		call callFunc
	}{
		{"stress canceled", "canceled", func(ctx context.Context) error {
			_, err := client.StressDays(ctx, "2026-06-01", "2026-06-01")
			return err
		}},
		{"stress deadline", "deadline", func(ctx context.Context) error {
			_, err := client.StressDays(ctx, "2026-06-01", "2026-06-01")
			return err
		}},
		{"doGet canceled", "canceled", func(ctx context.Context) error {
			_, err := client.BandData(ctx, "2026-06-01", "2026-06-01")
			return err
		}},
		{"v2 events canceled", "canceled", func(ctx context.Context) error {
			_, err := client.ReadinessDays(ctx, "2026-06-01", "2026-06-01")
			return err
		}},
		{"data-host canceled", "canceled", func(ctx context.Context) error {
			_, err := client.WeightRecords(ctx, "2026-06-01", "2026-06-01")
			return err
		}},
	}

	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, stop := context.WithCancel(t.Context())
			want := context.Canceled
			if tc.kind == "deadline" {
				stop()
				ctx, stop = context.WithTimeout(t.Context(), 0)
				want = context.DeadlineExceeded
			} else {
				stop()
			}
			defer stop()

			err := tc.call(ctx)
			if !errors.Is(err, want) {
				t.Fatalf("err = %v, want %v", err, want)
			}

			if msg := err.Error(); strings.Contains(msg, uid) || strings.Contains(msg, "userId=") {
				t.Fatalf("error leaked the user id: %q", msg)
			}
		})
	}
}
