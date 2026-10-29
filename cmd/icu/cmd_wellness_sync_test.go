package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
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
	hrvBody    string
	hrvQueries []string
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
			f.hrvQueries = append(f.hrvQueries, r.URL.RawQuery)
			if f.hrvBody != "" {
				_, _ = io.WriteString(w, f.hrvBody)

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
	if code != 0 || len(fake.puts) != 0 || !strings.Contains(out, "APPLY (nothing to write)") || !strings.Contains(out, "\nnothing to write\n") {
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

// Synthetic partial sleep: intervals.icu has 5000 s (about 1.4 h) while the
// fake Zepp has a full 18000 s night on 2026-08-11.
const syncExistingPartial = `[{"id":"2026-08-10","restingHR":50,"hrv":40,"sleepSecs":18000,"sleepScore":80},` +
	`{"id":"2026-08-11","restingHR":50,"hrv":44.5,"sleepSecs":5000,"sleepScore":30}]`

//nolint:paralleltest // sets env
func TestWellnessSyncFlagsPartialSleep(t *testing.T) {
	fake := &syncFake{existing: syncExistingPartial}
	code, out, errOut := runSync(t, fake.serve(t))
	if code != 0 || len(fake.puts) != 0 {
		t.Fatalf("code=%d puts=%v stderr=%s", code, fake.puts, errOut)
	}
	for _, want := range []string{
		"NOTE",
		"2026-08-11  sleepSecs   5000       18000  keep    partial sync?",
		"note: 2026-08-11 sleepSecs in intervals.icu is under half of Zepp's",
		"icu wellness sync --oldest 2026-08-11 --newest 2026-08-11 --overwrite --fields sleepSecs,sleepScore --timezone America/New_York",
		"nothing to write",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "2026-08-10  sleepSecs") && strings.Count(out, "partial sync?") != 1 {
		t.Errorf("only 2026-08-11 should be flagged:\n%s", out)
	}

	code, out, _ = runSync(t, fake.serve(t), "--json")
	var parsed wellnessSyncOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil || code != 0 {
		t.Fatalf("json %v %s", err, out)
	}
	if len(parsed.PartialSleep) != 1 || parsed.PartialSleep[0].Date != "2026-08-11" ||
		!strings.Contains(parsed.PartialSleep[0].Suggest, "--overwrite --fields sleepSecs,sleepScore") {
		t.Fatalf("partialSleep %+v", parsed.PartialSleep)
	}
	if !strings.Contains(out, `"note": "suspected-partial-sync"`) {
		t.Fatalf("missing note in json:\n%s", out)
	}

	code, _, _ = runSync(t, fake.serve(t), "--oldest", "2026-08-11", "--newest", "2026-08-11",
		"--overwrite", "--fields", "sleepSecs,sleepScore", "--apply")
	if code != 0 || len(fake.puts) != 1 || fake.puts[0] != `[{"id":"2026-08-11","sleepSecs":18000,"sleepScore":80}]` {
		t.Fatalf("code=%d puts=%v", code, fake.puts)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncHRVWindowCoversDayBeforeOldest(t *testing.T) {
	// 20:00 New York on 2026-08-09 (oldest-1) = 00:00Z on 08-10: the evening
	// rule puts it on 08-10 (oldest), so the HRV fetch must start a day early.
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 8, 9, 20, 0, 0, 0, ny).UnixMilli()
	fake := &syncFake{existing: `[]`, hrvBody: `{"items":[{"timestamp":` + itoa64(ts) + `,"value":"61.5"}]}`}
	code, out, errOut := runSync(t, fake.serve(t), "--fields", "hrv", "--oldest", "2026-08-10", "--newest", "2026-08-10", "--json")
	if code != 0 || len(fake.hrvQueries) != 2 {
		t.Fatalf("code=%d queries=%v stderr=%s", code, fake.hrvQueries, errOut)
	}
	from := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC).UnixMilli()
	to := time.Date(2026, 8, 11, 23, 59, 59, 999000000, time.UTC).UnixMilli()
	for _, q := range fake.hrvQueries {
		if !strings.Contains(q, "from="+itoa64(from)) || !strings.Contains(q, "to="+itoa64(to)) {
			t.Fatalf("hrv window query %s, want from=%d to=%d", q, from, to)
		}
	}
	var parsed wellnessSyncOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil || len(parsed.Changes) != 1 ||
		parsed.Changes[0].Date != "2026-08-10" || parsed.Changes[0].Zepp != 61.5 {
		t.Fatalf("changes %+v err=%v", parsed.Changes, err)
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncRefusesNonZeppHost(t *testing.T) {
	fake := &syncFake{existing: `[]`}
	srv := fake.serve(t)
	code, _, errOut := runSync(t, srv, "--zepp-base-url", "https://zepp.example.org", "--apply")
	if code == 0 || fake.hits != 0 || len(fake.puts) != 0 || !strings.Contains(errOut, "app token") || !strings.Contains(errOut, "--allow-insecure-zepp-host") {
		t.Fatalf("code=%d hits=%d stderr=%s", code, fake.hits, errOut)
	}
}

func TestCheckZeppBaseURLAllowInsecureHostWarnsWithSecret(t *testing.T) {
	t.Parallel()
	for _, secret := range []string{zeppSecretAppToken, zeppSecretPassword} {
		var stderr bytes.Buffer
		if err := checkZeppBaseURL("https://zepp.example.org", secret, true, &stderr); err != nil || !strings.Contains(stderr.String(), "warning:") || !strings.Contains(stderr.String(), secret) {
			t.Fatalf("%s: err=%v stderr=%q", secret, err, stderr.String())
		}
		stderr.Reset()
		err := checkZeppBaseURL("https://zepp.example.org", secret, false, &stderr)
		if err == nil || !strings.Contains(err.Error(), secret) || stderr.Len() != 0 {
			t.Fatalf("%s refused: err=%v stderr=%q", secret, err, stderr.String())
		}
		// plain http to a non-loopback host stays refused even when allowed.
		if err := checkZeppBaseURL("http://zepp.example.org", secret, true, &stderr); err == nil {
			t.Fatalf("%s: plain http allowed", secret)
		}
	}
	if !strings.Contains(zeppSecretAppToken, "app token") || !strings.Contains(zeppSecretPassword, "password") {
		t.Fatal("secret names")
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncJSONPartialSleepEmpty(t *testing.T) {
	fake := &syncFake{existing: syncExisting}
	_, out, _ := runSync(t, fake.serve(t), "--json")
	if !strings.Contains(out, `"partialSleep": []`) {
		t.Fatalf("want empty partialSleep:\n%s", out)
	}
}

//nolint:paralleltest // sets env
func TestZeppLoginRefusesNonZeppHostMentionsPassword(t *testing.T) {
	withZeppTestEnv(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"icu", "zepp", "login", "--email", "u@example.com", "--password", "secret", "--zepp-base-url", "https://zepp.example.org"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "password") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncSuggestionCarriesRunTargets(t *testing.T) {
	fake := &syncFake{existing: syncExistingPartial}
	srv := fake.serve(t)
	_, out, errOut := runSync(t, srv, "--json", "--athlete-id", "i1", "--timezone", "America/Chicago", "--lock")
	var parsed wellnessSyncOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil || len(parsed.PartialSleep) != 1 {
		t.Fatalf("json %v %s %s", err, out, errOut)
	}
	got := parsed.PartialSleep[0].Suggest
	want := "icu wellness sync --oldest 2026-08-11 --newest 2026-08-11 --overwrite --fields sleepSecs,sleepScore " +
		"--timezone America/Chicago --athlete-id i1 --base-url " + srv.URL + " --zepp-base-url " + srv.URL + " --lock --apply"
	if got != want {
		t.Fatalf("suggest\n got %s\nwant %s", got, want)
	}
	for _, secret := range []string{"api-key", "token", "password", "mock-app", "mock-login"} {
		if strings.Contains(got, secret) {
			t.Fatalf("suggestion leaks %q: %s", secret, got)
		}
	}
}

func TestPartialSleepSuggestionsContext(t *testing.T) {
	t.Parallel()
	changes := []icu.WellnessSyncChange{{Date: "2026-08-11", Field: icu.SyncFieldSleepSecs, Note: icu.SyncNotePartialSleep}}
	got := partialSleepSuggestions(changes, suggestionContext{
		baseURL: icu.BaseURL, zeppBaseURL: "https://zepp.example.org", allowInsecure: true, timezone: "Etc/GMT+3",
	})
	want := "icu wellness sync --oldest 2026-08-11 --newest 2026-08-11 --overwrite --fields sleepSecs,sleepScore " +
		"--timezone Etc/GMT+3 --zepp-base-url https://zepp.example.org --allow-insecure-zepp-host --apply"
	if len(got) != 1 || got[0].Suggest != want {
		t.Fatalf("got %+v\nwant %s", got, want)
	}
	if strings.Contains(got[0].Suggest, "--base-url") {
		t.Fatal("real API base URL must not be added")
	}
	if got := partialSleepSuggestions(changes, suggestionContext{overwrite: true}); len(got) != 0 {
		t.Fatalf("overwrite run must not suggest: %+v", got)
	}
	if shellQuote("") != "''" || shellQuote("a b") != "'a b'" || shellQuote("it's") != `'it'\''s'` {
		t.Fatal("shellQuote")
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncOverwriteRunSkipsSuggestion(t *testing.T) {
	fake := &syncFake{existing: syncExistingPartial}
	code, out, _ := runSync(t, fake.serve(t), "--overwrite")
	if code != 0 || !strings.Contains(out, "partial sync?") || strings.Contains(out, "note:") || !strings.Contains(out, "overwrite") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestPartialSleepSuggestionQuotingRoundTripsThroughShell(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	tz := "a b'c$(x)"
	changes := []icu.WellnessSyncChange{{Date: "2026-08-11", Field: icu.SyncFieldSleepSecs, Note: icu.SyncNotePartialSleep}}
	got := partialSleepSuggestions(changes, suggestionContext{timezone: tz, athleteID: "i1"})
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	want := `icu wellness sync --oldest 2026-08-11 --newest 2026-08-11 --overwrite --fields sleepSecs,sleepScore ` +
		`--timezone 'a b'\''c$(x)' --athlete-id i1 --apply`
	if got[0].Suggest != want {
		t.Fatalf("suggest\n got %s\nwant %s", got[0].Suggest, want)
	}
	// Parse the suggestion back with a real shell: icu is replaced by a
	// function printing one argument per line.
	script := `icu() { for a in "$@"; do printf '%s\n' "$a"; done; }; ` + got[0].Suggest
	out, err := exec.CommandContext(t.Context(), "sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	wantArgs := []string{
		"wellness", "sync", "--oldest", "2026-08-11", "--newest", "2026-08-11", "--overwrite",
		"--fields", "sleepSecs,sleepScore", "--timezone", tz, "--athlete-id", "i1", "--apply",
	}
	if strings.Join(args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("round trip\n got %q\nwant %q", args, wantArgs)
	}
}

func TestRedactURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
		stripped bool
	}{
		{"", "", false},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080", false},
		{"https://u:p@h.example/x?k=SECRET#frag", "https://h.example/x", true},
		{"https://u@h.example", "https://h.example", true},
		{"http://h.example/x?", "http://h.example/x", true},
		{"http://h.example/%zz", "", true},
	}
	for _, tc := range cases {
		got, stripped := redactURL(tc.in)
		if got != tc.want || stripped != tc.stripped {
			t.Errorf("redactURL(%q) = %q,%v want %q,%v", tc.in, got, stripped, tc.want, tc.stripped)
		}
	}
}

//nolint:paralleltest // sets env
func TestWellnessSyncSuggestionRedactsURLSecrets(t *testing.T) {
	fake := &syncFake{existing: syncExistingPartial}
	srv := fake.serve(t)
	withSecret := strings.Replace(srv.URL, "http://", "http://user:SECRETPW@", 1)
	for _, asJSON := range []bool{false, true} {
		extra := []string{"--base-url", withSecret, "--zepp-base-url", withSecret}
		if asJSON {
			extra = append(extra, "--json")
		}
		code, out, errOut := runSync(t, srv, extra...)
		if code != 0 {
			t.Fatalf("code=%d stderr=%s", code, errOut)
		}
		if strings.Contains(out+errOut, "SECRETPW") || strings.Contains(out+errOut, "user:") {
			t.Fatalf("secret leaked (json=%v):\nstdout=%s\nstderr=%s", asJSON, out, errOut)
		}
		if !strings.Contains(out, "--base-url "+srv.URL+" --zepp-base-url "+srv.URL) {
			t.Fatalf("suggestion lost the mock URL:\n%s", out)
		}
		if !strings.Contains(errOut, "credentials, query or fragment removed") {
			t.Fatalf("missing warning:\n%s", errOut)
		}
	}
	// A plain URL produces no warning.
	_, _, errOut := runSync(t, srv)
	if strings.Contains(errOut, "removed") {
		t.Fatalf("unexpected warning: %s", errOut)
	}
}

func TestPartialSleepSuggestionsQueryStripped(t *testing.T) {
	t.Parallel()
	base, _ := redactURL("https://u:p@mock.example/api?k=SECRET#f")
	zepp, _ := redactURL("http://u:p@127.0.0.1:9/z?token=SECRET")
	changes := []icu.WellnessSyncChange{{Date: "2026-08-11", Field: icu.SyncFieldSleepSecs, Note: icu.SyncNotePartialSleep}}
	got := partialSleepSuggestions(changes, suggestionContext{baseURL: base, zeppBaseURL: zepp})
	if s := got[0].Suggest; strings.Contains(s, "SECRET") || strings.Contains(s, "u:p") ||
		!strings.Contains(s, "--base-url https://mock.example/api --zepp-base-url http://127.0.0.1:9/z") {
		t.Fatalf("suggest %s", s)
	}
	realAPI := partialSleepSuggestions(changes, suggestionContext{baseURL: "https://INTERVALS.icu/"})
	if strings.Contains(realAPI[0].Suggest, "--base-url") {
		t.Fatalf("real API must not be added: %s", realAPI[0].Suggest)
	}
}
