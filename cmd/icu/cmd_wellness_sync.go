package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	icu "github.com/Thejuampi/icu"
)

const wellnessSyncDefaultDays = 7

var errWellnessSyncPartial = errors.New("wellness sync: some Zepp sources failed")

func wellnessSyncCommand() *Command {
	return &Command{
		Name: "",
		Usage: "wellness sync [--oldest DATE] [--newest DATE] [--fields restingHR,hrv,sleepSecs,sleepScore] " +
			"[--apply] [--overwrite] [--lock] [--timezone TZ] [--zepp-base-url URL] [--allow-insecure-zepp-host] [--json]",
		Description: "Copy resting HR, HRV (rMSSD: readiness sleepHRV, the nightly value the Zepp app shows; else the HRVRMSSD sample mean, an approximation) and sleep from Zepp into intervals.icu wellness. " +
			"Dry run by default: prints the diff and writes nothing; --apply sends one PUT wellness-bulk. " +
			"Fills only empty fields unless --overwrite. HRV goes on the athlete's local day (18:00+ readings count for the next day). " +
			"Suspected partial sleep sync: when intervals.icu has sleepSecs below half of Zepp's (e.g. about 1.6 h against a full night), " +
			"the row is flagged \"partial sync?\" (still keep in fill-only mode) and the output suggests " +
			"--overwrite --fields sleepSecs,sleepScore for that day (sleepScore too: a partial sync also undercounts the score; " +
			"no suggestion when the run already used --overwrite). " +
			"If intervals.icu's own Zepp sync lags, a manual alternative is in the intervals.icu web app: " +
			"Settings > Zepp > Download old data > Wellness for the affected range. " +
			"That is a web-only action; this CLI does not automate it.",
		Schema: &CommandSchema{
			RejectPositionals: true,
			Flags: []CommandFlag{
				{Name: queryOldest, ValueName: "DATE", Description: "First day (YYYY-MM-DD); default 6 days before --newest."},
				{Name: queryNewest, ValueName: "DATE", Description: "Last day (YYYY-MM-DD); default today in the athlete timezone."},
				{Name: "fields", ValueName: "CSV", Description: "Fields to sync.", Default: strings.Join(icu.SyncFields(), ",")},
				{Name: "apply", ValueName: "BOOL", Description: "Write the planned changes (default: dry run).", Kind: commandFlagBoolean},
				{Name: "overwrite", ValueName: "BOOL", Description: "Replace values intervals.icu already has (default: fill empty fields only).", Kind: commandFlagBoolean},
				{Name: "lock", ValueName: "BOOL", Description: "Set locked on written days so intervals.icu's own sync keeps them.", Kind: commandFlagBoolean},
				{Name: "timezone", ValueName: "TZ", Description: "IANA timezone for the day mapping; default is the athlete profile timezone."},
				{Name: zeppBaseURLFlag, ValueName: "URL", Description: "Zepp data and events host (e.g. a local mock)."},
				{Name: zeppAllowInsecureHostFlag, ValueName: "BOOL", Description: "Allow a --zepp-base-url host that is neither loopback nor zepp.com/huami.com (the app token is sent there).", Kind: commandFlagBoolean},
				{Name: "json", ValueName: "BOOL", Description: "Print JSON instead of a table.", Kind: commandFlagBoolean},
				{Name: "api-key", ValueName: "KEY", Description: "Intervals.icu API key."},
				{Name: "athlete-id", ValueName: "ID", Description: "Intervals.icu athlete ID."},
			},
		},
		Run: runWellnessSync,
	}
}

type wellnessSyncOutput struct {
	DryRun    bool                     `json:"dryRun"`
	Applied   bool                     `json:"applied"`
	Timezone  string                   `json:"timezone"`
	Oldest    string                   `json:"oldest"`
	Newest    string                   `json:"newest"`
	Fields    []string                 `json:"fields"`
	Overwrite bool                     `json:"overwrite"`
	Lock      bool                     `json:"lock"`
	ZeppDays  int                      `json:"zeppDays"`
	Changes   []icu.WellnessSyncChange `json:"changes"`
	Writes    []icu.WellnessWrite      `json:"writes"`
	Warnings  []string                 `json:"warnings"`
	// PartialSleep lists days whose intervals.icu sleepSecs is below half of
	// Zepp's, with the suggested command to fix each.
	PartialSleep []partialSleepSuggestion `json:"partialSleep"`
	// HRVSamplesInfo lists days with HRVRMSSD samples but no readiness
	// sleepHRV. Information only: their mean is never written.
	HRVSamplesInfo []icu.ZeppWellnessDay `json:"hrvSamplesInfo"`
}

type partialSleepSuggestion struct {
	Date    string `json:"date"`
	Suggest string `json:"suggest"`
}

// partialSleepSuggestions builds, per flagged day, the command that replaces
// sleepSecs and sleepScore. It carries every non-secret target flag of the
// run (--base-url when not the real API, --zepp-base-url,
// --allow-insecure-zepp-host, --timezone, --athlete-id, --lock) so a
// suggestion copied from a mock run stays on the mock. Never --api-key or
// Zepp tokens. No suggestions when the run already used --overwrite.
func partialSleepSuggestions(changes []icu.WellnessSyncChange, ctx suggestionContext) []partialSleepSuggestion {
	out := []partialSleepSuggestion{}
	// A --base-url/--zepp-base-url that does not parse cannot be repeated
	// safely: dropping it would point the suggestion at the real API.
	if ctx.overwrite || ctx.unparsableURL {
		return out
	}
	for _, date := range icu.PartialSleepDates(changes) {
		args := []string{
			"icu", "wellness", "sync", "--oldest", date, "--newest", date,
			"--overwrite", "--fields", icu.SyncFieldSleepSecs + "," + icu.SyncFieldSleepScore,
		}
		if ctx.timezone != "" {
			args = append(args, "--timezone", ctx.timezone)
		}
		if ctx.athleteID != "" {
			args = append(args, "--athlete-id", ctx.athleteID)
		}
		if ctx.baseURL != "" && !icu.IsRealAPIBaseURL(ctx.baseURL) {
			args = append(args, "--base-url", ctx.baseURL)
		}
		if ctx.zeppBaseURL != "" {
			args = append(args, "--"+zeppBaseURLFlag, ctx.zeppBaseURL)
		}
		if ctx.allowInsecure {
			args = append(args, "--"+zeppAllowInsecureHostFlag)
		}
		if ctx.lock {
			args = append(args, "--lock")
		}
		args = append(args, "--apply")
		quoted := make([]string, len(args))
		for i, arg := range args {
			quoted[i] = shellQuote(arg)
		}
		out = append(out, partialSleepSuggestion{Date: date, Suggest: strings.Join(quoted, " ")})
	}

	return out
}

// redactURL drops userinfo, query and fragment from raw so a suggested
// command never repeats credentials; stripped reports whether anything was
// removed. A value that does not parse is dropped entirely (the caller then
// emits no suggestion). Credentials in the URL path are NOT redacted.
func redactURL(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", true
	}
	stripped := parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != ""
	parsed.User, parsed.RawQuery, parsed.ForceQuery, parsed.Fragment, parsed.RawFragment = nil, "", false, "", ""

	return parsed.String(), stripped
}

type suggestionContext struct {
	overwrite, lock, allowInsecure, unparsableURL bool
	timezone, athleteID, baseURL, zeppBaseURL     string
}

// shellQuote single-quotes arg when it has characters a POSIX shell would
// interpret.
func shellQuote(arg string) string {
	if arg != "" && strings.Trim(arg, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./:,=+@%") == "" {
		return arg
	}

	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func runWellnessSync(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
	fields, err := icu.ParseSyncFields(icu.StringFlag(flags, "fields", ""))
	if err != nil {
		return fmt.Errorf("%w: --fields: %w", errInvalidValue, err)
	}
	zeppBase := strings.TrimRight(icu.StringFlag(flags, zeppBaseURLFlag, ""), "/")
	if err := checkZeppBaseURL(zeppBase, zeppSecretAppToken, BoolFlag(flags, zeppAllowInsecureHostFlag), stderr); err != nil {
		return err
	}
	tzInfo, err := resolveAnalysisTimezone(client, flags)
	if err != nil {
		return err
	}
	oldest, newest, err := wellnessSyncRange(flags, tzInfo.now())
	if err != nil {
		return err
	}

	out := wellnessSyncOutput{
		DryRun: !BoolFlag(flags, "apply"), Timezone: tzInfo.timezone, Oldest: oldest, Newest: newest, Fields: fields, PartialSleep: []partialSleepSuggestion{},
		Overwrite: BoolFlag(flags, "overwrite"), Lock: BoolFlag(flags, "lock"), Warnings: []string{},
	}

	zeppDays, hrvInfo, warnings, err := fetchZeppWellness(zeppBase, fields, tzInfo.location, oldest, newest)
	if err != nil {
		return wrapCommandError(err)
	}
	out.Warnings = warnings
	out.HRVSamplesInfo = hrvInfo
	out.ZeppDays = len(zeppDays)

	var existing []icu.Wellness
	query := map[string]string{queryOldest: oldest, queryNewest: newest}
	if err := client.Get("wellness", nil, query, &existing); err != nil {
		return wrapCommandError(fmt.Errorf("read intervals.icu wellness: %w", err))
	}

	plan := icu.PlanWellnessSync(zeppDays, existing, fields, out.Overwrite, out.Lock)
	out.Changes, out.Writes = plan.Changes, plan.Writes
	suggestBase, baseStripped := redactURL(client.BaseURL())
	suggestZepp, zeppStripped := redactURL(zeppBase)
	out.PartialSleep = partialSleepSuggestions(plan.Changes, suggestionContext{
		overwrite: out.Overwrite, lock: out.Lock, allowInsecure: BoolFlag(flags, zeppAllowInsecureHostFlag),
		timezone: out.Timezone, athleteID: icu.StringFlag(flags, "athlete-id", ""), baseURL: suggestBase, zeppBaseURL: suggestZepp,
		unparsableURL: (client.BaseURL() != "" && suggestBase == "") || (zeppBase != "" && suggestZepp == ""),
	})
	if len(out.PartialSleep) > 0 && (baseStripped || zeppStripped) {
		out.Warnings = append(out.Warnings, "credentials, query or fragment removed from --base-url/--zepp-base-url in the suggested command; add them back if needed")
	}
	if len(zeppDays) == 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("no Zepp wellness data between %s and %s", oldest, newest))
	}

	if !out.DryRun && len(plan.Writes) > 0 {
		if err := client.Put("wellness-bulk", nil, nil, plan.Writes, nil); err != nil {
			return wrapCommandError(fmt.Errorf("write wellness-bulk (%d day(s)): %w", len(plan.Writes), err))
		}
		out.Applied = true
	}

	if err := printWellnessSync(stdout, &out, BoolFlag(flags, "json")); err != nil {
		return err
	}
	for _, warning := range out.Warnings {
		_, _ = fmt.Fprintln(stderr, "warning: "+warning)
	}
	if len(warnings) > 0 {
		return fmt.Errorf("%w: %s", errWellnessSyncPartial, strings.Join(warnings, "; "))
	}

	return nil
}

func wellnessSyncRange(flags map[string]string, today time.Time) (string, string, error) { //nolint:gocritic // (oldest, newest, err)
	newest := icu.StringFlag(flags, queryNewest, today.Format(time.DateOnly))
	newestDay, err := time.Parse(time.DateOnly, newest)
	if err != nil {
		return "", "", fmt.Errorf("%w: --newest %q is not YYYY-MM-DD", errInvalidValue, newest)
	}
	oldest := icu.StringFlag(flags, queryOldest, newestDay.AddDate(0, 0, -wellnessSyncDefaultDays+1).Format(time.DateOnly))
	if _, err := time.Parse(time.DateOnly, oldest); err != nil {
		return "", "", fmt.Errorf("%w: --oldest %q is not YYYY-MM-DD", errInvalidValue, oldest)
	}
	if oldest > newest {
		return "", "", fmt.Errorf("%w: empty range: --oldest %s is after --newest %s", errInvalidValue, oldest, newest)
	}

	return oldest, newest, nil
}

func wantsField(fields []string, names ...string) bool {
	for _, field := range fields {
		for _, name := range names {
			if field == name {
				return true
			}
		}
	}

	return false
}

// fetchZeppWellness reads band data (resting HR, sleep) and Zepp nightly HRV (readiness sleepHRV) from
// Zepp. An auth failure aborts; any other source failure is returned as a
// warning so the other source still syncs (the command then exits non-zero).
// HRV is fetched one UTC day wider on each side because HRVLocalDay can move
// a reading across the UTC date line.
func fetchZeppWellness(zeppBase string, fields []string, loc *time.Location, oldest, newest string) ([]icu.ZeppWellnessDay, []icu.ZeppWellnessDay, []string, error) {
	var days []icu.ZeppWellnessDay
	hrvInfo := []icu.ZeppWellnessDay{}
	var warnings []string

	err := runZeppWithClientOptions(zeppSyncOptions(zeppBase), func(ctx context.Context, zc *icu.ZeppClient) error {
		var band []icu.BandDataDay
		var hrv []icu.ZeppHRVEvent

		if wantsField(fields, icu.SyncFieldRestingHR, icu.SyncFieldSleepSecs, icu.SyncFieldSleepScore) {
			got, err := zc.BandData(ctx, oldest, newest)
			if errors.Is(err, icu.ErrZeppUnauthorized) {
				return err
			}
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("zepp band data (restingHR, sleep) failed: %v", err))
			}
			band = got
		}
		if wantsField(fields, icu.SyncFieldHRV) {
			from, _ := time.Parse(time.DateOnly, oldest)
			to, _ := time.Parse(time.DateOnly, newest)
			hrvFrom, hrvTo := from.AddDate(0, 0, -1).Format(time.DateOnly), to.AddDate(0, 0, 1).Format(time.DateOnly)
			sleepHRV, errSleep := zc.SleepHRVDays(ctx, hrvFrom, hrvTo)
			if errors.Is(errSleep, icu.ErrZeppUnauthorized) {
				return errSleep
			}
			// Only readiness sleepHRV is written. Days without it get no HRV:
			// the HRVRMSSD samples mean is an approximation, and fill-only
			// mode would never replace it with the later readiness value.
			if errSleep != nil {
				warnings = append(warnings, fmt.Sprintf("zepp hrv: readiness sleepHRV failed (%v); HRV skipped for every day, rerun later", errSleep))
			}
			hrv = sleepHRV
			// HRVRMSSD samples are fetched for the dry-run information line
			// only; a failure there is not a sync failure.
			samples, errSamples := zc.HRVRMSSDDays(ctx, hrvFrom, hrvTo)
			if errors.Is(errSamples, icu.ErrZeppUnauthorized) {
				return errSamples
			}
			if errSamples == nil {
				hrvInfo = samplesOnlyDays(sleepHRV, samples, loc, oldest, newest)
			}
		}
		days = icu.BuildZeppWellnessDays(band, hrv, loc, oldest, newest)

		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("zepp: %w", err)
	}

	return days, hrvInfo, warnings, nil
}

// samplesOnlyDays returns the days in oldest..newest that have an HRVRMSSD
// samples mean but no readiness sleepHRV.
func samplesOnlyDays(sleepHRV, samples []icu.ZeppHRVEvent, loc *time.Location, oldest, newest string) []icu.ZeppWellnessDay {
	have := map[string]bool{}
	for _, day := range icu.BuildZeppWellnessDays(nil, sleepHRV, loc, oldest, newest) {
		have[day.Date] = day.HRV > 0
	}
	info := []icu.ZeppWellnessDay{}
	for _, day := range icu.BuildZeppWellnessDays(nil, samples, loc, oldest, newest) {
		if day.HRV > 0 && !have[day.Date] {
			info = append(info, icu.ZeppWellnessDay{Date: day.Date, HRV: day.HRV})
		}
	}

	return info
}

// zeppSyncOptions points both the Zepp data host and the events host at
// --zepp-base-url when given (one mock serves both).
func zeppSyncOptions(zeppBase string) []icu.ZeppClientOption {
	options := zeppClientOptionsForRun()
	if zeppBase != "" {
		options = append(options, icu.WithZeppBaseURL(zeppBase), icu.WithZeppEventsURL(zeppBase))
	}

	return options
}

func printWellnessSync(stdout io.Writer, out *wellnessSyncOutput, asJSON bool) error {
	if asJSON {
		return writeJSON(stdout, out)
	}

	mode := "DRY RUN (nothing written; add --apply to write)"
	if out.Applied {
		mode = "APPLIED"
	} else if !out.DryRun {
		mode = "APPLY (nothing to write)"
	}
	fmt.Fprintf(stdout, "wellness sync %s..%s (%s) %s\n", out.Oldest, out.Newest, out.Timezone, mode)

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DATE\tFIELD\tINTERVALS\tZEPP\tACTION\tNOTE")
	for _, change := range out.Changes {
		note := ""
		if change.Note == icu.SyncNotePartialSleep {
			note = "partial sync?"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", change.Date, change.Field,
			icu.FormatSyncValue(change.Intervals), icu.FormatSyncValue(change.Zepp), change.Action, note)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("print table: %w", err)
	}
	for _, suggestion := range out.PartialSleep {
		fmt.Fprintf(stdout, "note: %s sleepSecs in intervals.icu is under half of Zepp's (suspected partial sync); to replace it run:\n  %s\n",
			suggestion.Date, suggestion.Suggest)
	}
	if !out.Applied {
		for _, day := range out.HRVSamplesInfo {
			fmt.Fprintf(stdout, "info: %s HRVRMSSD samples mean %g ms (not written: no readiness sleepHRV that day)\n", day.Date, day.HRV)
		}
	}
	switch {
	case len(out.Writes) == 0:
		fmt.Fprintln(stdout, "nothing to write")
	case out.Applied:
		fmt.Fprintf(stdout, "wrote %d day(s) via wellness-bulk\n", len(out.Writes))
	default:
		fmt.Fprintf(stdout, "would write %d day(s) via wellness-bulk\n", len(out.Writes))
	}

	return nil
}
