package icu

import (
	"sort"
	"time"
)

const (
	recentWellnessDays         = 7
	minimumTrendSamples        = 14
	hoursPerDay                = 24
	hrvWatchRatioThreshold     = 0.97
	hrvRedRatioThreshold       = 0.9
	restingHRWatchDelta        = 5
	restingHRRedDelta          = 8
	sleepWatchScore            = 75
	sleepRedScore              = 65
	negativeFormWatchThreshold = -10
	negativeFormRedThreshold   = -25
)

type WellnessAnalysis struct {
	Scope      WellnessScope      `json:"scope"`
	Coverage   WellnessCoverage   `json:"coverage"`
	HRV        WellnessSignal     `json:"hrv"`
	RestingHR  WellnessSignal     `json:"restingHr"`
	Sleep      WellnessSignal     `json:"sleep"`
	Subjective SubjectiveWellness `json:"subjective"`
	Load       WellnessLoadState  `json:"load"`
	State      PhysiologyState    `json:"state"`
	Warnings   []string           `json:"warnings,omitempty"`
}

type WellnessScope struct {
	Records   int    `json:"records"`
	TotalDays int    `json:"totalDays"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

type WellnessCoverage struct {
	HRV        float64 `json:"hrv"`
	RestingHR  float64 `json:"restingHr"`
	Sleep      float64 `json:"sleep"`
	Subjective float64 `json:"subjective"`
}

type WellnessSignal struct {
	Mean            float64 `json:"mean,omitempty"`
	Latest          float64 `json:"latest,omitempty"`
	Ratio           float64 `json:"ratio,omitempty"`
	Delta           float64 `json:"delta,omitempty"`
	Trend7Day       float64 `json:"trend7d,omitempty"`
	Samples         int     `json:"samples"`
	CoveragePercent float64 `json:"coveragePercent"`
}

type SubjectiveWellness struct {
	Samples         int     `json:"samples"`
	CoveragePercent float64 `json:"coveragePercent"`
	MeanFatigue     float64 `json:"meanFatigue,omitempty"`
	MeanStress      float64 `json:"meanStress,omitempty"`
	MeanSoreness    float64 `json:"meanSoreness,omitempty"`
	MeanMotivation  float64 `json:"meanMotivation,omitempty"`
}

type WellnessLoadState struct {
	CTL float64 `json:"ctl,omitempty"`
	ATL float64 `json:"atl,omitempty"`
	TSB float64 `json:"tsb,omitempty"`
}

type PhysiologyState struct {
	State      string   `json:"state,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Source     string   `json:"source,omitempty"`
	Reasons    []string `json:"reasons,omitempty"`
}

type wellnessSample struct {
	date  string
	value float64
}

type wellnessAccumulator struct {
	analysis   WellnessAnalysis
	hrv        []wellnessSample
	restingHR  []wellnessSample
	sleep      []wellnessSample
	fatigue    numericAccumulator
	stress     numericAccumulator
	soreness   numericAccumulator
	motivation numericAccumulator
	subjective int
	latestLoad string
}

func AnalyzeWellness(records []Wellness, options AnalysisOptions) WellnessAnalysis {
	accumulator := newWellnessAccumulator(records, options)

	for index := range records {
		accumulator.add(&records[index])
	}

	return accumulator.finish()
}

func newWellnessAccumulator(records []Wellness, options AnalysisOptions) wellnessAccumulator {
	var accumulator wellnessAccumulator

	accumulator.analysis.Scope.Records = len(records)
	accumulator.analysis.Scope.StartDate = options.StartDate
	accumulator.analysis.Scope.EndDate = options.EndDate
	accumulator.analysis.Scope.TotalDays = analysisTotalDays(records, options)

	return accumulator
}

func (accumulator *wellnessAccumulator) add(record *Wellness) {
	accumulator.addHRV(record)
	accumulator.addRestingHR(record)
	accumulator.addSleep(record)
	accumulator.addSubjective(record)
	accumulator.addLoad(record)
}

func (accumulator *wellnessAccumulator) addHRV(record *Wellness) {
	if record.HRV == 0 {
		return
	}

	accumulator.hrv = append(accumulator.hrv, wellnessSample{date: record.ID, value: record.HRV})
}

func (accumulator *wellnessAccumulator) addRestingHR(record *Wellness) {
	if record.RestingHR == 0 {
		return
	}

	accumulator.restingHR = append(accumulator.restingHR, wellnessSample{
		date:  record.ID,
		value: float64(record.RestingHR),
	})
}

func (accumulator *wellnessAccumulator) addSleep(record *Wellness) {
	if record.SleepScore == 0 {
		return
	}

	accumulator.sleep = append(accumulator.sleep, wellnessSample{date: record.ID, value: record.SleepScore})
}

func (accumulator *wellnessAccumulator) addSubjective(record *Wellness) {
	var hasSubjective bool

	addSubjectiveMetric(record.Fatigue, &accumulator.fatigue, &hasSubjective)
	addSubjectiveMetric(record.Stress, &accumulator.stress, &hasSubjective)
	addSubjectiveMetric(record.Soreness, &accumulator.soreness, &hasSubjective)
	addSubjectiveMetric(record.Motivation, &accumulator.motivation, &hasSubjective)

	if hasSubjective {
		accumulator.subjective++
	}
}

func addSubjectiveMetric(value int, accumulator *numericAccumulator, hasSubjective *bool) {
	if value == 0 {
		return
	}

	accumulator.add(float64(value))

	*hasSubjective = true
}

func (accumulator *wellnessAccumulator) addLoad(record *Wellness) {
	if record.CTL == 0 && record.ATL == 0 {
		return
	}

	if record.ID < accumulator.latestLoad {
		return
	}

	accumulator.latestLoad = record.ID
	accumulator.analysis.Load.CTL = record.CTL
	accumulator.analysis.Load.ATL = record.ATL
	accumulator.analysis.Load.TSB = round2(record.CTL - record.ATL)
}

func (accumulator *wellnessAccumulator) finish() WellnessAnalysis {
	accumulator.analysis.HRV = wellnessSignal(accumulator.hrv, accumulator.analysis.Scope.TotalDays)
	accumulator.analysis.RestingHR = wellnessSignal(accumulator.restingHR, accumulator.analysis.Scope.TotalDays)
	accumulator.analysis.Sleep = wellnessSignal(accumulator.sleep, accumulator.analysis.Scope.TotalDays)
	accumulator.analysis.Coverage = accumulator.coverage()
	accumulator.analysis.Subjective = accumulator.subjectiveSummary()
	accumulator.analysis.State = physiologyState(&accumulator.analysis)

	if accumulator.analysis.Scope.Records == 0 {
		accumulator.analysis.Warnings = append(accumulator.analysis.Warnings, "no wellness records found")
	}

	return accumulator.analysis
}

func (accumulator *wellnessAccumulator) coverage() WellnessCoverage {
	return WellnessCoverage{
		HRV:        accumulator.analysis.HRV.CoveragePercent,
		RestingHR:  accumulator.analysis.RestingHR.CoveragePercent,
		Sleep:      accumulator.analysis.Sleep.CoveragePercent,
		Subjective: coveragePercent(accumulator.subjective, accumulator.analysis.Scope.TotalDays),
	}
}

func (accumulator *wellnessAccumulator) subjectiveSummary() SubjectiveWellness {
	return SubjectiveWellness{
		Samples:         accumulator.subjective,
		CoveragePercent: coveragePercent(accumulator.subjective, accumulator.analysis.Scope.TotalDays),
		MeanFatigue:     round2(accumulator.fatigue.average()),
		MeanStress:      round2(accumulator.stress.average()),
		MeanSoreness:    round2(accumulator.soreness.average()),
		MeanMotivation:  round2(accumulator.motivation.average()),
	}
}

func wellnessSignal(samples []wellnessSample, totalDays int) WellnessSignal {
	if len(samples) == 0 {
		return WellnessSignal{
			Mean:            0,
			Latest:          0,
			Ratio:           0,
			Delta:           0,
			Trend7Day:       0,
			Samples:         0,
			CoveragePercent: 0,
		}
	}

	sortWellnessSamples(samples)

	mean := wellnessMean(samples)
	latest := samples[len(samples)-1].value

	return WellnessSignal{
		Mean:            mean,
		Latest:          latest,
		Ratio:           ratio(latest, mean),
		Delta:           round2(latest - mean),
		Trend7Day:       wellnessTrend7Day(samples),
		Samples:         len(samples),
		CoveragePercent: coveragePercent(len(samples), totalDays),
	}
}

func sortWellnessSamples(samples []wellnessSample) {
	sort.Slice(samples, func(leftIndex, rightIndex int) bool {
		return samples[leftIndex].date < samples[rightIndex].date
	})
}

func wellnessMean(samples []wellnessSample) float64 {
	var sum float64

	for _, sample := range samples {
		sum += sample.value
	}

	return round2(sum / float64(len(samples)))
}

func wellnessTrend7Day(samples []wellnessSample) float64 {
	if len(samples) < minimumTrendSamples {
		return 0
	}

	recent := sampleMean(samples[len(samples)-recentWellnessDays:])
	previous := sampleMean(samples[len(samples)-minimumTrendSamples : len(samples)-recentWellnessDays])

	return round2(recent - previous)
}

func sampleMean(samples []wellnessSample) float64 {
	var sum float64

	for _, sample := range samples {
		sum += sample.value
	}

	return sum / float64(len(samples))
}

func coveragePercent(samples, totalDays int) float64 {
	if totalDays == 0 {
		return 0
	}

	return round2(float64(samples) * percentScale / float64(totalDays))
}

func ratio(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}

	return round2(numerator / denominator)
}

func physiologyState(analysis *WellnessAnalysis) PhysiologyState {
	var (
		reasons []string
		state   = "OK"
	)

	updatePhysiologyState(&state, &reasons, hrvPhysiologyState(analysis.HRV.Ratio))
	updatePhysiologyState(&state, &reasons, restingHRPhysiologyState(analysis.RestingHR.Delta))
	updatePhysiologyState(&state, &reasons, sleepPhysiologyState(analysis.Sleep.Latest))
	updatePhysiologyState(&state, &reasons, loadPhysiologyState(analysis.Load.TSB))

	return PhysiologyState{
		State:      state,
		Confidence: physiologyConfidence(analysis),
		Source:     "local_wellness_heuristic",
		Reasons:    reasons,
	}
}

func updatePhysiologyState(current *string, reasons *[]string, next PhysiologyState) {
	if next.State == "" || next.State == "OK" {
		return
	}

	*reasons = append(*reasons, next.Reasons...)

	if next.State == "RED" || *current == "OK" {
		*current = next.State
	}
}

func hrvPhysiologyState(hrvRatio float64) PhysiologyState {
	if hrvRatio == 0 {
		return newPhysiologyState("")
	}

	if hrvRatio < hrvRedRatioThreshold {
		return newPhysiologyState("RED", "hrv_ratio_red")
	}

	if hrvRatio < hrvWatchRatioThreshold {
		return newPhysiologyState("WATCH", "hrv_ratio_watch")
	}

	return newPhysiologyState("OK")
}

func restingHRPhysiologyState(restingHRDelta float64) PhysiologyState {
	if restingHRDelta >= restingHRRedDelta {
		return newPhysiologyState("RED", "resting_hr_delta_red")
	}

	if restingHRDelta >= restingHRWatchDelta {
		return newPhysiologyState("WATCH", "resting_hr_delta_watch")
	}

	return newPhysiologyState("OK")
}

func sleepPhysiologyState(sleepScore float64) PhysiologyState {
	if sleepScore == 0 {
		return newPhysiologyState("")
	}

	if sleepScore < sleepRedScore {
		return newPhysiologyState("RED", "sleep_score_red")
	}

	if sleepScore < sleepWatchScore {
		return newPhysiologyState("WATCH", "sleep_score_watch")
	}

	return newPhysiologyState("OK")
}

func loadPhysiologyState(tsb float64) PhysiologyState {
	if tsb <= negativeFormRedThreshold {
		return newPhysiologyState("RED", "negative_form_red")
	}

	if tsb <= negativeFormWatchThreshold {
		return newPhysiologyState("WATCH", "negative_form_watch")
	}

	return newPhysiologyState("OK")
}

func newPhysiologyState(state string, reasons ...string) PhysiologyState {
	return PhysiologyState{
		State:      state,
		Confidence: "",
		Source:     "",
		Reasons:    reasons,
	}
}

func physiologyConfidence(analysis *WellnessAnalysis) string {
	if analysis.Coverage.HRV >= 70 && analysis.Coverage.RestingHR >= 70 && analysis.Coverage.Sleep >= 70 {
		return "high"
	}

	if analysis.Coverage.HRV >= 40 || analysis.Coverage.RestingHR >= 40 || analysis.Coverage.Sleep >= 40 {
		return "medium"
	}

	return "low"
}

func analysisTotalDays(records []Wellness, options AnalysisOptions) int {
	optionDays := daysBetween(options.StartDate, options.EndDate)
	if optionDays > 0 {
		return optionDays
	}

	return uniqueWellnessDays(records)
}

func daysBetween(start, end string) int {
	startDate, startErr := time.Parse(analysisDateLayout, start)
	endDate, endErr := time.Parse(analysisDateLayout, end)

	if startErr != nil || endErr != nil || endDate.Before(startDate) {
		return 0
	}

	return int(endDate.Sub(startDate).Hours()/hoursPerDay) + 1
}

func uniqueWellnessDays(records []Wellness) int {
	dates := map[string]bool{}

	for index := range records {
		if records[index].ID != "" {
			dates[records[index].ID] = true
		}
	}

	return len(dates)
}
