package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	icu "github.com/Thejuampi/icu"
)

// B15: intervals.icu's `decoupling` field is unreliable, so every analysis
// command that consumes decoupling recomputes it from the activity streams
// (icu.ComputeStreamDecoupling) before analysis. Streams are fetched with GET
// only, at most once per activity: results (including "not measurable") are
// cached on disk under <config dir>/cache/decoupling/, keyed by activity id,
// algorithm version and options.
const (
	decouplingSourceFlag   = "decoupling-source"
	decouplingWarmupFlag   = "decoupling-warmup-min"
	decouplingCooldownFlag = "decoupling-cooldown-min"
	decouplingMinPowerFlag = "decoupling-min-power"
	decouplingHRLagFlag    = "decoupling-hr-lag"
	decouplingIQRKFlag     = "decoupling-iqr-k"
	decouplingModeAPI      = "api"
	decouplingWorkers      = 4
	activityStreamsPath    = "streams"
	secondsPerMinuteInt    = 60
)

func decouplingSchemaFlags() []CommandFlag {
	return []CommandFlag{
		{Name: decouplingSourceFlag, ValueName: "streams|api", Description: "Decoupling source: streams (recompute Pa:HR " +
			"from activity streams, default) or api (intervals.icu's decoupling field, no stream fetch).", Default: icu.DecouplingSourceStreams},
		{Name: decouplingWarmupFlag, ValueName: "N", Description: "Warm-up minutes trimmed before stream decoupling.", Default: "20"},
		{Name: decouplingCooldownFlag, ValueName: "N", Description: "Cool-down minutes trimmed before stream decoupling.", Default: "12"},
		{Name: decouplingMinPowerFlag, ValueName: "W", Description: "Minutes averaging below this power are excluded as coasting/stops.", Default: "50"},
		{Name: decouplingHRLagFlag, ValueName: "S", Description: "HR lag in seconds applied when pairing power with HR.", Default: "23"},
		{Name: decouplingIQRKFlag, ValueName: "K", Description: "IQR multiplier for per-minute power/HR ratio outliers (0 disables).", Default: "1.5"},
	}
}

func nonNegativeFloatFlag(flags map[string]string, name string, current float64) (float64, error) {
	raw, ok := flags[name]
	if !ok || raw == "" {
		return current, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return current, fmt.Errorf("--%s must be a non-negative number, got %q", name, raw)
	}

	return value, nil
}

func decouplingOptionsFromFlags(flags map[string]string) (icu.DecouplingOptions, string, error) {
	options := icu.DefaultDecouplingOptions()
	mode := strings.ToLower(icu.StringFlag(flags, decouplingSourceFlag, icu.DecouplingSourceStreams))
	if mode != icu.DecouplingSourceStreams && mode != decouplingModeAPI {
		return options, mode, fmt.Errorf("--%s must be streams or api, got %q", decouplingSourceFlag, mode)
	}
	warmup, err := nonNegativeFloatFlag(flags, decouplingWarmupFlag, float64(options.WarmupSeconds/secondsPerMinuteInt))
	if err != nil {
		return options, mode, err
	}
	cooldown, err := nonNegativeFloatFlag(flags, decouplingCooldownFlag, float64(options.CooldownSeconds/secondsPerMinuteInt))
	if err != nil {
		return options, mode, err
	}
	lag, err := nonNegativeFloatFlag(flags, decouplingHRLagFlag, float64(options.HRLagSeconds))
	if err != nil {
		return options, mode, err
	}
	if options.MinPowerWatts, err = nonNegativeFloatFlag(flags, decouplingMinPowerFlag, options.MinPowerWatts); err != nil {
		return options, mode, err
	}
	if options.IQRK, err = nonNegativeFloatFlag(flags, decouplingIQRKFlag, options.IQRK); err != nil {
		return options, mode, err
	}
	options.WarmupSeconds = int(warmup) * secondsPerMinuteInt
	options.CooldownSeconds = int(cooldown) * secondsPerMinuteInt
	options.HRLagSeconds = int(lag)

	return options, mode, nil
}

type decouplingCacheEntry struct {
	Version     int                   `json:"version"`
	Options     icu.DecouplingOptions `json:"options"`
	Result      *icu.StreamDecoupling `json:"result,omitempty"`
	Unavailable string                `json:"unavailable,omitempty"`
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

func readDecouplingCache(activityID string, options icu.DecouplingOptions) (decouplingCacheEntry, bool) {
	var entry decouplingCacheEntry
	path := decouplingCachePath(activityID)
	if path == "" {
		return entry, false
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &entry) != nil {
		return entry, false
	}
	if entry.Version != icu.DecouplingAlgorithmVersion || entry.Options != options {
		return entry, false
	}

	return entry, true
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
// from cache or a single GET of its streams.
func computeActivityDecoupling(client *icu.Client, activityID string, options icu.DecouplingOptions) decouplingCacheEntry {
	if entry, ok := readDecouplingCache(activityID, options); ok {
		return entry
	}
	entry := decouplingCacheEntry{
		Version: icu.DecouplingAlgorithmVersion,
		Options: options,
	}

	var raw []icu.ActivityStream
	query := map[string]string{"types": icu.DecouplingStreamTypes}
	if err := client.Get("activity", []string{activityID, activityStreamsPath}, query, &raw); err != nil {
		// Network/API errors are not cached.
		entry.Unavailable = "streams fetch failed: " + err.Error()

		return entry
	}
	streams, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		entry.Unavailable = "streams unreadable: " + err.Error()
		writeDecouplingCache(activityID, &entry)

		return entry
	}
	result, err := icu.ComputeStreamDecoupling(streams, options)
	if err != nil {
		reason := err.Error()
		if !errors.Is(err, icu.ErrDecouplingInsufficientStreams) {
			reason = "stream decoupling failed: " + reason
		}
		entry.Unavailable = reason
	} else {
		entry.Result = &result
	}
	writeDecouplingCache(activityID, &entry)

	return entry
}

// decouplingCandidate: only activities whose decoupling anyone consumes
// (cycling, plus runs for pace:HR) and that have HR are fetched, so the
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

// enrichActivitiesDecoupling replaces each candidate activity's API
// decoupling with the stream-derived value. When streams are unavailable,
// the API value is kept and labeled api_fallback with the reason.
func enrichActivitiesDecoupling(client *icu.Client, activities []icu.Activity, flags map[string]string) error {
	options, mode, err := decouplingOptionsFromFlags(flags)
	if err != nil {
		return err
	}
	if mode == decouplingModeAPI {
		for index := range activities {
			activities[index].MarkAPIDecouplingFallback("--decoupling-source api")
		}

		return nil
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for range decouplingWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				activity := &activities[index]
				entry := computeActivityDecoupling(client, activity.ID, options)
				if entry.Result != nil {
					activity.ApplyStreamDecoupling(entry.Result)
				} else {
					activity.MarkAPIDecouplingFallback(entry.Unavailable)
				}
			}
		}()
	}
	for index := range activities {
		if decouplingCandidate(&activities[index]) {
			jobs <- index
		} else {
			activities[index].MarkAPIDecouplingFallback("not a stream-decoupling candidate")
		}
	}
	close(jobs)
	wg.Wait()

	return nil
}

func activityDecouplingCommand() *Command {
	return &Command{
		Name: "",
		Usage: "activity <id> decoupling [--decoupling-warmup-min N] [--decoupling-cooldown-min N] " +
			"[--decoupling-min-power W] [--decoupling-hr-lag S] [--decoupling-iqr-k K]",
		Description: "Aerobic decoupling (Pa:HR, or pace:HR without power) recomputed from the activity streams " +
			"on per-minute averages (warm-up 20 / cool-down 12 min trimmed, <50 W minutes and IQR ratio outliers excluded, " +
			"HR lagged 23 s), next to intervals.icu's decoupling field. Read-only.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			options, _, err := decouplingOptionsFromFlags(flags)
			if err != nil {
				return err
			}
			var activity icu.Activity
			if err := client.Get("activity", []string{args[0]}, nil, &activity); err != nil {
				return wrapCommandError(err)
			}
			entry := computeActivityDecoupling(client, args[0], options)
			out := map[string]any{
				"id":            args[0],
				"name":          activity.Name,
				"apiDecoupling": activity.Decoupling,
			}
			if entry.Result != nil {
				out["decoupling"] = entry.Result.Value
				out["decouplingSource"] = icu.DecouplingSourceStreams
				out["streamDetail"] = entry.Result
			} else {
				out["decoupling"] = activity.Decoupling
				out["decouplingSource"] = icu.DecouplingSourceAPI
				out["decouplingNote"] = entry.Unavailable
			}

			return writeJSON(out)
		},
	}
}
