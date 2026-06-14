package icu

import (
	"math"
	"strings"
)

const (
	hrRecovery1MinWindow = 60
	hrRecovery2MinWindow = 120
	hrSlopeToMin         = 60
	zoneAlignmentHotGap  = 10
	zoneAlignmentColdGap = -5
	warmupStableWindow   = 10
	warmupStableThresh   = 3.0
)

type ActivityMicroAnalysis struct {
	ActivityID     string                `json:"activityId"`
	ActivityName   string                `json:"activityName"`
	Warmup         *WarmupAnalysis       `json:"warmup,omitempty"`
	Cooldown       *CooldownAnalysis     `json:"cooldown,omitempty"`
	Intervals      *IntervalAnalysis     `json:"intervals,omitempty"`
	ZoneAlignment  ZoneAlignmentAnalysis `json:"zoneAlignment,omitempty"`
	SessionSummary CyclingSession        `json:"sessionSummary"`
	Warnings       []string              `json:"warnings,omitempty"`
}

type WarmupAnalysis struct {
	DurationSeconds  int     `json:"durationSeconds"`
	AvgPower         float64 `json:"avgPower"`
	AvgHR            float64 `json:"avgHr"`
	HRStabilization  int     `json:"hrStabilizationSeconds"`
	HREfficiency     float64 `json:"hrEfficiency"`
	HRSlope          float64 `json:"hrSlope"`
	HRSlopePerMinute float64 `json:"hrSlopePerMinute"`
}

type CooldownAnalysis struct {
	DurationSeconds int     `json:"durationSeconds"`
	AvgPower        float64 `json:"avgPower"`
	AvgHR           float64 `json:"avgHr"`
	HRRecovery1Min  float64 `json:"hrRecovery1Min"`
	HRRecovery2Min  float64 `json:"hrRecovery2Min"`
	HRDecayRate     float64 `json:"hrDecayRate"`
}

type IntervalAnalysis struct {
	RepCount         int                   `json:"repCount"`
	WorkDuration     float64               `json:"workDurationSecs"`
	RecoveryDuration float64               `json:"recoveryDurationSecs"`
	Reps             []IntervalRepAnalysis `json:"reps"`
	Repeatability    RepeatabilityAnalysis `json:"repeatability"`
}

type IntervalRepAnalysis struct {
	Index           int     `json:"index"`
	Type            string  `json:"type"`
	AvgPower        float64 `json:"avgPower"`
	AvgHR           float64 `json:"avgHr"`
	MaxHR           float64 `json:"maxHr"`
	EF              float64 `json:"ef"`
	DurationSeconds int     `json:"durationSeconds"`
}

type RepeatabilityAnalysis struct {
	WorkPowerMean   float64 `json:"workPowerMean"`
	WorkPowerStdDev float64 `json:"workPowerStdDev"`
	WorkPowerCV     float64 `json:"workPowerCV"`
	WorkHRMean      float64 `json:"workHrMean"`
	WorkHRStdDev    float64 `json:"workHrStdDev"`
	PowerDrift      float64 `json:"powerDrift"`
	HRDrift         float64 `json:"hrDrift"`
}

type ZoneAlignmentAnalysis struct {
	PowerZ1Z2Pct   float64 `json:"powerZ1Z2Pct"`
	HRZ1Z2Pct      float64 `json:"hrZ1Z2Pct"`
	PowerZ3PlusPct float64 `json:"powerZ3PlusPct"`
	HRZ3PlusPct    float64 `json:"hrZ3PlusPct"`
	AlignmentGap   float64 `json:"alignmentGap"`
	Diagnosis      string  `json:"diagnosis"`
}

// AnalyzeWarmup computes warmup metrics from power and heart-rate streams.
func AnalyzeWarmup(watts, hr []float64, section ActivitySection, _ int) WarmupAnalysis {
	var result WarmupAnalysis

	result.DurationSeconds = section.EndIndex - section.StartIndex
	if result.DurationSeconds <= 0 || section.StartIndex < 0 || section.EndIndex > len(watts) || section.EndIndex > len(hr) {
		return result
	}

	result.AvgPower = AveragePower(watts, section.StartIndex, section.EndIndex)
	result.AvgHR = AverageHeartRate(hr, section.StartIndex, section.EndIndex)
	result.HRSlope = Slope(hr[section.StartIndex:section.EndIndex])
	result.HRSlopePerMinute = result.HRSlope * hrSlopeToMin
	result.HREfficiency = PowerHRRatio(watts, hr, section.StartIndex, section.EndIndex)

	sectionHR := hr[section.StartIndex:section.EndIndex]
	for i := warmupStableWindow; i < len(sectionHR)-warmupStableWindow; i++ {
		if Variance(sectionHR[i:i+warmupStableWindow]) < warmupStableThresh {
			result.HRStabilization = i
			break
		}
	}

	return result
}

func AnalyzeCooldown(watts, hr []float64, section ActivitySection, _ int) CooldownAnalysis {
	var result CooldownAnalysis

	result.DurationSeconds = section.EndIndex - section.StartIndex
	if result.DurationSeconds <= 0 || section.StartIndex < 0 || section.EndIndex > len(watts) || section.EndIndex > len(hr) {
		return result
	}

	result.AvgPower = AveragePower(watts, section.StartIndex, section.EndIndex)
	result.AvgHR = AverageHeartRate(hr, section.StartIndex, section.EndIndex)

	sectionHR := hr[section.StartIndex:section.EndIndex]
	result.HRDecayRate = -Slope(sectionHR) * hrSlopeToMin

	peakIdx := 0
	for i := range sectionHR {
		if sectionHR[i] > sectionHR[peakIdx] {
			peakIdx = i
		}
	}

	if len(sectionHR) >= hrRecovery1MinWindow {
		result.HRRecovery1Min = HeartRateRecovery(sectionHR, peakIdx, hrRecovery1MinWindow)
		if peakIdx+hrRecovery2MinWindow < len(sectionHR) {
			result.HRRecovery2Min = HeartRateRecovery(sectionHR, peakIdx, hrRecovery2MinWindow)
		}
	} else {
		recoveryWindow := len(sectionHR) - peakIdx - 1
		if recoveryWindow > 0 {
			result.HRRecovery1Min = HeartRateRecovery(sectionHR, peakIdx, recoveryWindow)
		}
	}

	return result
}

func AnalyzeIntervals(sections []ActivitySection, watts, hr []float64, _ int) IntervalAnalysis {
	var result IntervalAnalysis
	if len(sections) == 0 || len(watts) == 0 || len(hr) == 0 {
		return result
	}

	var workReps []IntervalRepAnalysis
	var totalWorkDuration int
	var totalRecoveryDuration int

	repIdx := 0
	for i := range sections {
		sec := sections[i]
		if sec.EndIndex <= sec.StartIndex || sec.StartIndex < 0 || sec.EndIndex > len(watts) || sec.EndIndex > len(hr) {
			continue
		}

		secType, _ := sec.Metadata["type"].(string)
		if secType == "" {
			secType = sec.Name
		}

		var rep IntervalRepAnalysis
		rep.Index = repIdx
		rep.Type = secType
		rep.AvgPower = AveragePower(watts, sec.StartIndex, sec.EndIndex)
		rep.AvgHR = AverageHeartRate(hr, sec.StartIndex, sec.EndIndex)
		rep.MaxHR = MaxValue(hr[sec.StartIndex:sec.EndIndex])
		rep.EF = PowerHRRatio(watts, hr, sec.StartIndex, sec.EndIndex)
		rep.DurationSeconds = sec.EndIndex - sec.StartIndex

		result.Reps = append(result.Reps, rep)
		repIdx++

		if secType == "WORK" {
			workReps = append(workReps, rep)
			totalWorkDuration += rep.DurationSeconds
		} else {
			totalRecoveryDuration += rep.DurationSeconds
		}
	}

	result.RepCount = len(workReps)
	result.WorkDuration = float64(totalWorkDuration)
	result.RecoveryDuration = float64(totalRecoveryDuration)

	if len(workReps) >= 2 {
		var workPowers, workHRs []float64
		for i := range workReps {
			workPowers = append(workPowers, workReps[i].AvgPower)
			workHRs = append(workHRs, workReps[i].AvgHR)
		}

		meanPower := mean(workPowers)
		result.Repeatability.WorkPowerMean = meanPower
		result.Repeatability.WorkPowerStdDev = stdDev(workPowers, meanPower)
		if meanPower > 0 {
			result.Repeatability.WorkPowerCV = result.Repeatability.WorkPowerStdDev / meanPower * percentScale
		}

		meanHR := mean(workHRs)
		result.Repeatability.WorkHRMean = meanHR
		result.Repeatability.WorkHRStdDev = stdDev(workHRs, meanHR)

		result.Repeatability.PowerDrift = workPowers[0] - workPowers[len(workPowers)-1]
		result.Repeatability.HRDrift = workHRs[len(workHRs)-1] - workHRs[0]
	}

	return result
}

func AnalyzeZoneAlignment(zoneTimes []ZoneTime, hrZoneTimes []int) ZoneAlignmentAnalysis {
	var result ZoneAlignmentAnalysis

	totalPowerSecs := sumZoneSeconds(zoneTimes)
	totalHRSecs := sumHRSlice(hrZoneTimes)

	if totalPowerSecs == 0 && totalHRSecs == 0 {
		return result
	}

	powerZ1Z2, powerZ3Plus := partitionPowerZones(zoneTimes)
	hrZ1Z2, hrZ3Plus := partitionHRZones(hrZoneTimes)

	if totalPowerSecs > 0 {
		result.PowerZ1Z2Pct = round2(float64(powerZ1Z2) / float64(totalPowerSecs) * percentScale)
		result.PowerZ3PlusPct = round2(float64(powerZ3Plus) / float64(totalPowerSecs) * percentScale)
	}
	if totalHRSecs > 0 {
		result.HRZ1Z2Pct = round2(float64(hrZ1Z2) / float64(totalHRSecs) * percentScale)
		result.HRZ3PlusPct = round2(float64(hrZ3Plus) / float64(totalHRSecs) * percentScale)
	}

	if totalPowerSecs > 0 && totalHRSecs > 0 {
		rawPowerZ3Plus := float64(powerZ3Plus) / float64(totalPowerSecs) * percentScale
		rawHRZ3Plus := float64(hrZ3Plus) / float64(totalHRSecs) * percentScale
		result.AlignmentGap = round2(rawHRZ3Plus - rawPowerZ3Plus)
	}

	switch {
	case result.AlignmentGap > zoneAlignmentHotGap:
		result.Diagnosis = "hot"
	case result.AlignmentGap < zoneAlignmentColdGap:
		result.Diagnosis = "cold"
	default:
		result.Diagnosis = "normal"
	}

	return result
}

func sumZoneSeconds(zoneTimes []ZoneTime) int {
	var total int
	for i := range zoneTimes {
		total += zoneTimes[i].Secs
	}
	return total
}

func sumHRSlice(hr []int) int {
	var total int
	for i := range hr {
		total += hr[i]
	}
	return total
}

func partitionPowerZones(zoneTimes []ZoneTime) (int, int) { //nolint:gocritic
	var z1z2, z3plus int
	for i := range zoneTimes {
		if zoneTimes[i].ID == "Z1" || zoneTimes[i].ID == "Z2" {
			z1z2 += zoneTimes[i].Secs
		} else {
			z3plus += zoneTimes[i].Secs
		}
	}
	return z1z2, z3plus
}

func partitionHRZones(hrZoneTimes []int) (int, int) { //nolint:gocritic
	var z1z2, z3plus int
	for i := range hrZoneTimes {
		if i <= 1 {
			z1z2 += hrZoneTimes[i]
		} else {
			z3plus += hrZoneTimes[i]
		}
	}
	return z1z2, z3plus
}

func AnalyzeActivityMicro(activity *Activity, streams StreamData, intervals *IntervalsDTO, ftp, lthr int) ActivityMicroAnalysis { //nolint:gocritic
	var result ActivityMicroAnalysis

	if activity == nil {
		return result
	}

	result.ActivityID = activity.ID
	result.ActivityName = activity.Name
	result.SessionSummary = cyclingSessionFromActivity(activity)
	result.ZoneAlignment = AnalyzeZoneAlignment(activity.ZoneTimes, activity.HRZoneTimes)

	watts, hr := findStreamPair(streams)
	hasStreams := len(watts) > 0 && len(hr) > 0

	if hasStreams {
		warmupSection, cooldownSection, _ := DetectSectionsByHRStabilization(hr, watts)

		warmup := AnalyzeWarmup(watts, hr, warmupSection, lthr)
		if warmup.DurationSeconds > 0 {
			result.Warmup = &warmup
		}

		cd := AnalyzeCooldown(watts, hr, cooldownSection, lthr)
		if cd.DurationSeconds > 0 {
			result.Cooldown = &cd
		}
	}

	if intervals != nil && len(intervals.Intervals) > 0 && hasStreams {
		intervalSections := DetectIntervalsFromDTO(intervals)
		iv := AnalyzeIntervals(intervalSections, watts, hr, ftp)
		if iv.RepCount > 0 {
			result.Intervals = &iv
		}
	}

	return result
}

func findStreamPair(streams StreamData) ([]float64, []float64) { //nolint:gocritic
	if len(streams) == 0 {
		return nil, nil
	}

	watts := streamValue(streams, "watts")
	hr := streamValue(streams, "heartrate")
	return watts, hr
}

func streamValue(streams StreamData, key string) []float64 {
	if v := streams[key]; v != nil {
		return v
	}

	for k, v := range streams {
		if strings.EqualFold(k, key) {
			return v
		}
	}

	// Known alternate keys for common stream types.
	// Only reached if case-insensitive match also fails.
	if key == "heartrate" {
		if v := streams["heart_rate"]; v != nil {
			return v
		}
	}
	if key == "watts" {
		if v := streams["power"]; v != nil {
			return v
		}
	}

	return nil
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	var sum float64
	for i := range data {
		sum += data[i]
	}

	return sum / float64(len(data))
}

func stdDev(data []float64, avg float64) float64 {
	if len(data) <= 1 {
		return 0
	}

	var sum float64
	for i := range data {
		diff := data[i] - avg
		sum += diff * diff
	}

	return math.Sqrt(sum / float64(len(data)-1))
}
