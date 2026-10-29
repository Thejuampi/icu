package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	icu "github.com/Thejuampi/icu"
)

// B15: intervals.icu's `decoupling` field stays the default consumed value
// (--decoupling-source api). With --decoupling-source streams every analysis
// command that consumes decoupling also recomputes it from the activity
// streams (icu.ComputeStreamDecoupling), reports it next to the API value
// and consumes it when measured. Streams are fetched with GET only, at most
// once per activity edit: results are cached on disk under
// <config dir>/cache/decoupling/, keyed by algorithm version, options,
// FTP/zones, moving time and intervals.icu's `analyzed` timestamp (which changes
// on every edit/reprocess). Not-measured results expire after
// decouplingRetryTTL so they are retried.
const (
	decouplingSourceFlag   = "decoupling-source"
	decouplingWarmupFlag   = "decoupling-warmup-min"
	decouplingCooldownFlag = "decoupling-cooldown-min"
	decouplingHRLagFlag    = "decoupling-hr-lag"
	decouplingIQRKFlag     = "decoupling-iqr-k"
	decouplingModeAPI      = icu.DecouplingSourceAPI
	decouplingModeStreams  = icu.DecouplingSourceStreams
	decouplingWorkers      = 4
	activityStreamsPath    = "streams"
	secondsPerMinuteInt    = 60
	decouplingRetryTTL     = 24 * time.Hour
	dateLayout             = "2006-01-02"
	queryOldest            = "oldest"
	queryNewest            = "newest"
)

func decouplingSchemaFlags() []CommandFlag {
	return []CommandFlag{
		{Name: decouplingSourceFlag, ValueName: "api|streams", Description: "[EXPERIMENTAL streams] Consumed decoupling: api (intervals.icu's " +
			"field, default, no stream fetch) or streams (recompute from activity streams, report both, consume the stream " +
			"value when measured).", Default: decouplingModeAPI},
		{Name: decouplingWarmupFlag, ValueName: "N", Description: "Lengthen the warm-up trim (whole moving minutes); " +
			"never shorter than the coach's trim: max(15 min, HR stabilization), cut at 20 min (low confidence) when HR does not stabilize."},
		{Name: decouplingCooldownFlag, ValueName: "N", Description: "Lengthen the cool-down trim (whole moving minutes); " +
			"never shorter than the coach's fixed 10 min."},
		{Name: decouplingHRLagFlag, ValueName: "S", Description: "Override the HR lag (whole seconds); default " +
			"measured by power/HR cross-correlation (0-120 s), 0 when uncorrelated."},
		{Name: decouplingIQRKFlag, ValueName: "K", Description: "Tukey IQR multiplier for per-minute power/HR ratio " +
			"outliers (0 disables).", Default: "1.5"},
	}
}

func nonNegativeIntFlag(flags map[string]string, name string, scale int) (*int, error) {
	raw, ok := flags[name]
	if !ok || raw == "" {
		return nil, nil //nolint:nilnil // nil = not overridden
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return nil, fmt.Errorf("--%s must be a non-negative integer, got %q", name, raw)
	}
	value *= scale

	return &value, nil
}

func decouplingOptionsFromFlags(flags map[string]string) (icu.DecouplingOptions, string, error) {
	options := icu.DefaultDecouplingOptions()
	mode := strings.ToLower(icu.StringFlag(flags, decouplingSourceFlag, decouplingModeAPI))
	if mode != decouplingModeStreams && mode != decouplingModeAPI {
		return options, mode, fmt.Errorf("--%s must be api or streams, got %q", decouplingSourceFlag, mode)
	}
	var err error
	if options.WarmupSeconds, err = nonNegativeIntFlag(flags, decouplingWarmupFlag, secondsPerMinuteInt); err != nil {
		return options, mode, err
	}
	if options.CooldownSeconds, err = nonNegativeIntFlag(flags, decouplingCooldownFlag, secondsPerMinuteInt); err != nil {
		return options, mode, err
	}
	if options.HRLagSeconds, err = nonNegativeIntFlag(flags, decouplingHRLagFlag, 1); err != nil {
		return options, mode, err
	}
	if raw, ok := flags[decouplingIQRKFlag]; ok && raw != "" {
		k, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil || k < 0 {
			return options, mode, fmt.Errorf("--%s must be a non-negative number, got %q", decouplingIQRKFlag, raw)
		}
		options.IQRK = k
	}

	return options, mode, nil
}

type decouplingCacheEntry struct {
	Key       string               `json:"key"`
	ExpiresAt string               `json:"expiresAt,omitempty"`
	Result    icu.StreamDecoupling `json:"result"`
}

// decouplingCacheKey hashes every input of the stream computation.
func decouplingCacheKey(activity *icu.Activity, options icu.DecouplingOptions, context icu.DecouplingContext) string {
	data, err := json.Marshal(struct {
		Version  int                   `json:"version"`
		Options  icu.DecouplingOptions `json:"options"`
		Context  icu.DecouplingContext `json:"context"`
		Analyzed string                `json:"analyzed"`
	}{icu.DecouplingAlgorithmVersion, options, context, activity.Analyzed})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func decouplingCacheDir() string {
	if dir := os.Getenv("ICU_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "cache", "decoupling")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".icu", "cache", "decoupling")
}

func decouplingCachePath(activityID string) string {
	dir := decouplingCacheDir()
	if dir == "" || activityID == "" || strings.ContainsAny(activityID, `/\.`) {
		return ""
	}

	return filepath.Join(dir, activityID+".json")
}

func readDecouplingCache(activityID, key string) (icu.StreamDecoupling, bool) {
	var entry decouplingCacheEntry
	path := decouplingCachePath(activityID)
	if path == "" {
		return entry.Result, false
	}
	data, err := os.ReadFile(path)
	if key == "" || err != nil || json.Unmarshal(data, &entry) != nil || entry.Key != key {
		return entry.Result, false
	}
	if entry.ExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, entry.ExpiresAt)
		if err != nil || !time.Now().Before(expires) {
			return entry.Result, false
		}
	}

	return entry.Result, true
}

func writeDecouplingCache(activityID string, entry *decouplingCacheEntry) {
	path := decouplingCachePath(activityID)
	if path == "" {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, path)
	}
}

// invalidateDecouplingCache drops a cached result after the activity's
// streams were rewritten (estimate-power-accept).
func invalidateDecouplingCache(activityID string) {
	if path := decouplingCachePath(activityID); path != "" {
		_ = os.Remove(path)
	}
}

// computeActivityDecoupling returns the stream decoupling for one activity,
// from cache or a single GET of its streams. Fetch errors are returned and
// never cached.
func computeActivityDecoupling(
	client *icu.Client,
	activity *icu.Activity,
	options icu.DecouplingOptions,
	context icu.DecouplingContext,
) (icu.StreamDecoupling, error) {
	key := decouplingCacheKey(activity, options, context)
	if result, ok := readDecouplingCache(activity.ID, key); ok {
		return result, nil
	}

	var raw []icu.ActivityStream
	query := map[string]string{"types": icu.DecouplingStreamTypes}
	if err := client.Get("activity", []string{activity.ID, activityStreamsPath}, query, &raw); err != nil {
		return icu.StreamDecoupling{}, fmt.Errorf("streams fetch failed: %w", err)
	}
	streams, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		return icu.StreamDecoupling{}, fmt.Errorf("streams unreadable: %w", err)
	}
	result := icu.ComputeStreamDecoupling(streams, options, context)
	entry := decouplingCacheEntry{Key: key, Result: result}
	if result.Status != icu.DecouplingStatusMeasured {
		entry.ExpiresAt = time.Now().Add(decouplingRetryTTL).UTC().Format(time.RFC3339)
	}
	writeDecouplingCache(activity.ID, &entry)

	return result, nil
}

// decouplingCandidate: only activities whose decoupling anyone consumes
// (cycling, plus runs for GAP:HR) and that have HR are fetched, so the
// stream request volume stays bounded by the analysis window.
func decouplingCandidate(activity *icu.Activity) bool {
	if activity.ID == "" || activity.IgnoreHR {
		return false
	}
	if !icu.IsCyclingActivityType(activity.Type) && !strings.Contains(activity.Type, "Run") {
		return false
	}

	return activity.Decoupling != 0 || activity.AverageHeartRate > 0
}

func activityDay(activity *icu.Activity) string {
	if len(activity.StartDateLocal) < len(dateLayout) {
		return ""
	}

	return activity.StartDateLocal[:len(dateLayout)]
}

// decouplingHistory holds the athlete's Ride sport settings (FTP and power
// zones for the coaching criteria) and the paired workouts' kinds.
type decouplingHistory struct {
	kinds    map[int]string
	settings icu.SportSettings
}

// context is the stream computation input: the activity's FTP (falling
// back to the sport-settings FTP) and the athlete's power zones.
func (history *decouplingHistory) context(activity *icu.Activity) icu.DecouplingContext {
	ftp := activity.FTP
	if ftp <= 0 {
		ftp = history.settings.FTP
	}

	return icu.DecouplingContext{MovingTimeSeconds: activity.MovingTime, FTP: ftp, PowerZones: history.settings.PowerZones}
}

// annotate applies the coaching criteria and adds the paired workout kind
// (informational only: the criteria decide validity).
func (history *decouplingHistory) annotate(activity *icu.Activity, result *icu.StreamDecoupling) {
	result.ApplyCoachCriteria()
	if activity.PairedEventID != nil {
		result.WorkoutKind = history.kinds[*activity.PairedEventID]
	}
}

// loadDecouplingHistory GETs the Ride sport settings and, when the window
// has dates, the events from oldest to newest for the paired workout kinds.
// Errors leave the context empty (the criteria then fail on missing zones).
func loadDecouplingHistory(client *icu.Client, oldest, newest string) *decouplingHistory {
	history := &decouplingHistory{kinds: map[int]string{}}
	if client.Get("sport-settings", []string{"Ride"}, nil, &history.settings) != nil {
		history.settings = icu.SportSettings{}
	}
	if oldest == "" || newest == "" {
		return history
	}
	var events []icu.Event
	if client.Get("events", nil, map[string]string{queryOldest: oldest, queryNewest: newest}, &events) == nil {
		for index := range events {
			if _, kind, ok := icu.PlannedDecouplingWorkoutZones(
				events[index].WorkoutDoc, history.settings.PowerZones, history.settings.FTP); ok {
				history.kinds[events[index].ID] = kind
			}
		}
	}

	return history
}

func minString(left, right string) string {
	if left != "" && left < right {
		return left
	}

	return right
}

// enrichActivitiesDecoupling labels every activity's decoupling source. In
// streams mode it also attaches the stream value to each candidate and
// consumes it when measured; otherwise the API value is kept, labeled api
// with the reason.
func enrichActivitiesDecoupling(client *icu.Client, activities []icu.Activity, flags map[string]string) error {
	options, mode, err := decouplingOptionsFromFlags(flags)
	if err != nil {
		return err
	}
	var candidates []int
	for index := range activities {
		activities[index].MarkAPIDecoupling("")
		if mode == decouplingModeStreams && decouplingCandidate(&activities[index]) {
			candidates = append(candidates, index)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	oldest, newest := "", ""
	for _, index := range candidates {
		if day := activityDay(&activities[index]); day != "" {
			oldest, newest = minString(oldest, day), max(newest, day)
		}
	}
	history := loadDecouplingHistory(client, oldest, newest)

	jobs := make(chan int)
	var wg sync.WaitGroup
	for range decouplingWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				activity := &activities[index]
				result, err := computeActivityDecoupling(client, activity, options, history.context(activity))
				if err != nil {
					activity.MarkAPIDecoupling(err.Error())
					activity.DecouplingStreamsStatus = icu.DecouplingStatusUnavailable
					activity.DecouplingExperimental = true

					continue
				}
				history.annotate(activity, &result)
				activity.AttachStreamDecoupling(&result, true)
			}
		}()
	}
	for _, index := range candidates {
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	return nil
}

func apiDecouplingValue(activity *icu.Activity) *float64 {
	if activity.Decoupling == 0 {
		return nil
	}
	value := activity.Decoupling

	return &value
}

func activityDecouplingCommand() *Command {
	return &Command{
		Name: "",
		Usage: "activity <id> decoupling [--decoupling-source api|streams] [--decoupling-warmup-min N] " +
			"[--decoupling-cooldown-min N] [--decoupling-hr-lag S] [--decoupling-iqr-k K]",
		Description: "[EXPERIMENTAL] Aerobic decoupling from the activity streams (Pa:HR; GAP:HR without power) on " +
			"moving-time minutes, always next to intervals.icu's decoupling field (the default consumed value). Uses " +
			"the coach's criteria on the analyzed range: fixed trim (warm-up max(15 min, HR stabilization), cut at 20 min " +
			"with low confidence when HR does not stabilize; cool-down 10 min; overrides can only lengthen it); VI <= 1.05 valid, 1.05-1.10 low confidence, > 1.10 not valid; >= 60 min; >= 85% of the time in " +
			"power Z1-Z2; no minute above 105% FTP; coasting < 10%. Any failed criterion: not valid; the raw stream value is shown " +
			"(rawValue) but never consumed, and the failed criteria are listed with their values (failedCriteria). Interpretation: < 5% good, 5-8% moderate, > 8% high; < -5% possible fatigue. HR lag by " +
			"cross-correlation, coasting/missing-HR minutes and Tukey ratio outliers excluded. Read-only.",
		Run: func(args []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			options, mode, err := decouplingOptionsFromFlags(flags)
			if err != nil {
				return err
			}
			var activity icu.Activity
			if err := client.Get("activity", []string{args[0]}, nil, &activity); err != nil {
				return wrapCommandError(err)
			}
			activity.ID = args[0]
			day := activityDay(&activity)
			history := loadDecouplingHistory(client, day, day)
			out := map[string]any{
				"id":                args[0],
				"name":              activity.Name,
				"experimental":      true,
				"apiDecoupling":     apiDecouplingValue(&activity),
				"streamsDecoupling": nil,
			}
			result, err := computeActivityDecoupling(client, &activity, options, history.context(&activity))
			if err != nil {
				activity.MarkAPIDecoupling(err.Error())
				activity.DecouplingStreamsStatus = icu.DecouplingStatusUnavailable
				activity.DecouplingExperimental = true
			} else {
				history.annotate(&activity, &result)
				activity.AttachStreamDecoupling(&result, mode == decouplingModeStreams)
				out["streamsDecoupling"] = activity.DecouplingStreams
				out["streamsRawDecoupling"] = result.RawValue
				out["validity"] = result.Validity
				out["failedCriteria"] = failedCriteriaDetail(&result)
				out["summary"] = decouplingSummary(&activity, &result)
				out["streamDetail"] = result
			}
			out["decoupling"] = sessionValue(&activity)
			out["decouplingSource"] = activity.DecouplingSource
			out["streamsStatus"] = activity.DecouplingStreamsStatus
			if activity.DecouplingNote != "" {
				out["decouplingNote"] = activity.DecouplingNote
			}

			return writeJSON(stdout, out)
		},
	}
}

// decouplingSummary is the compact, human-checkable view of one result.
func decouplingSummary(activity *icu.Activity, result *icu.StreamDecoupling) map[string]any {
	return map[string]any{
		"status":               result.Status,
		"reason":               result.Reason,
		"apiDecoupling":        apiDecouplingValue(activity),
		"streamsDecoupling":    activity.DecouplingStreams,
		"trim":                 result.Trim,
		"hrLag":                map[string]any{"seconds": result.HRLagSeconds, "method": result.HRLagMethod, "r": result.HRLagCorrelation},
		"minutesKept":          result.MinutesKept,
		"minutesInRange":       result.MinutesInRange,
		"excluded":             result.Excluded,
		"workoutKind":          result.WorkoutKind,
		"variabilityIndex":     result.VariabilityIndex,
		"streamsRawDecoupling": result.RawValue,
		"validity":             result.Validity,
		"failedCriteria":       failedCriteriaDetail(result),
		"failedCriteriaNames":  result.FailedCriteriaNames,
		"criteria":             result.Criteria,
		"hrStabilized":         result.HRStabilized,
		"interpretation":       result.Interpretation,
		"possibleFatigue":      result.PossibleFatigue,
		"powerCv":              result.PowerCV,
		"stopRule":             result.StopRule,
	}
}

// sessionValue is the consumed decoupling, nil when absent.
func sessionValue(activity *icu.Activity) *float64 {
	if !activity.HasDecoupling() {
		return nil
	}
	value := activity.Decoupling

	return &value
}

// failedCriteriaDetail lists the failed criteria with their values.
func failedCriteriaDetail(result *icu.StreamDecoupling) []icu.DecouplingCriterion {
	var failed []icu.DecouplingCriterion
	for _, criterion := range result.Criteria {
		if !criterion.Pass {
			failed = append(failed, criterion)
		}
	}

	return failed
}
