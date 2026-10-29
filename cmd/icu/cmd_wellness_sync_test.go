package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncFake is one host serving the intervals.icu and Zepp endpoints that
// `wellness sync` uses, recording writes. Synthetic data only.
type syncFake struct {
	mu         sync.Mutex
	puts       []string
	hits       int
	bandStatus int
	hrvStatus  int
	bulkStatus int
	existing   string
}

func (f *syncFake) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.hits++
		switch {
		case r.URL.Path == "/v1/data/band_data.json":
			if f.bandStatus != 0 {
				w.WriteHeader(f.bandStatus)

				return
			}
			summary := base64.StdEncoding.EncodeToString([]byte(`{"slp":{"dp":60,"lt":240,"rhr":50,"ss":80}}`))
			_, _ = io.WriteString(w, `{"code":1,"data":[{"date_time":"2026-08-10","summary":"`+summary+`"},{"date_time":"2026-08-11","summary":"`+summary+`"}]}`)
		case r.URL.Path == "/v2/users/me/events":
			if f.hrvStatus != 0 {
				w.WriteHeader(f.hrvStatus)

				return
			}
			// 23:30 New York on the 10th = 03:30Z on the 11th -> day 11.
			ts := time.Date(2026, 8, 11, 3, 30, 0, 0, time.UTC).UnixMilli()
			_, _ = io.WriteString(w, `{"items":[{"timestamp":`+itoa64(ts)+`,"value":"44.5"}]}`)
		case strings.HasSuffix(r.URL.Path, "/wellness-bulk") && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			f.puts = append(f.puts, string(body))
			if f.bulkStatus != 0 {
				w.WriteHeader(f.bulkStatus)

				return
			}
			_, _ = io.WriteString(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/wellness"):
			_, _ = io.WriteString(w, f.existing)
		case strings.HasSuffix(r.URL.Path, "/athlete/i1"):
			_, _ = io.WriteString(w, `{"id":"i1","timezone":"America/New_York"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func runSync(t *testing.T, srv *httptest.Server, extra ...string) (int, string, string) {
	t.Helper()
	withZeppTestEnv(t)
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "k")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "i1")
	args := append([]string{
		"icu", "wellness", "sync", "--oldest", "2026-08-10", "--newest", "2026-08-11",
		"--base-url", srv.URL, "--zepp-base-url", srv.URL,
	}, extra...)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)

	return code, stdout.String(), stderr.String()
}

const syncExisting = `[{"id":"2026-08-10","restingHR":55,"hrv":40,"sleepSecs":18000,"sleepScore":80}]`

//nolint:paralleltest // sets env
func TestWellnessSyncDryRunNeverWrites(t *testing.T) {
	fake := &syncFake{existing: syncExisting}
	code, out, errOut := runSync(t, fake.serve(t))
	if code != 0 || len(fake.puts) != 0 {
		t.Fatalf("code=%d puts=%v stderr=%s", code, fake.puts, errOut)
	}
	for _, want := range []string{"DRY RUN", "(America/New_York)", "2026-08-10  restingHR   55         50     keep", "2026-08-10  sleepScore  80         80     same", "2026-08-11  hrv         -          44.5   fill", "would write 1 day(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncApplyFillOnly(t *testing.T) {
	fake := &syncFake{existing: syncExisting}
	code, out, errOut := runSync(t, fake.serve(t), "--apply", "--lock", "--json")
	if code != 0 || len(fake.puts) != 1 {
		t.Fatalf("code=%d puts=%v stderr=%s", code, fake.puts, errOut)
	}
	want := `[{"id":"2026-08-11","restingHR":50,"hrv":44.5,"sleepSecs":18000,"sleepScore":80,"locked":true}]`
	if fake.puts[0] != want {
		t.Fatalf("put %s\nwant %s", fake.puts[0], want)
	}
	var parsed wellnessSyncOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil || !parsed.Applied || parsed.DryRun || parsed.ZeppDays != 2 {
		t.Fatalf("json %v %s", err, out)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncApplyOverwrite(t *testing.T) {
	fake := &syncFake{existing: syncExisting}
	code, out, _ := runSync(t, fake.serve(t), "--apply", "--overwrite", "--fields", "restingHR")
	if code != 0 || len(fake.puts) != 1 || !strings.Contains(out, "APPLIED") || !strings.Contains(out, "wrote 2 day(s)") {
		t.Fatalf("code=%d puts=%v out=%s", code, fake.puts, out)
	}
	if fake.puts[0] != `[{"id":"2026-08-10","restingHR":50},{"id":"2026-08-11","restingHR":50}]` {
		t.Fatalf("put %s", fake.puts[0])
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncApplyNothingToWrite(t *testing.T) {
	fake := &syncFake{existing: `[{"id":"2026-08-10","restingHR":50,"sleepSecs":18000,"sleepScore":80},{"id":"2026-08-11","restingHR":50,"hrv":44.5,"sleepSecs":18000,"sleepScore":80}]`}
	code, out, _ := runSync(t, fake.serve(t), "--apply")
	if code != 0 || len(fake.puts) != 0 || !strings.Contains(out, "APPLY (nothing to write)") {
		t.Fatalf("code=%d puts=%v out=%s", code, fake.puts, out)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncZeppAuthFailureAborts(t *testing.T) {
	fake := &syncFake{existing: syncExisting, bandStatus: http.StatusUnauthorized}
	code, _, errOut := runSync(t, fake.serve(t), "--apply")
	if code == 0 || len(fake.puts) != 0 || !strings.Contains(errOut, "zepp login") {
		t.Fatalf("code=%d puts=%v stderr=%s", code, fake.puts, errOut)
	}
	fake = &syncFake{existing: syncExisting, hrvStatus: http.StatusForbidden}
	if code, _, errOut = runSync(t, fake.serve(t), "--fields", "hrv"); code == 0 || !strings.Contains(errOut, "401/403") {
		t.Fatalf("hrv auth code=%d stderr=%s", code, errOut)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncPartialFailureStillSyncsOtherSource(t *testing.T) {
	fake := &syncFake{existing: syncExisting, hrvStatus: http.StatusServiceUnavailable}
	code, out, errOut := runSync(t, fake.serve(t), "--apply")
	if code == 0 || len(fake.puts) != 1 || strings.Contains(fake.puts[0], "hrv") || !strings.Contains(errOut, "zepp hrv failed") || !strings.Contains(out, "wrote 1 day(s)") {
		t.Fatalf("code=%d puts=%v out=%s stderr=%s", code, fake.puts, out, errOut)
	}
	fake = &syncFake{existing: syncExisting, bandStatus: http.StatusBadGateway}
	if code, _, errOut = runSync(t, fake.serve(t)); code == 0 || !strings.Contains(errOut, "band data") {
		t.Fatalf("band partial code=%d stderr=%s", code, errOut)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncBulkWriteFailure(t *testing.T) {
	fake := &syncFake{existing: syncExisting, bulkStatus: http.StatusInternalServerError}
	code, _, errOut := runSync(t, fake.serve(t), "--apply")
	if code == 0 || !strings.Contains(errOut, "wellness-bulk") {
		t.Fatalf("code=%d stderr=%s", code, errOut)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncEmptyZeppRange(t *testing.T) {
	fake := &syncFake{existing: `[]`}
	srv := fake.serve(t)
	withZeppTestEnv(t)
	code, _, errOut := runSync(t, srv, "--fields", "sleepScore", "--timezone", "UTC", "--oldest", "2026-08-12", "--newest", "2026-08-12")
	if code != 0 || len(fake.puts) != 0 || !strings.Contains(errOut, "no Zepp wellness data") {
		t.Fatalf("code=%d stderr=%s", code, errOut)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncValidation(t *testing.T) {
	fake := &syncFake{existing: `[]`}
	srv := fake.serve(t)
	for _, extra := range [][]string{
		{"--fields", "weight"},
		{"--oldest", "2026-08-12", "--newest", "2026-08-11"},
		{"--newest", "bad"},
		{"--oldest", "bad"},
		{"--timezone", "Nowhere/City"},
		{"--zepp-base-url", "http://example.com"},
	} {
		if code, _, _ := runSync(t, srv, extra...); code == 0 {
			t.Errorf("%v accepted", extra)
		}
	}
	if len(fake.puts) != 0 {
		t.Fatal("validation wrote")
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncRequiresZeppLogin(t *testing.T) {
	fake := &syncFake{existing: `[]`}
	srv := fake.serve(t)
	withZeppTestEnv(t)
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("INTERVALS_ICU_API_KEY", "k")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "i1")
	t.Setenv("ZEPP_APP_TOKEN", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"icu", "wellness", "sync", "--base-url", srv.URL, "--zepp-base-url", srv.URL}, &stdout, &stderr)
	if code == 0 || len(fake.puts) != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestWellnessSyncDefaultRange(t *testing.T) {
	t.Parallel()
	oldest, newest, err := wellnessSyncRange(map[string]string{}, time.Date(2026, 8, 10, 23, 0, 0, 0, time.UTC))
	if err != nil || oldest != "2026-08-04" || newest != "2026-08-10" {
		t.Fatalf("%s %s %v", oldest, newest, err)
	}
	if !wantsField([]string{"hrv"}, "x", "hrv") || wantsField(nil, "hrv") {
		t.Fatal("wantsField")
	}
}
