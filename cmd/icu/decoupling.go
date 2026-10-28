package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
// <config dir>/cache/decoupling/, keyed by algorithm version, options, plan
// trim, moving time and intervals.icu's `analyzed` timestamp (which changes
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
	// decouplingVIHistoryDays: the VI-cutoff history window of the
	// single-activity command (the analysis commands use their own window).
	decouplingVIHistoryDays = 84
	dateLayout              = "2006-01-02"
	queryOldest             = "oldest"
	queryNewest             = "newest"
)

func decouplingSchemaFlags() []CommandFlag {
	return []CommandFlag{
		{Name: decouplingSourceFlag, ValueName: "api|streams", Description: "Consumed decoupling: api (intervals.icu's " +
			"field, default, no stream fetch) or streams (recompute from activity streams, report both, consume the stream " +
			"value when measured).", Default: decouplingModeAPI},
		{Name: decouplingWarmupFlag, ValueName: "N", Description: "Override the warm-up trim (whole moving minutes); " +
			"default derived from the paired workout's steps, then steady-state detection."},
		{Name: decouplingCooldownFlag, ValueName: "N", Description: "Override the cool-down trim (whole moving minutes); " +
			"default derived like the warm-up."},
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
	plan *icu.DecouplingPlanTrim,
) (icu.StreamDecoupling, error) {
	context := icu.DecouplingContext{MovingTimeSeconds: activity.MovingTime, Plan: plan}
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

// decouplingPlanTrims reads the paired workouts' warm-up/cool-down with a
// single events GET over the candidates' date span. Errors fall back to
// steady-state trimming (empty map).
func decouplingPlanTrims(client *icu.Client, activities []icu.Activity, candidates []int) map[int]*icu.DecouplingPlanTrim {
	trims := map[int]*icu.DecouplingPlanTrim{}
	oldest, newest := "", ""
	for _, index := range candidates {
		activity := &activities[index]
		day := activityDay(activity)
		if activity.PairedEventID == nil || day == "" {
			continue
		}
		if oldest == "" || day < oldest {
			oldest = day
		}
		if day > newest {
			newest = day
		}
	}
	if oldest == "" {
		return trims
	}
	var events []icu.Event
	if err := client.Get("events", nil, map[string]string{queryOldest: oldest, queryNewest: newest}, &events); err != nil {
		return trims
	}
	for index := range events {
		addEventPlanTrim(trims, &events[index])
	}

	return trims
}

func addEventPlanTrim(trims map[int]*icu.DecouplingPlanTrim, event *icu.Event) {
	doc, err := icu.DecodeWorkoutDoc(event.WorkoutDoc)
	if err != nil {
		return
	}
	if trim, ok := icu.PlannedDecouplingTrim(doc); ok {
		trims[event.ID] = trim
	}
}

// decouplingVIValues are the cycling VIs the cutoff is derived from.
func decouplingVIValues(activities []icu.Activity) []float64 {
	var values []float64
	for index := range activities {
		if icu.IsCyclingActivityType(activities[index].Type) && activities[index].VariabilityIndex > 0 {
			values = append(values, activities[index].VariabilityIndex)
		}
	}

	return values
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
	trims := decouplingPlanTrims(client, activities, candidates)
	cutoff := icu.DecouplingVICutoff(decouplingVIValues(activities))

	jobs := make(chan int)
	var wg sync.WaitGroup
	for range decouplingWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				activity := &activities[index]
				var plan *icu.DecouplingPlanTrim
				if activity.PairedEventID != nil {
					plan = trims[*activity.PairedEventID]
				}
				result, err := computeActivityDecoupling(client, activity, options, plan)
				if err != nil {
					activity.MarkAPIDecoupling(err.Error())
					activity.DecouplingStreamsStatus = icu.DecouplingStatusUnavailable

					continue
				}
				result.ApplyVIGate(activity.VariabilityIndex, cutoff)
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

// singleActivityContext fetches the paired workout's trim and the VI cutoff
// (last decouplingVIHistoryDays of rides) for one activity. Both are
// optional: errors leave them nil.
func singleActivityContext(client *icu.Client, activity *icu.Activity) (*icu.DecouplingPlanTrim, *float64) {
	var plan *icu.DecouplingPlanTrim
	if activity.PairedEventID != nil {
		var event icu.Event
		if client.Get("events", []string{strconv.Itoa(*activity.PairedEventID)}, nil, &event) == nil {
			trims := map[int]*icu.DecouplingPlanTrim{}
			addEventPlanTrim(trims, &event)
			plan = trims[event.ID]
		}
	}
	day, err := time.Parse(dateLayout, activityDay(activity))
	if err != nil {
		return plan, nil
	}
	query := map[string]string{
		queryOldest: day.AddDate(0, 0, -decouplingVIHistoryDays).Format(dateLayout),
		queryNewest: day.Format(dateLayout),
		"fields":    "id,type,icu_variability_index",
	}
	var history []icu.Activity
	if client.Get("activities", nil, query, &history) != nil {
		return plan, nil
	}

	return plan, icu.DecouplingVICutoff(decouplingVIValues(history))
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
		Description: "Aerobic decoupling from the activity streams (Pa:HR; GAP:HR without power) on moving-time minutes, " +
			"next to intervals.icu's decoupling field. Trim from the paired workout's steps then steady-state detection, " +
			"HR lag by cross-correlation, coasting and missing-HR minutes and Tukey ratio outliers excluded, VI gate from " +
			"the athlete's history. The API value stays the consumed one unless --decoupling-source streams. Read-only.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
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
			plan, cutoff := singleActivityContext(client, &activity)
			out := map[string]any{
				"id":                args[0],
				"name":              activity.Name,
				"apiDecoupling":     apiDecouplingValue(&activity),
				"streamsDecoupling": nil,
			}
			result, err := computeActivityDecoupling(client, &activity, options, plan)
			if err != nil {
				activity.MarkAPIDecoupling(err.Error())
				activity.DecouplingStreamsStatus = icu.DecouplingStatusUnavailable
			} else {
				result.ApplyVIGate(activity.VariabilityIndex, cutoff)
				activity.AttachStreamDecoupling(&result, mode == decouplingModeStreams)
				out["streamsDecoupling"] = result.Value
				out["streamDetail"] = result
			}
			out["decoupling"] = sessionValue(&activity)
			out["decouplingSource"] = activity.DecouplingSource
			out["streamsStatus"] = activity.DecouplingStreamsStatus
			if activity.DecouplingNote != "" {
				out["decouplingNote"] = activity.DecouplingNote
			}

			return writeJSON(out)
		},
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
