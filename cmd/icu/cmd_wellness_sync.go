package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
		Description: "Copy resting HR, HRV (rMSSD) and sleep from Zepp into intervals.icu wellness. " +
			"Dry run by default: prints the diff and writes nothing; --apply sends one PUT wellness-bulk. " +
			"Fills only empty fields unless --overwrite. HRV goes on the athlete's local day (18:00+ readings count for the next day). " +
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
		DryRun: !BoolFlag(flags, "apply"), Timezone: tzInfo.timezone, Oldest: oldest, Newest: newest, Fields: fields,
		Overwrite: BoolFlag(flags, "overwrite"), Lock: BoolFlag(flags, "lock"), Warnings: []string{},
	}

	zeppDays, warnings, err := fetchZeppWellness(zeppBase, fields, tzInfo.location, oldest, newest)
	if err != nil {
		return wrapCommandError(err)
	}
	out.Warnings = warnings
	out.ZeppDays = len(zeppDays)

	var existing []icu.Wellness
	query := map[string]string{queryOldest: oldest, queryNewest: newest}
	if err := client.Get("wellness", nil, query, &existing); err != nil {
		return wrapCommandError(fmt.Errorf("read intervals.icu wellness: %w", err))
	}

	plan := icu.PlanWellnessSync(zeppDays, existing, fields, out.Overwrite, out.Lock)
	out.Changes, out.Writes = plan.Changes, plan.Writes
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

// fetchZeppWellness reads band data (resting HR, sleep) and HRV rMSSD from
// Zepp. An auth failure aborts; any other source failure is returned as a
// warning so the other source still syncs (the command then exits non-zero).
// HRV is fetched one UTC day wider on each side because HRVLocalDay can move
// a reading across the UTC date line.
func fetchZeppWellness(zeppBase string, fields []string, loc *time.Location, oldest, newest string) ([]icu.ZeppWellnessDay, []string, error) {
	var days []icu.ZeppWellnessDay
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
			got, err := zc.HRVRMSSDDays(ctx, from.AddDate(0, 0, -1).Format(time.DateOnly), to.AddDate(0, 0, 1).Format(time.DateOnly))
			if errors.Is(err, icu.ErrZeppUnauthorized) {
				return err
			}
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("zepp hrv failed: %v", err))
			}
			hrv = got
		}
		days = icu.BuildZeppWellnessDays(band, hrv, loc, oldest, newest)

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("zepp: %w", err)
	}

	return days, warnings, nil
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
	fmt.Fprintln(tw, "DATE\tFIELD\tINTERVALS\tZEPP\tACTION")
	for _, change := range out.Changes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", change.Date, change.Field, icu.FormatSyncValue(change.Intervals), icu.FormatSyncValue(change.Zepp), change.Action)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("print table: %w", err)
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
