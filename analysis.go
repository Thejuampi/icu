package icu

import (
	"math"
	"sort"
	"strings"
	"time"
)

const (
	analysisDateLayout       = "2006-01-02"
	acuteLoadDays            = 7
	chronicLoadDays          = 28
	longEnduranceSeconds     = 7200
	secondsPerMinute         = 60
	secondsPerHour           = 3600
	metersPerKilometer       = 1000
	percentScale             = 100
	thousandScale            = 1000
	unknownState             = "unknown"
	stateLabelLoadPressure   = "Load Pressure"
	stateOperationalRecovery = "recovery_priority"
	stateLoadPressure        = "load_pressure"
	stateDurabilityWatch     = "watch"
	highIntensityThreshold   = 0.85
	highDecouplingPct        = 5
	hotTempThreshold         = 30
	hotFeelsLikeThreshold    = 32
	windSpeedThreshold       = 15
	headwindThreshold        = 40
	climbingGradientPct      = 3
	wPrimeRepeatablePct      = 50
	wPrimeModeratePct        = 30
	isdmBaseScore            = 100
	isdmDecouplingPenalty    = 5
	isdmHighDriftPenalty     = 10
	lowEfficiencyThreshold   = 1.2
	highEfficiencyThreshold  = 1.4

	// DefaultAnalysisTimezone is the timezone used for computed analysis date ranges.
	DefaultAnalysisTimezone = "UTC"
	// DefaultAnalysisTimezoneSource documents how the analysis timezone was selected.
	DefaultAnalysisTimezoneSource = "default_utc"
)

type AnalysisOptions struct {
	StartDate      string `json:"startDate,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	TimezoneSource string `json:"timezoneSource,omitempty"`
}

type CyclingAnalysis struct {
	Scope        CyclingScope                   `json:"scope"`
	State        CyclingState                   `json:"state"`
	Volume       CyclingVolume                  `json:"volume"`
	PowerAnchors CyclingPowerAnchors            `json:"powerAnchors"`
	Environment  CyclingEnvironmentContext      `json:"environment"`
	Load         CyclingLoad                    `json:"load"`
	Intensity    CyclingIntensity               `json:"intensity"`
	Durability   CyclingDurability              `json:"durability"`
	Anaerobic    CyclingAnaerobic               `json:"anaerobic"`
	Performance  CyclingPerformanceIntelligence `json:"performance"`
	Sessions     []CyclingSession               `json:"sessions,omitempty"`
	Warnings     []string                       `json:"warnings,omitempty"`
}

type CyclingPowerAnchors struct {
	FTP           int    `json:"ftp,omitempty"`
	CriticalPower int    `json:"criticalPower,omitempty"`
	WPrime        int    `json:"wPrime,omitempty"`
	PMax          int    `json:"pMax,omitempty"`
	RollingFTP    int    `json:"rollingFtp,omitempty"`
	Source        string `json:"source,omitempty"`
}

type CyclingEnvironmentContext struct {
	Samples              int     `json:"samples"`
	AverageTemp          float64 `json:"averageTemp,omitempty"`
	MaxTemp              float64 `json:"maxTemp,omitempty"`
	AverageWeatherTemp   float64 `json:"averageWeatherTemp,omitempty"`
	AverageFeelsLike     float64 `json:"averageFeelsLike,omitempty"`
	AverageWindSpeed     float64 `json:"averageWindSpeed,omitempty"`
	MaxWindGust          float64 `json:"maxWindGust,omitempty"`
	HeadwindPercent      float64 `json:"headwindPercent,omitempty"`
	TailwindPercent      float64 `json:"tailwindPercent,omitempty"`
	AverageAltitude      float64 `json:"averageAltitude,omitempty"`
	MaxAltitude          float64 `json:"maxAltitude,omitempty"`
	AverageGradient      float64 `json:"averageGradient,omitempty"`
	AverageYaw           float64 `json:"averageYaw,omitempty"`
	AverageStrainScore   float64 `json:"averageStrainScore,omitempty"`
	HotSessions          int     `json:"hotSessions,omitempty"`
	WindAffectedSessions int     `json:"windAffectedSessions,omitempty"`
	ClimbingSessions     int     `json:"climbingSessions,omitempty"`
	Source               string  `json:"source,omitempty"`
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
	Timezone          string `json:"timezone,omitempty"`
	TimezoneSource    string `json:"timezoneSource,omitempty"`
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
	TotalJoulesAboveFTP      int     `json:"totalJoulesAboveFtp"`
	MaxWBalDepletion         int     `json:"maxWbalDepletion"`
	WBalActivities           int     `json:"wbalActivities"`
	WPrimeCapacity           int     `json:"wPrimeCapacity,omitempty"`
	MaxWBalDepletionPercent  float64 `json:"maxWbalDepletionPercent,omitempty"`
	MeanWBalDepletionPercent float64 `json:"meanWbalDepletionPercent,omitempty"`
}

type CyclingSession struct {
	ID                 string     `json:"id,omitempty"`
	Date               string     `json:"date,omitempty"`
	Name               string     `json:"name,omitempty"`
	Type               string     `json:"type,omitempty"`
	MovingTimeSecs     int        `json:"movingTimeSecs"`
	MovingTimeMinutes  float64    `json:"movingTimeMinutes"`
	DistanceMeters     float64    `json:"distanceMeters"`
	DistanceKilometers float64    `json:"distanceKilometers"`
	TrainingLoad       int        `json:"trainingLoad"`
	Intensity          float64    `json:"intensity"`
	WeightedAvgPower   int        `json:"weightedAvgPower,omitempty"`
	AverageHeartRate   int        `json:"averageHeartRate,omitempty"`
	Decoupling         float64    `json:"decoupling,omitempty"`
	EfficiencyFactor   float64    `json:"efficiencyFactor,omitempty"`
	VariabilityIndex   float64    `json:"variabilityIndex,omitempty"`
	JoulesAboveFTP     int        `json:"joulesAboveFtp,omitempty"`
	MaxWBalDepletion   int        `json:"maxWbalDepletion,omitempty"`
	WPrime             int        `json:"wPrime,omitempty"`
	WBalDepletionPct   float64    `json:"wbalDepletionPct,omitempty"`
	CriticalPower      int        `json:"criticalPower,omitempty"`
	PMax               int        `json:"pMax,omitempty"`
	FTP                int        `json:"ftp,omitempty"`
	RollingFTP         int        `json:"rollingFtp,omitempty"`
	AverageTemp        float64    `json:"averageTemp,omitempty"`
	AverageFeelsLike   float64    `json:"averageFeelsLike,omitempty"`
	HeadwindPercent    float64    `json:"headwindPercent,omitempty"`
	StrainScore        float64    `json:"strainScore,omitempty"`
	ZoneTimes          []ZoneTime `json:"zoneTimes,omitempty"`
	HRZoneTimes        []int      `json:"hrZoneTimes,omitempty"`
	MaxHeartRate       int        `json:"maxHeartRate,omitempty"`
}

type CyclingPerformanceIntelligence struct {
	Repeatability CyclingRepeatability    `json:"repeatability"`
	Durability    CyclingDurabilitySignal `json:"durability"`
	NeuralDensity CyclingNeuralDensity    `json:"neuralDensity"`
	Efficiency    CyclingEfficiencySignal `json:"efficiency"`
}

type CyclingRepeatability struct {
	TotalWorkAboveFTP        int     `json:"totalWorkAboveFtp"`
	MaxWBalDepletion         int     `json:"maxWbalDepletion,omitempty"`
	MeanMaxWBalDepletion     float64 `json:"meanMaxWbalDepletion,omitempty"`
	MaxWBalDepletionPercent  float64 `json:"maxWbalDepletionPercent,omitempty"`
	MeanWBalDepletionPercent float64 `json:"meanWbalDepletionPercent,omitempty"`
	WPrimeCapacity           int     `json:"wPrimeCapacity,omitempty"`
	WorkAboveFTPPerHour      float64 `json:"workAboveFtpPerHour,omitempty"`
	SessionsWithWBalData     int     `json:"sessionsWithWbalData"`
	Pattern                  string  `json:"pattern,omitempty"`
	Classification           string  `json:"classification,omitempty"`
	ClassificationContext    string  `json:"classificationContext,omitempty"`
}

type CyclingDurabilitySignal struct {
	State                 string  `json:"state,omitempty"`
	MeanDecoupling        float64 `json:"meanDecoupling"`
	MaxDecoupling         float64 `json:"maxDecoupling"`
	HighDriftSessions     int     `json:"highDriftSessions"`
	LongEnduranceSessions int     `json:"longEnduranceSessions"`
	ISDMState             string  `json:"isdmState,omitempty"`
	ISDMScore             float64 `json:"isdmScore,omitempty"`
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

type CyclingEfficiencySignal struct {
	State                  string  `json:"state,omitempty"`
	MeanEfficiencyFactor   float64 `json:"meanEfficiencyFactor,omitempty"`
	MeanVariabilityIndex   float64 `json:"meanVariabilityIndex,omitempty"`
	SessionsWithEfficiency int     `json:"sessionsWithEfficiency"`
	Classification         string  `json:"classification,omitempty"`
}

type cyclingAnalysisAccumulator struct {
	analysis                CyclingAnalysis
	dailyLoads              map[string]int
	intensityWeightedSum    float64
	intensity               numericAccumulator
	decoupling              numericAccumulator
	efficiency              numericAccumulator
	variability             numericAccumulator
	wbalDepletion           numericAccumulator
	wbalDepletionPercent    numericAccumulator
	maxWbalDepletionPercent float64
	temp                    numericAccumulator
	weatherTemp             numericAccumulator
	feelsLike               numericAccumulator
	windSpeed               numericAccumulator
	headwind                numericAccumulator
	tailwind                numericAccumulator
	altitude                numericAccumulator
	gradient                numericAccumulator
	yaw                     numericAccumulator
	strain                  numericAccumulator
	environmentSamples      int
	maxTemp                 float64
	maxWindGust             float64
	maxAltitude             float64
	hotSessions             int
	windAffectedSessions    int
	climbingSessions        int
	maxDecoupling           float64
	longEndurance           int
	latestLoadDate          string
	latestAnchorDate        string
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
	accumulator.addPowerAnchors(activity)
	accumulator.addAnaerobic(activity)
	accumulator.addEnvironment(activity)
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

	if activity.WPrime > 0 && activity.MaxWbalDepletion > 0 {
		depletionPercent := round2(float64(activity.MaxWbalDepletion) * percentScale / float64(activity.WPrime))
		accumulator.wbalDepletionPercent.add(depletionPercent)

		if depletionPercent > accumulator.maxWbalDepletionPercent {
			accumulator.maxWbalDepletionPercent = depletionPercent
		}
	}
}

func (accumulator *cyclingAnalysisAccumulator) addPowerAnchors(activity *Activity) {
	if !activityHasPowerAnchors(activity) {
		return
	}

	date := activityDate(activity)
	if date != "" && date < accumulator.latestAnchorDate {
		return
	}

	accumulator.latestAnchorDate = date
	accumulator.analysis.PowerAnchors.FTP = activity.FTP
	accumulator.analysis.PowerAnchors.CriticalPower = activity.CriticalPower
	accumulator.analysis.PowerAnchors.WPrime = activity.WPrime
	accumulator.analysis.PowerAnchors.PMax = activity.PMax
	accumulator.analysis.PowerAnchors.RollingFTP = activity.RollingFTP
	accumulator.analysis.PowerAnchors.Source = "latest_activity_model_fields"
}

func activityHasPowerAnchors(activity *Activity) bool {
	return activity.FTP != 0 || activity.CriticalPower != 0 || activity.WPrime != 0 ||
		activity.PMax != 0 || activity.RollingFTP != 0
}

func (accumulator *cyclingAnalysisAccumulator) addEnvironment(activity *Activity) {
	if !activityHasEnvironment(activity) {
		return
	}

	accumulator.environmentSamples++
	accumulator.temp.addIfNonZero(activity.AverageTemp)
	accumulator.weatherTemp.addIfNonZero(activity.AverageWeatherTemp)
	accumulator.feelsLike.addIfNonZero(activity.AverageFeelsLike)
	accumulator.windSpeed.addIfNonZero(activity.AverageWindSpeed)
	accumulator.headwind.addIfNonZero(activity.HeadwindPercent)
	accumulator.tailwind.addIfNonZero(activity.TailwindPercent)
	accumulator.altitude.addIfNonZero(activity.AverageAltitude)
	accumulator.gradient.addIfNonZero(activity.AverageGradient)
	accumulator.yaw.addIfNonZero(activity.AverageYaw)
	accumulator.strain.addIfNonZero(activity.StrainScore)
	accumulator.maxTemp = maxFloat(accumulator.maxTemp, activity.MaxTemp)
	accumulator.maxWindGust = maxFloat(accumulator.maxWindGust, activity.AverageWindGust)
	accumulator.maxAltitude = maxFloat(accumulator.maxAltitude, activity.MaxAltitude)

	if activity.AverageTemp >= hotTempThreshold || activity.AverageFeelsLike >= hotFeelsLikeThreshold {
		accumulator.hotSessions++
	}

	if activity.AverageWindSpeed >= windSpeedThreshold || activity.HeadwindPercent >= headwindThreshold {
		accumulator.windAffectedSessions++
	}

	if activity.AverageGradient >= climbingGradientPct {
		accumulator.climbingSessions++
	}
}

func activityHasEnvironment(activity *Activity) bool {
	return activity.AverageTemp != 0 || activity.AverageWeatherTemp != 0 || activity.AverageFeelsLike != 0 ||
		activity.AverageWindSpeed != 0 || activity.HeadwindPercent != 0 || activity.AverageAltitude != 0 ||
		activity.AverageGradient != 0 || activity.StrainScore != 0
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

	// Skip activities that omit CTL/ATL (both zero). A later session without
	// fitness-model fields must not wipe a previously recorded form state.
	if activity.CTL == 0 && activity.ATL == 0 {
		if accumulator.latestLoadDate == "" {
			accumulator.latestLoadDate = date
		}
		return
	}

	accumulator.latestLoadDate = date
	accumulator.analysis.Load.LatestCTL = activity.CTL
	accumulator.analysis.Load.LatestATL = activity.ATL
	accumulator.analysis.Load.LatestForm = round2(activity.CTL - activity.ATL)
}

func (accumulator *cyclingAnalysisAccumulator) finish(options AnalysisOptions) CyclingAnalysis {
	accumulator.analysis.Scope.Timezone = options.Timezone
	accumulator.analysis.Scope.TimezoneSource = options.TimezoneSource
	accumulator.finishVolume()

	if accumulator.analysis.Scope.CyclingActivities == 0 {
		accumulator.analysis.Warnings = append(accumulator.analysis.Warnings, "no cycling activities found")

		return accumulator.analysis
	}

	accumulator.finishLoad(options)
	accumulator.finishIntensity()
	accumulator.finishDurability()
	accumulator.finishAnaerobic()
	accumulator.finishEnvironment()
	accumulator.finishState()
	accumulator.finishPerformance()

	return accumulator.analysis
}

func (accumulator *cyclingAnalysisAccumulator) finishAnaerobic() {
	accumulator.analysis.Anaerobic.WPrimeCapacity = accumulator.analysis.PowerAnchors.WPrime
	accumulator.analysis.Anaerobic.MaxWBalDepletionPercent = round2(accumulator.maxWbalDepletionPercent)
	accumulator.analysis.Anaerobic.MeanWBalDepletionPercent = round2(accumulator.wbalDepletionPercent.average())
}

func (accumulator *cyclingAnalysisAccumulator) finishEnvironment() {
	if accumulator.environmentSamples == 0 {
		return
	}

	accumulator.analysis.Environment = CyclingEnvironmentContext{
		Samples:              accumulator.environmentSamples,
		AverageTemp:          round2(accumulator.temp.average()),
		MaxTemp:              round2(accumulator.maxTemp),
		AverageWeatherTemp:   round2(accumulator.weatherTemp.average()),
		AverageFeelsLike:     round2(accumulator.feelsLike.average()),
		AverageWindSpeed:     round2(accumulator.windSpeed.average()),
		MaxWindGust:          round2(accumulator.maxWindGust),
		HeadwindPercent:      round2(accumulator.headwind.average()),
		TailwindPercent:      round2(accumulator.tailwind.average()),
		AverageAltitude:      round2(accumulator.altitude.average()),
		MaxAltitude:          round2(accumulator.maxAltitude),
		AverageGradient:      round2(accumulator.gradient.average()),
		AverageYaw:           round2(accumulator.yaw.average()),
		AverageStrainScore:   round2(accumulator.strain.average()),
		HotSessions:          accumulator.hotSessions,
		WindAffectedSessions: accumulator.windAffectedSessions,
		ClimbingSessions:     accumulator.climbingSessions,
		Source:               "activity_environment_fields",
	}
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
		accumulator.analysis.State.StateLabel = stateLabelLoadPressure
		accumulator.analysis.State.OperationalState = stateOperationalRecovery
		accumulator.analysis.State.LoadRecoveryState = stateLoadPressure
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
		Efficiency:    accumulator.efficiencySignal(),
	}
}

func (accumulator *cyclingAnalysisAccumulator) repeatabilitySignal() CyclingRepeatability {
	return CyclingRepeatability{
		TotalWorkAboveFTP:        accumulator.analysis.Anaerobic.TotalJoulesAboveFTP,
		MaxWBalDepletion:         accumulator.analysis.Anaerobic.MaxWBalDepletion,
		MeanMaxWBalDepletion:     round2(accumulator.wbalDepletion.average()),
		MaxWBalDepletionPercent:  accumulator.analysis.Anaerobic.MaxWBalDepletionPercent,
		MeanWBalDepletionPercent: accumulator.analysis.Anaerobic.MeanWBalDepletionPercent,
		WPrimeCapacity:           accumulator.analysis.Anaerobic.WPrimeCapacity,
		WorkAboveFTPPerHour:      accumulator.workAboveFTPPerHour(),
		SessionsWithWBalData:     accumulator.analysis.Anaerobic.WBalActivities,
		Pattern:                  wPrimePattern(accumulator.maxWbalDepletionPercent),
		Classification:           "informational",
		ClassificationContext:    "local_activity_fields",
	}
}

func (accumulator *cyclingAnalysisAccumulator) durabilitySignal() CyclingDurabilitySignal {
	return CyclingDurabilitySignal{
		State:                 durabilityState(accumulator.maxDecoupling),
		MeanDecoupling:        accumulator.analysis.Durability.AverageDecoupling,
		MaxDecoupling:         round2(accumulator.maxDecoupling),
		HighDriftSessions:     accumulator.analysis.Durability.HighDecouplingActivities,
		LongEnduranceSessions: accumulator.longEndurance,
		ISDMState:             isdmState(accumulator.maxDecoupling, accumulator.longEndurance),
		ISDMScore:             isdmScore(accumulator.maxDecoupling, accumulator.analysis.Durability.HighDecouplingActivities),
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

func (accumulator *cyclingAnalysisAccumulator) efficiencySignal() CyclingEfficiencySignal {
	meanEfficiency := accumulator.analysis.Durability.AverageEfficiencyFactor
	meanVariability := accumulator.analysis.Durability.AverageVariabilityIndex

	return CyclingEfficiencySignal{
		State:                  efficiencyState(meanEfficiency, meanVariability),
		MeanEfficiencyFactor:   meanEfficiency,
		MeanVariabilityIndex:   meanVariability,
		SessionsWithEfficiency: accumulator.efficiency.count,
		Classification:         "local_efficiency_heuristic",
	}
}

func (accumulator *cyclingAnalysisAccumulator) workAboveFTPPerHour() float64 {
	if accumulator.analysis.Volume.MovingTimeSecs == 0 {
		return 0
	}

	movingHours := float64(accumulator.analysis.Volume.MovingTimeSecs) / secondsPerHour

	return round2(float64(accumulator.analysis.Anaerobic.TotalJoulesAboveFTP) / movingHours)
}

func wPrimePattern(maxDepletionPercent float64) string {
	if maxDepletionPercent == 0 {
		return ""
	}

	if maxDepletionPercent >= wPrimeRepeatablePct {
		return "repeatable_w_prime_depletion"
	}

	if maxDepletionPercent >= wPrimeModeratePct {
		return "moderate_w_prime_depletion"
	}

	return "low_w_prime_depletion"
}

func isdmState(maxDecoupling float64, longEnduranceSessions int) string {
	if maxDecoupling == 0 && longEnduranceSessions == 0 {
		return unknownState
	}

	if maxDecoupling >= highDecouplingPct {
		return "durability_limiter"
	}

	if longEnduranceSessions > 0 {
		return "durability_supported"
	}

	return "durability_unproven"
}

func isdmScore(maxDecoupling float64, highDriftSessions int) float64 {
	if maxDecoupling == 0 && highDriftSessions == 0 {
		return 0
	}

	score := isdmBaseScore - maxDecoupling*isdmDecouplingPenalty -
		float64(highDriftSessions*isdmHighDriftPenalty)

	if score < 0 {
		return 0
	}

	return round2(score)
}

func efficiencyState(meanEfficiency, meanVariability float64) string {
	if meanEfficiency == 0 {
		return unknownState
	}

	if meanEfficiency < lowEfficiencyThreshold {
		return "low_efficiency"
	}

	if meanEfficiency >= highEfficiencyThreshold && meanVariability >= lowEfficiencyThreshold {
		return "efficient_variable"
	}

	if meanEfficiency >= highEfficiencyThreshold {
		return "efficient_stable"
	}

	return "steady"
}

func durabilityState(maxDecoupling float64) string {
	if maxDecoupling == 0 {
		return unknownState
	}

	if maxDecoupling >= highDecouplingPct {
		return stateDurabilityWatch
	}

	return "stable"
}

func (accumulator *numericAccumulator) add(value float64) {
	accumulator.sum += value
	accumulator.count++
}

func (accumulator *numericAccumulator) addIfNonZero(value float64) {
	if value == 0 {
		return
	}

	accumulator.add(value)
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
	wbalDepletionPct := float64(0)
	if activity.WPrime > 0 && activity.MaxWbalDepletion > 0 {
		wbalDepletionPct = round2(float64(activity.MaxWbalDepletion) * percentScale / float64(activity.WPrime))
	}

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
		WPrime:             activity.WPrime,
		WBalDepletionPct:   wbalDepletionPct,
		CriticalPower:      activity.CriticalPower,
		PMax:               activity.PMax,
		FTP:                activity.FTP,
		RollingFTP:         activity.RollingFTP,
		AverageTemp:        activity.AverageTemp,
		AverageFeelsLike:   activity.AverageFeelsLike,
		HeadwindPercent:    activity.HeadwindPercent,
		StrainScore:        activity.StrainScore,
		ZoneTimes:          activity.ZoneTimes,
		HRZoneTimes:        activity.HRZoneTimes,
		MaxHeartRate:       activity.MaxHeartRate,
	}
}

func maxFloat(current, candidate float64) float64 {
	if candidate > current {
		return candidate
	}

	return current
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
	if len(loads) == 0 {
		return nil
	}

	dates := make([]string, 0, len(loads))
	for date := range loads {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	startDate, startErr := time.Parse(analysisDateLayout, dates[0])
	endDate, endErr := time.Parse(analysisDateLayout, dates[len(dates)-1])
	// Fall back to sparse ride-only series when keys are not calendar dates.
	if startErr != nil || endErr != nil || endDate.Before(startDate) {
		days := make([]DailyTrainingLoad, 0, len(dates))
		for _, date := range dates {
			days = append(days, DailyTrainingLoad{Date: date, Load: loads[date]})
		}
		return days
	}

	// Expand min→max inclusive so rest days contribute zero load for monotony,
	// strain, and acute/chronic windows (Foster-style consecutive days).
	var days []DailyTrainingLoad
	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		key := current.Format(analysisDateLayout)
		days = append(days, DailyTrainingLoad{Date: key, Load: loads[key]})
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
