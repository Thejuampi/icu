package icu

import (
	"sort"
	"strconv"
	"time"
)

const (
	adaptationDeltaThresholdPct = 3
	adaptationMinimumCurves     = 2
	adaptationSourceMMP         = "mmp_model"
	adaptationSourceSport       = "sport_settings"
	adaptationSourceCurves      = "power_curve_comparison"
	adaptationSourcePhase       = "activity_weekly_load_segments"
	adaptationStatusImproving   = "improving"
	adaptationStatusDeclining   = "declining"
	adaptationStatusStable      = "stable"
	adaptationTrendIncreasing   = "increasing"
	adaptationTrendDecreasing   = "decreasing"
)

type CyclingAdaptationAnalysis struct {
	Scope            AdaptationScope            `json:"scope"`
	PowerAnchors     AdaptationPowerAnchors     `json:"powerAnchors"`
	PowerCurveDeltas []PowerCurveDelta          `json:"powerCurveDeltas,omitempty"`
	SystemStatus     AdaptationSystemStatus     `json:"systemStatus"`
	Lactate          WellnessLactateCalibration `json:"lactate"`
	PhaseSummary     AdaptationPhaseSummary     `json:"phaseSummary"`
	Warnings         []string                   `json:"warnings,omitempty"`
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
			Status:        adaptationDeltaStatus(deltaPercent),
			CurrentCurve:  curveLabel(pair.current),
			BaselineCurve: curveLabel(pair.baseline),
		})
	}

	return deltas
}

func adaptationComparableCurves(curves []DataCurve) adaptationCurvePair {
	if len(curves) < adaptationMinimumCurves {
		return adaptationCurvePair{current: nil, baseline: nil, ok: false}
	}

	sortedCurves := append([]DataCurve(nil), curves...)
	sort.Slice(sortedCurves, func(left, right int) bool {
		return curveDays(&sortedCurves[left]) < curveDays(&sortedCurves[right])
	})

	return adaptationCurvePair{
		current:  &sortedCurves[0],
		baseline: &sortedCurves[len(sortedCurves)-1],
		ok:       true,
	}
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

func adaptationDeltaStatus(deltaPercent float64) string {
	if deltaPercent >= adaptationDeltaThresholdPct {
		return "improved"
	}

	if deltaPercent <= -adaptationDeltaThresholdPct {
		return "declined"
	}

	return adaptationStatusStable
}

func curveLabel(curve *DataCurve) string {
	if curve.Label != "" {
		return curve.Label
	}

	return curve.ID
}

func adaptationSystemStatus(deltas []PowerCurveDelta) AdaptationSystemStatus {
	status := emptyAdaptationSystemStatus()

	for index := range deltas {
		switch deltas[index].Status {
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
