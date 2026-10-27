package icu

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

const (
	adaptationMinimumCurves = 2
	// adaptationMinDeltaSamples is the fewest curve durations needed for a
	// median/MAD noise estimate (method parameter).
	adaptationMinDeltaSamples = 3
	adaptationSourceMMP       = "mmp_model"
	adaptationSourceSport     = "sport_settings"
	adaptationSourceCurves    = "power_curve_comparison"
	adaptationSourcePhase     = "activity_weekly_load_segments"
	adaptationStatusImproving = "improving"
	adaptationStatusDeclining = "declining"
	adaptationStatusStable    = "stable"
	adaptationTrendIncreasing = "increasing"
	adaptationTrendDecreasing = "decreasing"
)

type CyclingAdaptationAnalysis struct {
	Scope            AdaptationScope        `json:"scope"`
	PowerAnchors     AdaptationPowerAnchors `json:"powerAnchors"`
	PowerCurveDeltas []PowerCurveDelta      `json:"powerCurveDeltas,omitempty"`
	// PowerCurveChangeThreshold is the ±% beyond which a duration counts as
	// improved/declined: WatchZ × the robust spread (MAD × 1.4826) of this
	// athlete's own deltas across durations, or the profile's
	// adaptationChangePct.
	PowerCurveChangeThreshold *DerivedThreshold          `json:"powerCurveChangeThreshold,omitempty"`
	SystemStatus              AdaptationSystemStatus     `json:"systemStatus"`
	Lactate                   WellnessLactateCalibration `json:"lactate"`
	PhaseSummary              AdaptationPhaseSummary     `json:"phaseSummary"`
	Warnings                  []string                   `json:"warnings,omitempty"`
}

type AdaptationScope struct {
	StartDate      string `json:"startDate,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	Curves         int    `json:"curves"`
	Activities     int    `json:"activities"`
	Timezone       string `json:"timezone,omitempty"`
	TimezoneSource string `json:"timezoneSource,omitempty"`
}

type AdaptationPowerAnchors struct {
	FTP           int    `json:"ftp,omitempty"`
	CriticalPower int    `json:"criticalPower,omitempty"`
	WPrime        int    `json:"wPrime,omitempty"`
	PMax          int    `json:"pMax,omitempty"`
	Source        string `json:"source,omitempty"`
}

type PowerCurveDelta struct {
	DurationSecs  int     `json:"durationSecs"`
	CurrentWatts  int     `json:"currentWatts"`
	BaselineWatts int     `json:"baselineWatts"`
	DeltaWatts    int     `json:"deltaWatts"`
	DeltaPercent  float64 `json:"deltaPercent"`
	Status        string  `json:"status,omitempty"`
	CurrentCurve  string  `json:"currentCurve,omitempty"`
	BaselineCurve string  `json:"baselineCurve,omitempty"`
}

type AdaptationSystemStatus struct {
	Status        string `json:"status,omitempty"`
	Improved      int    `json:"improved"`
	Stable        int    `json:"stable"`
	Declined      int    `json:"declined"`
	PrimarySignal string `json:"primarySignal,omitempty"`
	Source        string `json:"source,omitempty"`
}

type AdaptationPhaseSummary struct {
	Weeks        int    `json:"weeks"`
	RecentLoad   int    `json:"recentLoad"`
	PreviousLoad int    `json:"previousLoad"`
	Trend        string `json:"trend,omitempty"`
	Phase        string `json:"phase,omitempty"`
	Source       string `json:"source,omitempty"`
}

type adaptationHistoryWeek struct {
	load int
}

type adaptationCurvePair struct {
	current  *DataCurve
	baseline *DataCurve
	ok       bool
}

func AnalyzeCyclingAdaptation(
	curves []DataCurve,
	model PowerModel,
	settings *SportSettings,
	activities []Activity,
	wellness *WellnessAnalysis,
	options AnalysisOptions,
) CyclingAdaptationAnalysis {
	// Stage 6 R14: route through the shared choke point (activity_validity.go)
	// before Scope.Activities is counted or any load is summed below. A D-24
	// STRAVA stub is not a session; every other aggregator in this package
	// already filters here (rebalance.go, plan.go, microcycle_analysis.go) —
	// this function was the one that didn't, inflating both the reported
	// count and (silently) every load sum derived from activities.
	activities = ExcludeStubActivities(activities)
	analysis := CyclingAdaptationAnalysis{
		Scope: AdaptationScope{
			StartDate:      options.StartDate,
			EndDate:        options.EndDate,
			Curves:         len(curves),
			Activities:     len(activities),
			Timezone:       options.Timezone,
			TimezoneSource: options.TimezoneSource,
		},
		PowerAnchors:     adaptationPowerAnchors(model, settings),
		PowerCurveDeltas: adaptationPowerCurveDeltas(curves),
		SystemStatus:     emptyAdaptationSystemStatus(),
		Lactate:          adaptationLactate(wellness),
		PhaseSummary:     adaptationPhaseSummary(activities, options),
		Warnings:         nil,
	}
	analysis.PowerCurveChangeThreshold = adaptationChangeThreshold(analysis.PowerCurveDeltas, options.Thresholds)
	applyAdaptationDeltaStatus(analysis.PowerCurveDeltas, analysis.PowerCurveChangeThreshold)
	analysis.SystemStatus = adaptationSystemStatus(analysis.PowerCurveDeltas)

	if len(analysis.PowerCurveDeltas) == 0 {
		analysis.Warnings = append(analysis.Warnings, "power curve comparison unavailable")
	}

	return analysis
}

func adaptationPowerAnchors(model PowerModel, settings *SportSettings) AdaptationPowerAnchors {
	if model.CriticalPower != 0 || model.WPrime != 0 || model.PMax != 0 || model.FTP != 0 {
		return AdaptationPowerAnchors{
			FTP:           model.FTP,
			CriticalPower: model.CriticalPower,
			WPrime:        model.WPrime,
			PMax:          model.PMax,
			Source:        adaptationSourceMMP,
		}
	}

	if settings == nil || settings.FTP == 0 && settings.WPrime == 0 && settings.PMax == 0 {
		return emptyAdaptationPowerAnchors()
	}

	return AdaptationPowerAnchors{
		FTP:           settings.FTP,
		CriticalPower: 0,
		WPrime:        settings.WPrime,
		PMax:          settings.PMax,
		Source:        adaptationSourceSport,
	}
}

func emptyAdaptationPowerAnchors() AdaptationPowerAnchors {
	return AdaptationPowerAnchors{FTP: 0, CriticalPower: 0, WPrime: 0, PMax: 0, Source: ""}
}

func adaptationLactate(wellness *WellnessAnalysis) WellnessLactateCalibration {
	if wellness == nil {
		return WellnessLactateCalibration{
			Mean:            0,
			Latest:          0,
			Trend7Day:       0,
			Samples:         0,
			CoveragePercent: 0,
			State:           "",
			Source:          "",
		}
	}

	return wellness.Lactate
}

func adaptationPowerCurveDeltas(curves []DataCurve) []PowerCurveDelta {
	pair := adaptationComparableCurves(curves)
	if !pair.ok {
		return nil
	}

	baselineValues := curveValuesByDuration(pair.baseline)
	deltas := make([]PowerCurveDelta, 0, len(pair.current.Secs))

	for index, duration := range pair.current.Secs {
		if index >= len(pair.current.Values) {
			continue
		}

		baselineWatts, ok := baselineValues[duration]
		if !ok || baselineWatts == 0 {
			continue
		}

		currentWatts := pair.current.Values[index]
		deltaWatts := currentWatts - baselineWatts
		deltaPercent := round2(float64(deltaWatts) * percentScale / float64(baselineWatts))
		deltas = append(deltas, PowerCurveDelta{
			DurationSecs:  duration,
			CurrentWatts:  currentWatts,
			BaselineWatts: baselineWatts,
			DeltaWatts:    deltaWatts,
			DeltaPercent:  deltaPercent,
			CurrentCurve:  curveLabel(pair.current),
			BaselineCurve: curveLabel(pair.baseline),
		})
	}

	return deltas
}

// adaptationComparableCurves picks the current curve (latest end date, then
// fewest days) and the baseline curve (earliest end date, then most days).
// With the default non-overlapping windows the baseline is the period before
// the analysis scope; with nested windows such as "42d,365d" both end today
// and the shorter curve is current, as before.
func adaptationComparableCurves(curves []DataCurve) adaptationCurvePair {
	if len(curves) < adaptationMinimumCurves {
		return adaptationCurvePair{current: nil, baseline: nil, ok: false}
	}

	sortedCurves := append([]DataCurve(nil), curves...)
	sort.SliceStable(sortedCurves, func(left, right int) bool {
		leftEnd, rightEnd := curveEndDay(&sortedCurves[left]), curveEndDay(&sortedCurves[right])
		if leftEnd != rightEnd {
			return leftEnd > rightEnd
		}

		return curveDays(&sortedCurves[left]) < curveDays(&sortedCurves[right])
	})

	return adaptationCurvePair{
		current:  &sortedCurves[0],
		baseline: &sortedCurves[len(sortedCurves)-1],
		ok:       true,
	}
}

func curveEndDay(curve *DataCurve) string {
	if len(curve.EndDate) >= len(analysisDateLayout) {
		return curve.EndDate[:len(analysisDateLayout)]
	}

	return curve.EndDate
}

// AdaptationCurveWindows returns the intervals.icu custom-range curve IDs
// ("r.START.END") for the analysis scope and for the equally long window
// immediately before it, so the baseline never contains the current period.
func AdaptationCurveWindows(oldest, newest string) (string, error) {
	start, err := time.Parse(analysisDateLayout, oldest)
	if err != nil {
		return "", fmt.Errorf("invalid oldest date %q: %w", oldest, err)
	}
	end, err := time.Parse(analysisDateLayout, newest)
	if err != nil {
		return "", fmt.Errorf("invalid newest date %q: %w", newest, err)
	}
	if end.Before(start) {
		return "", fmt.Errorf("newest %s is before oldest %s", newest, oldest)
	}

	days := int(end.Sub(start).Hours()/hoursPerDay) + 1
	baselineEnd := start.AddDate(0, 0, -1)
	baselineStart := baselineEnd.AddDate(0, 0, -(days - 1))

	return "r." + oldest + "." + newest + ",r." + baselineStart.Format(analysisDateLayout) + "." +
		baselineEnd.Format(analysisDateLayout), nil
}

func curveDays(curve *DataCurve) int {
	if curve.Days > 0 {
		return curve.Days
	}

	start, startErr := time.Parse(analysisDateLayout, curve.StartDate)
	end, endErr := time.Parse(analysisDateLayout, curve.EndDate)

	if startErr != nil || endErr != nil || end.Before(start) {
		return 0
	}

	return int(end.Sub(start).Hours()/hoursPerDay) + 1
}

func curveValuesByDuration(curve *DataCurve) map[int]int {
	values := map[int]int{}

	for index, duration := range curve.Secs {
		if index >= len(curve.Values) {
			continue
		}

		values[duration] = curve.Values[index]
	}

	return values
}

// adaptationChangeThreshold derives the change bound from the spread of
// the athlete's own curve deltas (noise across durations), or the profile.
func adaptationChangeThreshold(deltas []PowerCurveDelta, overrides *TrainingThresholds) *DerivedThreshold {
	if pinned := overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.AdaptationChangePct }); pinned != nil {
		return &DerivedThreshold{Watch: *pinned, Red: *pinned, Source: thresholdSourceProfile}
	}
	if len(deltas) < adaptationMinDeltaSamples {
		return nil
	}
	values := make([]float64, 0, len(deltas))
	for index := range deltas {
		values = append(values, deltas[index].DeltaPercent)
	}
	_, scale, source := robustScale(values)
	if scale == 0 {
		return nil
	}
	bound := round2(thresholdMethod(overrides).watchZ * scale)

	return &DerivedThreshold{Watch: bound, Red: bound, Scale: round2(scale), Samples: len(values), Source: source}
}

// applyAdaptationDeltaStatus labels each delta against threshold; without a
// threshold the status is "insufficient_history".
func applyAdaptationDeltaStatus(deltas []PowerCurveDelta, threshold *DerivedThreshold) {
	for index := range deltas {
		delta := &deltas[index]
		switch {
		case threshold == nil:
			delta.Status = "insufficient_history"
		case delta.DeltaPercent >= threshold.Watch:
			delta.Status = "improved"
		case delta.DeltaPercent <= -threshold.Watch:
			delta.Status = "declined"
		default:
			delta.Status = adaptationStatusStable
		}
	}
}

func curveLabel(curve *DataCurve) string {
	if curve.Label != "" {
		return curve.Label
	}

	return curve.ID
}

// isAdaptationAnchorDuration reports whether secs is on the duration grid
// systemStatus counts: one signal per standard power-curve anchor rather than
// every per-second point, which are strongly correlated neighbours.
func isAdaptationAnchorDuration(secs int) bool {
	switch secs {
	case 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 3600: //nolint:mnd // standard power-curve anchor grid in seconds.
		return true
	default:
		return false
	}
}

func adaptationSystemStatus(deltas []PowerCurveDelta) AdaptationSystemStatus {
	status := emptyAdaptationSystemStatus()

	anchors := make([]PowerCurveDelta, 0, len(deltas))
	for index := range deltas {
		if isAdaptationAnchorDuration(deltas[index].DurationSecs) {
			anchors = append(anchors, deltas[index])
		}
	}
	counted := deltas
	if len(anchors) > 0 {
		counted = anchors
	}

	for index := range counted {
		switch counted[index].Status {
		case "improved":
			status.Improved++
		case "declined":
			status.Declined++
		default:
			status.Stable++
		}
	}

	if len(deltas) == 0 {
		return status
	}

	status.PrimarySignal = strongestAdaptationSignal(deltas)

	if status.Improved > status.Declined {
		status.Status = adaptationStatusImproving

		return status
	}

	if status.Declined > status.Improved {
		status.Status = adaptationStatusDeclining

		return status
	}

	status.Status = adaptationStatusStable

	return status
}

func emptyAdaptationSystemStatus() AdaptationSystemStatus {
	return AdaptationSystemStatus{
		Status:        unknownState,
		Improved:      0,
		Stable:        0,
		Declined:      0,
		PrimarySignal: "",
		Source:        adaptationSourceCurves,
	}
}

func strongestAdaptationSignal(deltas []PowerCurveDelta) string {
	if len(deltas) == 0 {
		return ""
	}

	strongest := deltas[0]

	for index := range deltas[1:] {
		candidate := deltas[index+1]
		if absFloat(candidate.DeltaPercent) > absFloat(strongest.DeltaPercent) {
			strongest = candidate
		}
	}

	return durationLabel(strongest.DurationSecs) + "_" + strongest.Status
}

func durationLabel(seconds int) string {
	if seconds%secondsPerHour == 0 && seconds >= secondsPerHour {
		return stringInt(seconds/secondsPerHour) + "h"
	}

	if seconds%secondsPerMinute == 0 {
		return stringInt(seconds/secondsPerMinute) + "m"
	}

	return stringInt(seconds) + "s"
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}

func adaptationPhaseSummary(activities []Activity, options AnalysisOptions) AdaptationPhaseSummary {
	weeklyLoads := adaptationWeeklyLoads(activities, options)
	keys := make([]string, 0, len(weeklyLoads))

	for key := range weeklyLoads {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	if len(keys) == 0 {
		return AdaptationPhaseSummary{
			Weeks:        0,
			RecentLoad:   0,
			PreviousLoad: 0,
			Trend:        unknownState,
			Phase:        unknownState,
			Source:       adaptationSourcePhase,
		}
	}

	recentLoad := weeklyLoads[keys[len(keys)-1]].load
	previousLoad := 0

	if len(keys) >= planMinimumTrendWeeks {
		previousLoad = weeklyLoads[keys[len(keys)-2]].load
	}

	trend := adaptationLoadTrend(recentLoad, previousLoad)

	return AdaptationPhaseSummary{
		Weeks:        len(keys),
		RecentLoad:   recentLoad,
		PreviousLoad: previousLoad,
		Trend:        trend,
		Phase:        adaptationPhaseFromTrend(trend),
		Source:       adaptationSourcePhase,
	}
}

func adaptationWeeklyLoads(activities []Activity, options AnalysisOptions) map[string]adaptationHistoryWeek {
	weeklyLoads := map[string]adaptationHistoryWeek{}

	for index := range activities {
		activity := &activities[index]
		if !IsCyclingActivityType(activity.Type) {
			continue
		}

		date := activityDate(activity)
		if date == "" || !dateWithinRange(date, options.StartDate, options.EndDate) {
			continue
		}

		parsed, err := time.Parse(analysisDateLayout, date)
		if err != nil {
			continue
		}

		key := isoWeekKey(parsed)
		week := weeklyLoads[key]
		week.load += activity.TrainingLoad
		weeklyLoads[key] = week
	}

	return weeklyLoads
}

func adaptationLoadTrend(recentLoad, previousLoad int) string {
	if previousLoad == 0 {
		return unknownState
	}

	if float64(recentLoad) >= float64(previousLoad)*planBuildProgressionRatio {
		return adaptationTrendIncreasing
	}

	if float64(recentLoad) <= float64(previousLoad)*planDeloadRatio {
		return adaptationTrendDecreasing
	}

	return adaptationStatusStable
}

func adaptationPhaseFromTrend(trend string) string {
	switch trend {
	case adaptationTrendIncreasing:
		return planPhaseBuild
	case adaptationTrendDecreasing:
		return planPhaseRecovery
	case adaptationStatusStable:
		return "maintenance"
	default:
		return unknownState
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}

	return value
}
