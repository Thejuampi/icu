package icu

import (
	"math"
	"sort"
	"strings"
	"time"
)

const (
	analysisDateLayout     = "2006-01-02"
	acuteLoadDays          = 7
	chronicLoadDays        = 28
	longEnduranceSeconds   = 7200
	secondsPerMinute       = 60
	secondsPerHour         = 3600
	metersPerKilometer     = 1000
	percentScale           = 100
	thousandScale          = 1000
	highIntensityThreshold = 0.85
	highDecouplingPct      = 5
)

type AnalysisOptions struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

type CyclingAnalysis struct {
	Scope       CyclingScope                   `json:"scope"`
	State       CyclingState                   `json:"state"`
	Volume      CyclingVolume                  `json:"volume"`
	Load        CyclingLoad                    `json:"load"`
	Intensity   CyclingIntensity               `json:"intensity"`
	Durability  CyclingDurability              `json:"durability"`
	Anaerobic   CyclingAnaerobic               `json:"anaerobic"`
	Performance CyclingPerformanceIntelligence `json:"performance"`
	Sessions    []CyclingSession               `json:"sessions,omitempty"`
	Warnings    []string                       `json:"warnings,omitempty"`
}

type CyclingState struct {
	StateLabel        string  `json:"stateLabel,omitempty"`
	OperationalState  string  `json:"operationalState,omitempty"`
	LoadRecoveryState string  `json:"loadRecoveryState,omitempty"`
	LoadPressure      float64 `json:"loadPressure,omitempty"`
	Directive         string  `json:"directive,omitempty"`
	Source            string  `json:"source,omitempty"`
}

type CyclingScope struct {
	Activities        int    `json:"activities"`
	CyclingActivities int    `json:"cyclingActivities"`
	StartDate         string `json:"startDate,omitempty"`
	EndDate           string `json:"endDate,omitempty"`
}

type CyclingVolume struct {
	MovingTimeSecs      int     `json:"movingTimeSecs"`
	MovingTimeHours     float64 `json:"movingTimeHours"`
	DistanceMeters      float64 `json:"distanceMeters"`
	DistanceKilometers  float64 `json:"distanceKilometers"`
	ElevationGainMeters float64 `json:"elevationGainMeters"`
}

type CyclingLoad struct {
	Total                 int                 `json:"total"`
	AveragePerActivity    float64             `json:"averagePerActivity"`
	PerHour               float64             `json:"perHour"`
	Monotony              float64             `json:"monotony"`
	Strain                float64             `json:"strain"`
	Acute7                int                 `json:"acute7"`
	Chronic28             int                 `json:"chronic28"`
	AcuteChronicWorkRatio float64             `json:"acuteChronicWorkRatio"`
	LatestCTL             float64             `json:"latestCtl,omitempty"`
	LatestATL             float64             `json:"latestAtl,omitempty"`
	LatestForm            float64             `json:"latestForm,omitempty"`
	Daily                 []DailyTrainingLoad `json:"daily"`
}

type DailyTrainingLoad struct {
	Date string `json:"date"`
	Load int    `json:"load"`
}

type CyclingIntensity struct {
	WeightedAverageIntensity float64            `json:"weightedAverageIntensity"`
	HighIntensityActivities  int                `json:"highIntensityActivities"`
	ZoneSeconds              map[string]int     `json:"zoneSeconds,omitempty"`
	ZonePercent              map[string]float64 `json:"zonePercent,omitempty"`
	HRZoneSeconds            []int              `json:"hrZoneSeconds,omitempty"`
}

type CyclingDurability struct {
	AverageDecoupling        float64 `json:"averageDecoupling"`
	HighDecouplingActivities int     `json:"highDecouplingActivities"`
	AverageEfficiencyFactor  float64 `json:"averageEfficiencyFactor"`
	AverageVariabilityIndex  float64 `json:"averageVariabilityIndex"`
}

type CyclingAnaerobic struct {
	TotalJoulesAboveFTP int `json:"totalJoulesAboveFtp"`
	MaxWBalDepletion    int `json:"maxWbalDepletion"`
	WBalActivities      int `json:"wbalActivities"`
}

type CyclingSession struct {
	ID                 string  `json:"id,omitempty"`
	Date               string  `json:"date,omitempty"`
	Name               string  `json:"name,omitempty"`
	Type               string  `json:"type,omitempty"`
	MovingTimeSecs     int     `json:"movingTimeSecs"`
	MovingTimeMinutes  float64 `json:"movingTimeMinutes"`
	DistanceMeters     float64 `json:"distanceMeters"`
	DistanceKilometers float64 `json:"distanceKilometers"`
	TrainingLoad       int     `json:"trainingLoad"`
	Intensity          float64 `json:"intensity"`
	WeightedAvgPower   int     `json:"weightedAvgPower,omitempty"`
	AverageHeartRate   int     `json:"averageHeartRate,omitempty"`
	Decoupling         float64 `json:"decoupling,omitempty"`
	EfficiencyFactor   float64 `json:"efficiencyFactor,omitempty"`
	VariabilityIndex   float64 `json:"variabilityIndex,omitempty"`
	JoulesAboveFTP     int     `json:"joulesAboveFtp,omitempty"`
	MaxWBalDepletion   int     `json:"maxWbalDepletion,omitempty"`
}

type CyclingPerformanceIntelligence struct {
	Repeatability CyclingRepeatability    `json:"repeatability"`
	Durability    CyclingDurabilitySignal `json:"durability"`
	NeuralDensity CyclingNeuralDensity    `json:"neuralDensity"`
}

type CyclingRepeatability struct {
	TotalWorkAboveFTP     int     `json:"totalWorkAboveFtp"`
	MaxWBalDepletion      int     `json:"maxWbalDepletion,omitempty"`
	MeanMaxWBalDepletion  float64 `json:"meanMaxWbalDepletion,omitempty"`
	SessionsWithWBalData  int     `json:"sessionsWithWbalData"`
	Classification        string  `json:"classification,omitempty"`
	ClassificationContext string  `json:"classificationContext,omitempty"`
}

type CyclingDurabilitySignal struct {
	State                 string  `json:"state,omitempty"`
	MeanDecoupling        float64 `json:"meanDecoupling"`
	MaxDecoupling         float64 `json:"maxDecoupling"`
	HighDriftSessions     int     `json:"highDriftSessions"`
	LongEnduranceSessions int     `json:"longEnduranceSessions"`
	Classification        string  `json:"classification,omitempty"`
}

type CyclingNeuralDensity struct {
	RollingWorkAboveFTP  int     `json:"rollingWorkAboveFtp"`
	HighIntensityDays    int     `json:"highIntensityDays"`
	MeanIntensity        float64 `json:"meanIntensity"`
	MeanEfficiencyFactor float64 `json:"meanEfficiencyFactor"`
	MeanVariabilityIndex float64 `json:"meanVariabilityIndex"`
	Classification       string  `json:"classification,omitempty"`
}

type cyclingAnalysisAccumulator struct {
	analysis             CyclingAnalysis
	dailyLoads           map[string]int
	intensityWeightedSum float64
	intensity            numericAccumulator
	decoupling           numericAccumulator
	efficiency           numericAccumulator
	variability          numericAccumulator
	wbalDepletion        numericAccumulator
	maxDecoupling        float64
	longEndurance        int
	latestLoadDate       string
}

type numericAccumulator struct {
	sum   float64
	count int
}

func AnalyzeCyclingActivities(activities []Activity, options AnalysisOptions) CyclingAnalysis {
	accumulator := newCyclingAnalysisAccumulator(len(activities))

	for index := range activities {
		activity := &activities[index]
		if !IsCyclingActivityType(activity.Type) {
			continue
		}

		accumulator.add(activity)
	}

	return accumulator.finish(options)
}

func newCyclingAnalysisAccumulator(totalActivities int) cyclingAnalysisAccumulator {
	var accumulator cyclingAnalysisAccumulator

	accumulator.analysis.Scope.Activities = totalActivities
	accumulator.analysis.Intensity.ZoneSeconds = map[string]int{}
	accumulator.analysis.Intensity.ZonePercent = map[string]float64{}
	accumulator.dailyLoads = map[string]int{}

	return accumulator
}

func (accumulator *cyclingAnalysisAccumulator) add(activity *Activity) {
	accumulator.analysis.Scope.CyclingActivities++
	accumulator.addVolume(activity)
	accumulator.addLoad(activity)
	accumulator.addIntensity(activity)
	accumulator.addDurability(activity)
	accumulator.addAnaerobic(activity)
	accumulator.addDate(activity)
	accumulator.addSession(activity)
}

func (accumulator *cyclingAnalysisAccumulator) addVolume(activity *Activity) {
	accumulator.analysis.Volume.MovingTimeSecs += activity.MovingTime
	accumulator.analysis.Volume.DistanceMeters += activity.Distance
	accumulator.analysis.Volume.ElevationGainMeters += activity.TotalElevationGain

	if activity.MovingTime >= longEnduranceSeconds {
		accumulator.longEndurance++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addLoad(activity *Activity) {
	accumulator.analysis.Load.Total += activity.TrainingLoad
}

func (accumulator *cyclingAnalysisAccumulator) addIntensity(activity *Activity) {
	accumulator.intensityWeightedSum += activity.Intensity * float64(activity.MovingTime)

	if activity.Intensity != 0 {
		accumulator.intensity.add(activity.Intensity)
	}

	if activity.Intensity >= highIntensityThreshold {
		accumulator.analysis.Intensity.HighIntensityActivities++
	}

	for _, zone := range activity.ZoneTimes {
		accumulator.analysis.Intensity.ZoneSeconds[zone.ID] += zone.Secs
	}

	mergeHRZoneSeconds(&accumulator.analysis.Intensity.HRZoneSeconds, activity.HRZoneTimes)
}

func (accumulator *cyclingAnalysisAccumulator) addDurability(activity *Activity) {
	if activity.Decoupling != 0 {
		accumulator.decoupling.add(activity.Decoupling)
		accumulator.updateMaxDecoupling(activity.Decoupling)
		accumulator.addHighDecoupling(activity.Decoupling)
	}

	if activity.EfficiencyFactor != 0 {
		accumulator.efficiency.add(activity.EfficiencyFactor)
	}

	if activity.VariabilityIndex != 0 {
		accumulator.variability.add(activity.VariabilityIndex)
	}
}

func (accumulator *cyclingAnalysisAccumulator) updateMaxDecoupling(decoupling float64) {
	absoluteDecoupling := math.Abs(decoupling)

	if absoluteDecoupling > accumulator.maxDecoupling {
		accumulator.maxDecoupling = absoluteDecoupling
	}
}

func (accumulator *cyclingAnalysisAccumulator) addHighDecoupling(decoupling float64) {
	if math.Abs(decoupling) >= highDecouplingPct {
		accumulator.analysis.Durability.HighDecouplingActivities++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addAnaerobic(activity *Activity) {
	accumulator.analysis.Anaerobic.TotalJoulesAboveFTP += activity.JoulesAboveFTP

	if activity.MaxWbalDepletion > 0 {
		accumulator.analysis.Anaerobic.WBalActivities++
		accumulator.wbalDepletion.add(float64(activity.MaxWbalDepletion))
	}

	if activity.MaxWbalDepletion > accumulator.analysis.Anaerobic.MaxWBalDepletion {
		accumulator.analysis.Anaerobic.MaxWBalDepletion = activity.MaxWbalDepletion
	}
}

func (accumulator *cyclingAnalysisAccumulator) addDate(activity *Activity) {
	date := activityDate(activity)
	if date == "" {
		return
	}

	accumulator.dailyLoads[date] += activity.TrainingLoad
	accumulator.analysis.Scope.StartDate = minDateString(accumulator.analysis.Scope.StartDate, date)
	accumulator.analysis.Scope.EndDate = maxDateString(accumulator.analysis.Scope.EndDate, date)
	accumulator.updateLatestLoad(activity, date)
}

func (accumulator *cyclingAnalysisAccumulator) addSession(activity *Activity) {
	accumulator.analysis.Sessions = append(accumulator.analysis.Sessions, cyclingSessionFromActivity(activity))
}

func (accumulator *cyclingAnalysisAccumulator) updateLatestLoad(activity *Activity, date string) {
	if date < accumulator.latestLoadDate {
		return
	}

	accumulator.latestLoadDate = date
	accumulator.analysis.Load.LatestCTL = activity.CTL
	accumulator.analysis.Load.LatestATL = activity.ATL
	accumulator.analysis.Load.LatestForm = round2(activity.CTL - activity.ATL)
}

func (accumulator *cyclingAnalysisAccumulator) finish(options AnalysisOptions) CyclingAnalysis {
	accumulator.finishVolume()

	if accumulator.analysis.Scope.CyclingActivities == 0 {
		accumulator.analysis.Warnings = append(accumulator.analysis.Warnings, "no cycling activities found")

		return accumulator.analysis
	}

	accumulator.finishLoad(options)
	accumulator.finishIntensity()
	accumulator.finishDurability()
	accumulator.finishState()
	accumulator.finishPerformance()

	return accumulator.analysis
}

func (accumulator *cyclingAnalysisAccumulator) finishVolume() {
	accumulator.analysis.Volume.MovingTimeHours = round2(float64(accumulator.analysis.Volume.MovingTimeSecs) / secondsPerHour)
	accumulator.analysis.Volume.DistanceKilometers = round2(accumulator.analysis.Volume.DistanceMeters / metersPerKilometer)
}

func (accumulator *cyclingAnalysisAccumulator) finishLoad(options AnalysisOptions) {
	averagePerActivity := float64(accumulator.analysis.Load.Total) /
		float64(accumulator.analysis.Scope.CyclingActivities)

	accumulator.analysis.Load.AveragePerActivity = round2(averagePerActivity)
	accumulator.analysis.Load.Daily = buildDailyTrainingLoads(accumulator.dailyLoads, options)
	accumulator.analysis.Load.Acute7 = sumLastLoads(accumulator.analysis.Load.Daily, acuteLoadDays)
	accumulator.analysis.Load.Chronic28 = sumLastLoads(accumulator.analysis.Load.Daily, chronicLoadDays)
	accumulator.analysis.Load.AcuteChronicWorkRatio = acuteChronicWorkRatio(
		accumulator.analysis.Load.Acute7,
		accumulator.analysis.Load.Chronic28,
	)
	accumulator.analysis.Load.Monotony = round2(trainingMonotony(accumulator.analysis.Load.Daily))
	accumulator.analysis.Load.Strain = round2(float64(accumulator.analysis.Load.Total) * accumulator.analysis.Load.Monotony)
}

func (accumulator *cyclingAnalysisAccumulator) finishIntensity() {
	if accumulator.analysis.Volume.MovingTimeSecs > 0 {
		movingHours := float64(accumulator.analysis.Volume.MovingTimeSecs) / secondsPerHour
		loadPerHour := float64(accumulator.analysis.Load.Total) / movingHours
		weightedIntensity := accumulator.intensityWeightedSum /
			float64(accumulator.analysis.Volume.MovingTimeSecs)

		accumulator.analysis.Load.PerHour = round2(loadPerHour)
		accumulator.analysis.Intensity.WeightedAverageIntensity = round3(weightedIntensity)
	}

	accumulator.analysis.Intensity.ZonePercent = zonePercentages(accumulator.analysis.Intensity.ZoneSeconds)
}

func (accumulator *cyclingAnalysisAccumulator) finishDurability() {
	accumulator.analysis.Durability.AverageDecoupling = round2(accumulator.decoupling.average())
	accumulator.analysis.Durability.AverageEfficiencyFactor = round2(accumulator.efficiency.average())
	accumulator.analysis.Durability.AverageVariabilityIndex = round2(accumulator.variability.average())
}

func (accumulator *cyclingAnalysisAccumulator) finishState() {
	if accumulator.analysis.Load.LatestCTL == 0 && accumulator.analysis.Load.LatestATL == 0 {
		return
	}

	loadPressure := round2(accumulator.analysis.Load.LatestATL - accumulator.analysis.Load.LatestCTL)
	accumulator.analysis.State.LoadPressure = loadPressure
	accumulator.analysis.State.Source = "local_ctl_atl_heuristic"

	if loadPressure > 0 {
		accumulator.analysis.State.StateLabel = "Load Pressure"
		accumulator.analysis.State.OperationalState = "recovery_priority"
		accumulator.analysis.State.LoadRecoveryState = "load_pressure"
		accumulator.analysis.State.Directive = "prioritise aerobic work and recovery spacing"

		return
	}

	accumulator.analysis.State.StateLabel = "Load Accepting"
	accumulator.analysis.State.OperationalState = "load_accepting"
	accumulator.analysis.State.LoadRecoveryState = "load_accepting"
	accumulator.analysis.State.Directive = "continue planned training with normal recovery checks"
}

func (accumulator *cyclingAnalysisAccumulator) finishPerformance() {
	accumulator.analysis.Performance = CyclingPerformanceIntelligence{
		Repeatability: accumulator.repeatabilitySignal(),
		Durability:    accumulator.durabilitySignal(),
		NeuralDensity: accumulator.neuralDensitySignal(),
	}
}

func (accumulator *cyclingAnalysisAccumulator) repeatabilitySignal() CyclingRepeatability {
	return CyclingRepeatability{
		TotalWorkAboveFTP:     accumulator.analysis.Anaerobic.TotalJoulesAboveFTP,
		MaxWBalDepletion:      accumulator.analysis.Anaerobic.MaxWBalDepletion,
		MeanMaxWBalDepletion:  round2(accumulator.wbalDepletion.average()),
		SessionsWithWBalData:  accumulator.analysis.Anaerobic.WBalActivities,
		Classification:        "informational",
		ClassificationContext: "local_activity_fields",
	}
}

func (accumulator *cyclingAnalysisAccumulator) durabilitySignal() CyclingDurabilitySignal {
	return CyclingDurabilitySignal{
		State:                 durabilityState(accumulator.maxDecoupling),
		MeanDecoupling:        accumulator.analysis.Durability.AverageDecoupling,
		MaxDecoupling:         round2(accumulator.maxDecoupling),
		HighDriftSessions:     accumulator.analysis.Durability.HighDecouplingActivities,
		LongEnduranceSessions: accumulator.longEndurance,
		Classification:        "local_heuristic",
	}
}

func (accumulator *cyclingAnalysisAccumulator) neuralDensitySignal() CyclingNeuralDensity {
	return CyclingNeuralDensity{
		RollingWorkAboveFTP:  accumulator.analysis.Anaerobic.TotalJoulesAboveFTP,
		HighIntensityDays:    accumulator.analysis.Intensity.HighIntensityActivities,
		MeanIntensity:        round3(accumulator.intensity.average()),
		MeanEfficiencyFactor: accumulator.analysis.Durability.AverageEfficiencyFactor,
		MeanVariabilityIndex: accumulator.analysis.Durability.AverageVariabilityIndex,
		Classification:       "local_heuristic",
	}
}

func durabilityState(maxDecoupling float64) string {
	if maxDecoupling == 0 {
		return "unknown"
	}

	if maxDecoupling >= highDecouplingPct {
		return "watch"
	}

	return "stable"
}

func (accumulator *numericAccumulator) add(value float64) {
	accumulator.sum += value
	accumulator.count++
}

func (accumulator *numericAccumulator) average() float64 {
	if accumulator.count == 0 {
		return 0
	}

	return accumulator.sum / float64(accumulator.count)
}

func IsCyclingActivityType(activityType string) bool {
	normalized := strings.ToLower(activityType)

	switch normalized {
	case "ride", "virtualride", "ebikeride", "emountainbikeride", "gravelride", "trackride",
		"mountainbikeride", "cyclocross", "handcycle", "velomobile":
		return true
	default:
		return false
	}
}

func acuteChronicWorkRatio(acuteLoad, chronicLoad int) float64 {
	if chronicLoad == 0 {
		return 0
	}

	acuteDaily := float64(acuteLoad) / acuteLoadDays
	chronicDaily := float64(chronicLoad) / chronicLoadDays

	return round2(acuteDaily / chronicDaily)
}

func activityDate(activity *Activity) string {
	if len(activity.StartDateLocal) >= len(analysisDateLayout) {
		return activity.StartDateLocal[:len(analysisDateLayout)]
	}

	if len(activity.StartDate) >= len(analysisDateLayout) {
		return activity.StartDate[:len(analysisDateLayout)]
	}

	return ""
}

func cyclingSessionFromActivity(activity *Activity) CyclingSession {
	return CyclingSession{
		ID:                 activity.ID,
		Date:               activityDate(activity),
		Name:               activity.Name,
		Type:               activity.Type,
		MovingTimeSecs:     activity.MovingTime,
		MovingTimeMinutes:  round2(float64(activity.MovingTime) / secondsPerMinute),
		DistanceMeters:     activity.Distance,
		DistanceKilometers: round2(activity.Distance / metersPerKilometer),
		TrainingLoad:       activity.TrainingLoad,
		Intensity:          activity.Intensity,
		WeightedAvgPower:   activity.WeightedAvgPower,
		AverageHeartRate:   activity.AverageHeartRate,
		Decoupling:         activity.Decoupling,
		EfficiencyFactor:   activity.EfficiencyFactor,
		VariabilityIndex:   activity.VariabilityIndex,
		JoulesAboveFTP:     activity.JoulesAboveFTP,
		MaxWBalDepletion:   activity.MaxWbalDepletion,
	}
}

func minDateString(current, candidate string) string {
	if current == "" || candidate < current {
		return candidate
	}

	return current
}

func maxDateString(current, candidate string) string {
	if current == "" || candidate > current {
		return candidate
	}

	return current
}

func mergeHRZoneSeconds(total *[]int, next []int) {
	if len(next) == 0 {
		return
	}

	for len(*total) < len(next) {
		*total = append(*total, 0)
	}

	for index, seconds := range next {
		(*total)[index] += seconds
	}
}

func buildDailyTrainingLoads(loads map[string]int, options AnalysisOptions) []DailyTrainingLoad {
	start := options.StartDate
	end := options.EndDate

	if start == "" || end == "" {
		return dailyTrainingLoadsFromMap(loads)
	}

	startDate, startErr := time.Parse(analysisDateLayout, start)
	endDate, endErr := time.Parse(analysisDateLayout, end)
	invalidRange := startErr != nil || endErr != nil || endDate.Before(startDate)

	if invalidRange {
		return dailyTrainingLoadsFromMap(loads)
	}

	var days []DailyTrainingLoad

	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		key := current.Format(analysisDateLayout)
		days = append(days, DailyTrainingLoad{Date: key, Load: loads[key]})
	}

	return days
}

func dailyTrainingLoadsFromMap(loads map[string]int) []DailyTrainingLoad {
	dates := make([]string, 0, len(loads))

	for date := range loads {
		dates = append(dates, date)
	}

	sort.Strings(dates)

	days := make([]DailyTrainingLoad, 0, len(dates))

	for _, date := range dates {
		days = append(days, DailyTrainingLoad{Date: date, Load: loads[date]})
	}

	return days
}

func sumLastLoads(days []DailyTrainingLoad, count int) int {
	if len(days) == 0 {
		return 0
	}

	start := len(days) - count

	if start < 0 {
		start = 0
	}

	var total int

	for _, day := range days[start:] {
		total += day.Load
	}

	return total
}

func trainingMonotony(days []DailyTrainingLoad) float64 {
	if len(days) == 0 {
		return 0
	}

	var mean float64

	for _, day := range days {
		mean += float64(day.Load)
	}

	mean /= float64(len(days))

	var variance float64

	for _, day := range days {
		delta := float64(day.Load) - mean
		variance += delta * delta
	}

	standardDeviation := math.Sqrt(variance / float64(len(days)))

	if standardDeviation == 0 {
		return 0
	}

	return mean / standardDeviation
}

func zonePercentages(zoneSeconds map[string]int) map[string]float64 {
	var total int

	for _, seconds := range zoneSeconds {
		total += seconds
	}

	if total == 0 {
		return map[string]float64{}
	}

	percentages := map[string]float64{}

	for zone, seconds := range zoneSeconds {
		percentages[zone] = round2(float64(seconds) * percentScale / float64(total))
	}

	return percentages
}

func round2(value float64) float64 {
	return math.Round(value*percentScale) / percentScale
}

func round3(value float64) float64 {
	return math.Round(value*thousandScale) / thousandScale
}
