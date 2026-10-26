package icu

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Window sizes for trend summaries (method parameters, not thresholds):
// a 7-day recent window compared with the 7 days before it. Readiness
// levels are derived from the athlete's own baseline in
// baseline_thresholds.go.
const (
	recentWellnessDays  = 7
	minimumTrendSamples = 14
	hoursPerDay         = 24
)

type WellnessAnalysis struct {
	Scope      WellnessScope              `json:"scope"`
	Coverage   WellnessCoverage           `json:"coverage"`
	HRV        WellnessSignal             `json:"hrv"`
	RestingHR  WellnessSignal             `json:"restingHr"`
	Sleep      WellnessSignal             `json:"sleep"`
	Lactate    WellnessLactateCalibration `json:"lactate"`
	Subjective SubjectiveWellness         `json:"subjective"`
	Load       WellnessLoadState          `json:"load"`
	State      PhysiologyState            `json:"state"`
	Warnings   []string                   `json:"warnings,omitempty"`
}

type WellnessLactateCalibration struct {
	Mean            float64           `json:"mean,omitempty"`
	Latest          float64           `json:"latest,omitempty"`
	Trend7Day       float64           `json:"trend7d,omitempty"`
	Samples         int               `json:"samples"`
	CoveragePercent float64           `json:"coveragePercent"`
	State           string            `json:"state,omitempty"`
	Source          string            `json:"source,omitempty"`
	Threshold       *DerivedThreshold `json:"threshold,omitempty"`
}

type WellnessScope struct {
	Records        int    `json:"records"`
	TotalDays      int    `json:"totalDays"`
	StartDate      string `json:"startDate,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	TimezoneSource string `json:"timezoneSource,omitempty"`
}

type WellnessCoverage struct {
	HRV        float64 `json:"hrv"`
	RestingHR  float64 `json:"restingHr"`
	Sleep      float64 `json:"sleep"`
	Subjective float64 `json:"subjective"`
}

type WellnessSignal struct {
	Mean              float64 `json:"mean,omitempty"`
	Latest            float64 `json:"latest,omitempty"`
	Ratio             float64 `json:"ratio,omitempty"`
	Delta             float64 `json:"delta,omitempty"`
	Trend7Day         float64 `json:"trend7d,omitempty"`
	RecentMean        float64 `json:"recentMean,omitempty"`
	BaselineMean      float64 `json:"baselineMean,omitempty"`
	BaselineMAD       float64 `json:"baselineMad,omitempty"`
	ZScore            float64 `json:"zScore,omitempty"`
	ZScoreSource      string  `json:"zScoreSource,omitempty"`
	ScoreName         string  `json:"scoreName,omitempty"`
	FallbackScoreName string  `json:"fallbackScoreName,omitempty"`
	Samples           int     `json:"samples"`
	CoveragePercent   float64 `json:"coveragePercent"`
	// Level is OK/WATCH/RED against Threshold, or "" when there is not
	// enough of the athlete's own history to judge.
	Level     string            `json:"level,omitempty"`
	Threshold *DerivedThreshold `json:"threshold,omitempty"`
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
	// Date is the wellness day CTL/ATL/TSB were read from.
	Date string `json:"date,omitempty"`
	// Level/Threshold judge TSB against the athlete's own TSB history.
	Level     string            `json:"level,omitempty"`
	Threshold *DerivedThreshold `json:"threshold,omitempty"`
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
	analysis           WellnessAnalysis
	hrv                []wellnessSample
	restingHR          []wellnessSample
	sleep              []wellnessSample
	lactate            []wellnessSample
	fatigue            numericAccumulator
	stress             numericAccumulator
	soreness           numericAccumulator
	motivation         numericAccumulator
	subjective         int
	latestLoad         string
	sleepPreferredName string
	sleepFallbackName  string
	sleepPreferredDays int
	sleepFallbackDays  int
	tsb                []wellnessSample
	thresholds         *TrainingThresholds
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
	accumulator.analysis.Scope.Timezone = options.Timezone
	accumulator.analysis.Scope.TimezoneSource = options.TimezoneSource
	accumulator.thresholds = options.Thresholds

	return accumulator
}

func (accumulator *wellnessAccumulator) add(record *Wellness) {
	addWellnessSample(record.HRV, &accumulator.hrv, record.ID)
	addWellnessSample(float64(record.RestingHR), &accumulator.restingHR, record.ID)
	accumulator.addSleepScore(record)
	addWellnessSample(record.Lactate, &accumulator.lactate, record.ID)
	accumulator.addSubjective(record)
	accumulator.addLoad(record)
}

func (accumulator *wellnessAccumulator) addSleepScore(record *Wellness) {
	score, preferred := preferredWellnessScore(record)
	if score.Value == 0 {
		return
	}

	addWellnessSample(score.Value, &accumulator.sleep, record.ID)
	if preferred {
		if accumulator.sleepPreferredName == "" {
			accumulator.sleepPreferredName = score.Name
		}
		accumulator.sleepPreferredDays++

		return
	}

	if accumulator.sleepFallbackName == "" {
		accumulator.sleepFallbackName = score.Name
	}
	accumulator.sleepFallbackDays++
}

func preferredWellnessScore(record *Wellness) (NamedWellnessScore, bool) {
	if record.PreferredScore.Name != "" && record.PreferredScore.Value != 0 {
		return record.PreferredScore, true
	}
	if record.SleepScore != 0 {
		return NamedWellnessScore{Name: "sleepScore", Value: record.SleepScore}, false
	}

	return NamedWellnessScore{}, false
}

func addWellnessSample(value float64, samples *[]wellnessSample, date string) {
	if value == 0 {
		return
	}

	*samples = append(*samples, wellnessSample{date: date, value: value})
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

	// Intervals.icu returns projected CTL/ATL for future days; a row after
	// the analysis end date is a forecast, not the current state.
	if end := accumulator.analysis.Scope.EndDate; end != "" && record.ID > end {
		return
	}

	accumulator.tsb = append(accumulator.tsb, wellnessSample{date: record.ID, value: round2(record.CTL - record.ATL)})
	if record.ID < accumulator.latestLoad {
		return
	}

	accumulator.latestLoad = record.ID
	accumulator.analysis.Load.CTL = record.CTL
	accumulator.analysis.Load.ATL = record.ATL
	accumulator.analysis.Load.TSB = round2(record.CTL - record.ATL)
	accumulator.analysis.Load.Date = record.ID
}

func (accumulator *wellnessAccumulator) finish() WellnessAnalysis {
	accumulator.analysis.HRV = hrvWellnessSignal(accumulator.hrv, accumulator.analysis.Scope.TotalDays)
	accumulator.analysis.RestingHR = wellnessSignal(accumulator.restingHR, accumulator.analysis.Scope.TotalDays)
	accumulator.analysis.Sleep = wellnessSignal(accumulator.sleep, accumulator.analysis.Scope.TotalDays)
	accumulator.annotateSleepSignal()
	accumulator.analysis.Lactate = lactateCalibration(accumulator.lactate, accumulator.analysis.Scope.TotalDays)
	accumulator.judgeAgainstBaselines()
	accumulator.analysis.Coverage = accumulator.coverage()
	accumulator.analysis.Subjective = accumulator.subjectiveSummary()
	accumulator.analysis.State = physiologyState(&accumulator.analysis)

	if accumulator.analysis.Scope.Records == 0 {
		accumulator.analysis.Warnings = append(accumulator.analysis.Warnings, "no wellness records found")
	}

	return accumulator.analysis
}

func (accumulator *wellnessAccumulator) annotateSleepSignal() {
	if accumulator.sleepPreferredName != "" {
		accumulator.analysis.Sleep.ScoreName = accumulator.sleepPreferredName
	}
	if accumulator.sleepPreferredName == "" && accumulator.sleepFallbackName != "" {
		accumulator.analysis.Sleep.ScoreName = accumulator.sleepFallbackName

		return
	}
	if accumulator.sleepPreferredName != "" && accumulator.sleepFallbackName != "" && accumulator.sleepFallbackDays > 0 {
		accumulator.analysis.Sleep.FallbackScoreName = accumulator.sleepFallbackName
		accumulator.analysis.Warnings = append(
			accumulator.analysis.Warnings,
			fmt.Sprintf(
				"sleep signal uses %s when available and falls back to %s for %d day(s)",
				accumulator.sleepPreferredName,
				accumulator.sleepFallbackName,
				accumulator.sleepFallbackDays,
			),
		)
	}
}

func lactateCalibration(samples []wellnessSample, totalDays int) WellnessLactateCalibration {
	if len(samples) == 0 {
		return WellnessLactateCalibration{}
	}

	sortWellnessSamples(samples)

	mean := wellnessMean(samples)
	latest := samples[len(samples)-1].value

	return WellnessLactateCalibration{
		Mean:            mean,
		Latest:          latest,
		Trend7Day:       wellnessTrend7Day(samples),
		Samples:         len(samples),
		CoveragePercent: coveragePercent(len(samples), totalDays),
		Source:          "wellness_lactate",
	}
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
		return WellnessSignal{}
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

func hrvWellnessSignal(samples []wellnessSample, totalDays int) WellnessSignal {
	signal := wellnessSignal(samples, totalDays)
	addHRVMovingBaseline(&signal, samples)

	return signal
}

func addHRVMovingBaseline(signal *WellnessSignal, samples []wellnessSample) {
	if len(samples) < minimumTrendSamples {
		return
	}

	sortWellnessSamples(samples)

	recentStart := len(samples) - recentWellnessDays
	if recentStart <= 0 {
		return
	}

	baseline := samples[:recentStart]
	recent := samples[recentStart:]
	if len(baseline) < recentWellnessDays {
		return
	}

	signal.RecentMean = round2(sampleMean(recent))
	signal.BaselineMean = round2(sampleMean(baseline))

	baselineMedian := sampleMedian(baseline)
	baselineMAD := sampleMAD(baseline, baselineMedian)
	if baselineMAD > 0 {
		signal.BaselineMAD = round2(baselineMAD)
		signal.ZScore = round2(0.6745 * (signal.RecentMean - baselineMedian) / baselineMAD)
		signal.ZScoreSource = "mad"

		return
	}

	standardDeviation := sampleStandardDeviation(baseline, signal.BaselineMean)
	if standardDeviation > 0 {
		signal.ZScore = round2((signal.RecentMean - signal.BaselineMean) / standardDeviation)
		signal.ZScoreSource = "stddev"
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

func sampleMedian(samples []wellnessSample) float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.value)
	}

	sort.Float64s(values)

	midpoint := len(values) / 2
	if len(values)%2 == 1 {
		return values[midpoint]
	}

	return (values[midpoint-1] + values[midpoint]) / 2
}

func sampleMAD(samples []wellnessSample, median float64) float64 {
	deviations := make([]float64, 0, len(samples))
	for _, sample := range samples {
		deviations = append(deviations, math.Abs(sample.value-median))
	}

	sort.Float64s(deviations)

	midpoint := len(deviations) / 2
	if len(deviations)%2 == 1 {
		return deviations[midpoint]
	}

	return (deviations[midpoint-1] + deviations[midpoint]) / 2
}

func sampleStandardDeviation(samples []wellnessSample, mean float64) float64 {
	if len(samples) == 0 {
		return 0
	}

	var sumSquares float64
	for _, sample := range samples {
		delta := sample.value - mean
		sumSquares += delta * delta
	}

	return math.Sqrt(sumSquares / float64(len(samples)))
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

	updatePhysiologyState(&state, &reasons, levelPhysiologyState(analysis.HRV.Level, "hrv_below_baseline"))
	updatePhysiologyState(&state, &reasons, levelPhysiologyState(analysis.RestingHR.Level, "resting_hr_above_baseline"))
	updatePhysiologyState(&state, &reasons, levelPhysiologyState(analysis.Sleep.Level, "sleep_below_baseline"))
	updatePhysiologyState(&state, &reasons, levelPhysiologyState(analysis.Load.Level, "form_below_baseline"))

	return PhysiologyState{
		State:      state,
		Confidence: physiologyConfidence(analysis),
		Source:     "athlete_baseline_robust_z",
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

func levelPhysiologyState(level, reason string) PhysiologyState {
	switch level {
	case BaselineLevelRed:
		return newPhysiologyState("RED", reason+"_red")
	case BaselineLevelWatch:
		return newPhysiologyState("WATCH", reason+"_watch")
	case BaselineLevelOK:
		return newPhysiologyState("OK")
	default:
		return newPhysiologyState("")
	}
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

// judgeAgainstBaselines sets each signal's Level/Threshold from the
// athlete's own history (see baseline_thresholds.go). The judged value is the
// latest sample (the 7-day recent mean for HRV), compared with the samples
// before it.
func (accumulator *wellnessAccumulator) judgeAgainstBaselines() {
	overrides := accumulator.thresholds
	method := thresholdMethod(overrides)
	analysis := &accumulator.analysis

	if values := hrvBaselineValues(accumulator.hrv); analysis.HRV.RecentMean != 0 {
		analysis.HRV.Threshold = deriveThreshold(values, baselineLowIsBad, method,
			overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.HRVWatch }),
			overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.HRVRed }))
		analysis.HRV.Level = thresholdLevel(analysis.HRV.RecentMean, analysis.HRV.Threshold, baselineLowIsBad)
	}
	judgeLatest(&analysis.RestingHR, accumulator.restingHR, baselineHighIsBad, method,
		overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.RestingHRWatch }),
		overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.RestingHRRed }))
	judgeLatest(&analysis.Sleep, accumulator.sleep, baselineLowIsBad, method,
		overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.SleepWatch }),
		overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.SleepRed }))

	var lactate WellnessSignal
	judgeLatest(&lactate, accumulator.lactate, baselineHighIsBad, method,
		overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.LactateWatch }),
		overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.LactateHigh }))
	analysis.Lactate.Threshold = lactate.Threshold
	analysis.Lactate.State = lactateStateFromLevel(lactate.Level, len(accumulator.lactate))

	if len(accumulator.tsb) > 0 {
		sortWellnessSamples(accumulator.tsb)
		baseline := sampleValues(accumulator.tsb[:len(accumulator.tsb)-1])
		analysis.Load.Threshold = deriveThreshold(baseline, baselineLowIsBad, method,
			overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.TSBWatch }),
			overrideOf(overrides, func(t *TrainingThresholds) *float64 { return t.TSBRed }))
		analysis.Load.Level = thresholdLevel(analysis.Load.TSB, analysis.Load.Threshold, baselineLowIsBad)
	}
}

func judgeLatest(
	signal *WellnessSignal, samples []wellnessSample, direction int, method baselineMethod, watch, red *float64,
) {
	if len(samples) == 0 {
		return
	}
	sortWellnessSamples(samples)
	signal.Threshold = deriveThreshold(sampleValues(samples[:len(samples)-1]), direction, method, watch, red)
	signal.Level = thresholdLevel(samples[len(samples)-1].value, signal.Threshold, direction)
}

// hrvBaselineValues is the HRV history before the recent window, which the
// recent mean is judged against.
func hrvBaselineValues(samples []wellnessSample) []float64 {
	if len(samples) <= recentWellnessDays {
		return nil
	}
	sortWellnessSamples(samples)

	return sampleValues(samples[:len(samples)-recentWellnessDays])
}

func sampleValues(samples []wellnessSample) []float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.value)
	}

	return values
}

func lactateStateFromLevel(level string, samples int) string {
	switch {
	case samples == 0:
		return ""
	case level == BaselineLevelRed:
		return "high"
	case level == BaselineLevelWatch:
		return "watch"
	case level == BaselineLevelOK:
		return "baseline"
	default:
		return "insufficient_history"
	}
}
