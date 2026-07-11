package icu

import "time"

const (
	microcycleCommand              = "analysis microcycle"
	microcycleStatusAvailable      = "available"
	microcycleStatusMissing        = "missing"
	microcycleStatusPartial        = "partial"
	microcycleStatusIncomplete     = "incomplete"
	microcycleStatusExcluded       = "excluded"
	microcycleStatusDataLimited    = "data_limited"
	microcycleStatusHigh           = "high"
	microcycleLookback7Days        = 7
	microcycleLookback28Days       = 28
	microcycleLookback90Days       = 90
	microcycleZ4IntensityThreshold = 0.85
	microcycleZ4RiskThreshold      = 2
	microcycleLongRideSeconds      = 7200
	microcycleDateLayout           = "2006-01-02"
	microcyclePlanCategoryWorkout  = "WORKOUT"
	microcycleConfidenceHigh       = "high"
	microcycleConfidenceMedium     = "medium"
	microcycleConfidenceLow        = "low"
	microcycleLowPenalty           = 4
	microcycleHighZonePercent      = 20
	microcyclePyramidalLowPercent  = 60
	microcycleLoadAboveRatio       = 1.15
	microcycleLoadBelowRatio       = 0.85
)

type MicrocycleOptions struct {
	StartDate        string    `json:"startDate,omitempty"`
	EndDate          string    `json:"endDate,omitempty"`
	Timezone         string    `json:"timezone,omitempty"`
	TimezoneSource   string    `json:"timezoneSource,omitempty"`
	Now              time.Time `json:"-"`
	PlanIncluded     bool      `json:"planIncluded"`
	WellnessIncluded bool      `json:"wellnessIncluded"`
	SportType        string    `json:"sportType,omitempty"`
	Full             bool      `json:"full,omitempty"`
}

type MicrocycleAnalysis struct {
	Command            string                      `json:"command"`
	Status             string                      `json:"status"`
	Microcycle         MicrocycleScope             `json:"microcycle"`
	Sources            MicrocycleSources           `json:"sources"`
	DataQuality        MicrocycleDataQuality       `json:"dataQuality"`
	PlannedVsActual    MicrocyclePlannedVsActual   `json:"plannedVsActual"`
	Load               MicrocycleLoad              `json:"load"`
	Intensity          MicrocycleIntensity         `json:"intensity"`
	Wellness           MicrocycleWellness          `json:"wellness"`
	FatigueDurability  MicrocycleFatigueDurability `json:"fatigueDurability"`
	Classification     MicrocycleAdaptationSignal  `json:"classification"`
	AdaptationSignal   MicrocycleAdaptationSignal  `json:"adaptationSignal"`
	Risks              []MicrocycleRisk            `json:"risks,omitempty"`
	Evidence           []string                    `json:"evidence,omitempty"`
	OpenQuestions      []string                    `json:"openQuestions,omitempty"`
	Confidence         string                      `json:"confidence"`
	SideEffects        MicrocycleSideEffects       `json:"sideEffects"`
	HumanReadableNotes []string                    `json:"humanReadableNotes,omitempty"`
}

type MicrocycleScope struct {
	StartDate      string `json:"start"`
	EndDate        string `json:"end"`
	Timezone       string `json:"timezone"`
	TimezoneSource string `json:"timezoneSource,omitempty"`
	IsPartial      bool   `json:"isPartial"`
	ElapsedDays    int    `json:"elapsedDays"`
	RemainingDays  int    `json:"remainingDays"`
	SportType      string `json:"sportType,omitempty"`
}

type MicrocycleSources struct {
	Activities string `json:"activities"`
	Plan       string `json:"plan"`
	Wellness   string `json:"wellness"`
	Sport      string `json:"sport"`
}

type MicrocycleDataQuality struct {
	Activities string   `json:"activities"`
	Plan       string   `json:"plan"`
	Power      string   `json:"power"`
	HeartRate  string   `json:"heartRate"`
	Wellness   string   `json:"wellness"`
	FTP        string   `json:"ftp"`
	Zones      string   `json:"zones"`
	Timezone   string   `json:"timezone"`
	Warnings   []string `json:"warnings,omitempty"`
}

type MicrocyclePlannedVsActual struct {
	Available         bool    `json:"available"`
	PlannedSessions   int     `json:"plannedSessions"`
	CompletedSessions int     `json:"completedSessions"`
	MissedSessions    int     `json:"missedSessions"`
	ExtraSessions     int     `json:"extraSessions"`
	RemainingSessions int     `json:"remainingSessions"`
	PlannedLoad       int     `json:"plannedLoad"`
	ActualLoad        int     `json:"actualLoad"`
	PlannedDuration   int     `json:"plannedDurationSeconds"`
	ActualDuration    int     `json:"actualDurationSeconds"`
	LoadCompliance    float64 `json:"loadCompliance,omitempty"`
	VolumeCompliance  float64 `json:"volumeCompliance,omitempty"`
}

type MicrocycleLoad struct {
	DurationSeconds int                      `json:"durationSeconds"`
	DistanceMeters  float64                  `json:"distanceMeters"`
	TSS             int                      `json:"tss"`
	ActivityCount   int                      `json:"activityCount"`
	IntensityFactor float64                  `json:"intensityFactor,omitempty"`
	CTL             float64                  `json:"ctl,omitempty"`
	ATL             float64                  `json:"atl,omitempty"`
	TSB             float64                  `json:"tsb,omitempty"`
	ACWR            float64                  `json:"acwr,omitempty"`
	Monotony        float64                  `json:"monotony,omitempty"`
	Strain          float64                  `json:"strain,omitempty"`
	Comparison      MicrocycleLoadComparison `json:"comparison"`
	Daily           []DailyTrainingLoad      `json:"daily,omitempty"`
	Sessions        []CyclingSession         `json:"sessions,omitempty"`
}

type MicrocycleLoadComparison struct {
	Previous7Days  string `json:"previous7Days,omitempty"`
	Previous28Days string `json:"previous28Days,omitempty"`
	Previous90Days string `json:"previous90Days,omitempty"`
}

type MicrocycleIntensity struct {
	ZoneSeconds      map[string]int     `json:"zoneSeconds,omitempty"`
	ZonePercent      map[string]float64 `json:"zonePercent,omitempty"`
	HRZoneSeconds    []int              `json:"hrZoneSeconds,omitempty"`
	Z4PlusSessions   int                `json:"z4PlusSessions"`
	IntensityDensity string             `json:"intensityDensity"`
	Distribution     string             `json:"distribution,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
}

type MicrocycleWellness struct {
	Available        bool             `json:"available"`
	ConfidenceImpact string           `json:"confidenceImpact,omitempty"`
	State            PhysiologyState  `json:"state,omitempty"`
	PositiveSignals  []string         `json:"positiveSignals,omitempty"`
	NegativeSignals  []string         `json:"negativeSignals,omitempty"`
	MissingSignals   []string         `json:"missingSignals,omitempty"`
	Coverage         WellnessCoverage `json:"coverage,omitempty"`
}

type MicrocycleFatigueDurability struct {
	State               string   `json:"state"`
	LongRides           int      `json:"longRides"`
	HighDecouplingRides int      `json:"highDecouplingRides"`
	AverageDecoupling   float64  `json:"averageDecoupling,omitempty"`
	LatestForm          float64  `json:"latestForm,omitempty"`
	Evidence            []string `json:"evidence,omitempty"`
}

type MicrocycleAdaptationSignal struct {
	Value              string   `json:"value"`
	Rationale          []string `json:"rationale,omitempty"`
	SupportingEvidence []string `json:"supportingEvidence,omitempty"`
	Confidence         string   `json:"confidence"`
	MainPositiveSignal string   `json:"mainPositiveSignal,omitempty"`
	MainRisk           string   `json:"mainRisk,omitempty"`
}

type MicrocycleRisk struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

type MicrocycleSideEffects struct {
	MutatedPlan        bool `json:"mutatedPlan"`
	MutatedConfig      bool `json:"mutatedConfig"`
	SyncedExternalData bool `json:"syncedExternalData"`
}

func AnalyzeMicrocycle(
	activities []Activity,
	events []Event,
	wellness *WellnessAnalysis,
	settings *SportSettings,
	options *MicrocycleOptions,
) MicrocycleAnalysis {
	// Work on a local copy so caller-owned options are never mutated.
	local := MicrocycleOptions{}
	if options != nil {
		local = *options
	}
	if local.Now.IsZero() {
		local.Now = time.Now()
	}
	if local.SportType == "" {
		local.SportType = "Ride"
	}
	options = &local

	selectedActivities := filterActivitiesByDate(activities, options.StartDate, options.EndDate)
	cycling := AnalyzeCyclingActivities(selectedActivities, AnalysisOptions{
		StartDate: options.StartDate,
		EndDate:   options.EndDate,
	})

	analysis := MicrocycleAnalysis{
		Command:           microcycleCommand,
		Status:            "success",
		Microcycle:        microcycleScope(options),
		Sources:           microcycleSources(options),
		DataQuality:       microcycleDataQuality(selectedActivities, events, wellness, settings, options),
		PlannedVsActual:   microcyclePlannedVsActual(selectedActivities, events, options),
		Load:              microcycleLoad(&cycling, activities, options),
		Intensity:         microcycleIntensity(&cycling),
		Wellness:          microcycleWellness(wellness, options),
		FatigueDurability: microcycleFatigueDurability(&cycling),
		SideEffects: MicrocycleSideEffects{
			MutatedPlan:        false,
			MutatedConfig:      false,
			SyncedExternalData: false,
		},
	}

	analysis.Risks = microcycleRisks(&analysis)
	analysis.Evidence = microcycleEvidence(&analysis)
	analysis.OpenQuestions = microcycleOpenQuestions(&analysis)
	analysis.Confidence = microcycleConfidence(&analysis)
	analysis.AdaptationSignal = microcycleAdaptationSignal(&analysis)
	analysis.Classification = analysis.AdaptationSignal
	analysis.HumanReadableNotes = microcycleHumanNotes(&analysis)

	return analysis
}

func microcycleScope(options *MicrocycleOptions) MicrocycleScope {
	start, startErr := time.Parse(microcycleDateLayout, options.StartDate)
	end, endErr := time.Parse(microcycleDateLayout, options.EndDate)
	totalDays := 0
	elapsedDays := 0

	if startErr == nil && endErr == nil && !end.Before(start) {
		totalDays = int(end.Sub(start).Hours()/hoursPerDay) + 1
		today := options.Now.Format(microcycleDateLayout)

		switch {
		case today < options.StartDate:
			elapsedDays = 0
		case today > options.EndDate:
			elapsedDays = totalDays
		default:
			nowDay, err := time.Parse(microcycleDateLayout, today)
			if err == nil {
				elapsedDays = int(nowDay.Sub(start).Hours()/hoursPerDay) + 1
			}
		}
	}

	remainingDays := totalDays - elapsedDays
	if remainingDays < 0 {
		remainingDays = 0
	}

	return MicrocycleScope{
		StartDate:      options.StartDate,
		EndDate:        options.EndDate,
		Timezone:       options.Timezone,
		TimezoneSource: options.TimezoneSource,
		IsPartial:      elapsedDays < totalDays,
		ElapsedDays:    elapsedDays,
		RemainingDays:  remainingDays,
		SportType:      options.SportType,
	}
}

func microcycleSources(options *MicrocycleOptions) MicrocycleSources {
	sources := MicrocycleSources{
		Activities: "api.activities",
		Sport:      "api.sport-settings",
	}
	if options.PlanIncluded {
		sources.Plan = "api.events"
	} else {
		sources.Plan = "excluded_by_flag"
	}
	if options.WellnessIncluded {
		sources.Wellness = "api.wellness"
	} else {
		sources.Wellness = "excluded_by_flag"
	}

	return sources
}

func microcycleDataQuality(
	activities []Activity,
	events []Event,
	wellness *WellnessAnalysis,
	settings *SportSettings,
	options *MicrocycleOptions,
) MicrocycleDataQuality {
	quality := MicrocycleDataQuality{
		Activities: availability(len(activities) > 0),
		Plan:       microcycleStatusExcluded,
		Power:      powerAvailability(activities),
		HeartRate:  heartRateAvailability(activities),
		Wellness:   microcycleStatusExcluded,
		FTP:        ftpAvailability(activities, settings),
		Zones:      zonesAvailability(activities, settings),
		Timezone:   "configured",
	}

	if options.PlanIncluded {
		quality.Plan = availability(countWorkoutEvents(events, options) > 0)
	}
	if options.WellnessIncluded {
		quality.Wellness = wellnessAvailability(wellness)
	}
	if options.TimezoneSource == "system" {
		quality.Timezone = "system_fallback"
		quality.Warnings = append(quality.Warnings, "missing athlete timezone; using system timezone")
	}
	if quality.Activities == microcycleStatusMissing {
		quality.Warnings = append(quality.Warnings, "no completed activities found for this microcycle")
	}
	if quality.Plan == microcycleStatusMissing {
		quality.Warnings = append(quality.Warnings, "no athlete plan found; planned-vs-actual analysis unavailable")
	}
	if quality.Wellness == microcycleStatusMissing || quality.Wellness == microcycleStatusIncomplete {
		quality.Warnings = append(quality.Warnings, "wellness data incomplete; readiness confidence reduced")
	}
	if quality.FTP == microcycleStatusMissing {
		quality.Warnings = append(quality.Warnings, "FTP configuration is missing")
	}
	if quality.Zones == microcycleStatusMissing {
		quality.Warnings = append(quality.Warnings, "zone configuration is missing; zone-based intensity confidence reduced")
	}

	return quality
}

func microcyclePlannedVsActual(
	activities []Activity,
	events []Event,
	options *MicrocycleOptions,
) MicrocyclePlannedVsActual {
	result := MicrocyclePlannedVsActual{
		Available:         options.PlanIncluded,
		CompletedSessions: len(activities),
		ActualLoad:        activityLoad(activities),
		ActualDuration:    activityDuration(activities),
	}
	if !options.PlanIncluded {
		return result
	}

	workouts := plannedWorkoutEvents(events, options)
	result.PlannedSessions = len(workouts)
	result.PlannedLoad = eventLoad(workouts)
	result.PlannedDuration = eventDuration(workouts)
	result.RemainingSessions = remainingWorkoutEvents(workouts, options)
	result.MissedSessions = missedWorkoutEvents(workouts, activities, options)

	matched := result.PlannedSessions - result.MissedSessions - result.RemainingSessions
	if matched < 0 {
		matched = 0
	}
	result.ExtraSessions = result.CompletedSessions - matched
	if result.ExtraSessions < 0 {
		result.ExtraSessions = 0
	}
	if result.PlannedLoad > 0 {
		result.LoadCompliance = round2(float64(result.ActualLoad) * percentScale / float64(result.PlannedLoad))
	}
	if result.PlannedDuration > 0 {
		result.VolumeCompliance = round2(float64(result.ActualDuration) * percentScale / float64(result.PlannedDuration))
	}

	return result
}

func microcycleLoad(cycling *CyclingAnalysis, allActivities []Activity, options *MicrocycleOptions) MicrocycleLoad {
	return MicrocycleLoad{
		DurationSeconds: cycling.Volume.MovingTimeSecs,
		DistanceMeters:  cycling.Volume.DistanceMeters,
		TSS:             cycling.Load.Total,
		ActivityCount:   cycling.Scope.CyclingActivities,
		IntensityFactor: cycling.Intensity.WeightedAverageIntensity,
		CTL:             cycling.Load.LatestCTL,
		ATL:             cycling.Load.LatestATL,
		TSB:             cycling.Load.LatestForm,
		ACWR:            cycling.Load.AcuteChronicWorkRatio,
		Monotony:        cycling.Load.Monotony,
		Strain:          cycling.Load.Strain,
		Comparison:      microcycleLoadComparison(cycling.Load.Total, allActivities, options),
		Daily:           cycling.Load.Daily,
		Sessions:        cycling.Sessions,
	}
}

func microcycleLoadComparison(currentLoad int, activities []Activity, options *MicrocycleOptions) MicrocycleLoadComparison {
	return MicrocycleLoadComparison{
		Previous7Days:  compareLoad(currentLoad, previousLoad(activities, options.StartDate, microcycleLookback7Days)),
		Previous28Days: compareLoad(currentLoad, previousLoad(activities, options.StartDate, microcycleLookback28Days)),
		Previous90Days: compareLoad(currentLoad, previousLoad(activities, options.StartDate, microcycleLookback90Days)),
	}
}

func microcycleIntensity(cycling *CyclingAnalysis) MicrocycleIntensity {
	z4 := countZ4PlusSessions(cycling.Sessions)
	intensity := MicrocycleIntensity{
		ZoneSeconds:      cycling.Intensity.ZoneSeconds,
		ZonePercent:      cycling.Intensity.ZonePercent,
		HRZoneSeconds:    cycling.Intensity.HRZoneSeconds,
		Z4PlusSessions:   z4,
		IntensityDensity: "acceptable",
		Distribution:     intensityDistribution(cycling.Intensity.ZonePercent),
	}
	if z4 > microcycleZ4RiskThreshold {
		intensity.IntensityDensity = microcycleStatusHigh
		intensity.Warnings = append(intensity.Warnings, "more than 2 Z4+ sessions detected")
	}

	return intensity
}

func microcycleWellness(wellness *WellnessAnalysis, options *MicrocycleOptions) MicrocycleWellness {
	if !options.WellnessIncluded || wellness == nil {
		return MicrocycleWellness{
			Available:        false,
			ConfidenceImpact: "reduced",
			MissingSignals:   []string{"wellness"},
		}
	}

	result := MicrocycleWellness{
		Available:        wellness.Scope.Records > 0,
		ConfidenceImpact: "none",
		State:            wellness.State,
		Coverage:         wellness.Coverage,
	}
	if !result.Available || wellness.State.Confidence == microcycleConfidenceLow {
		result.ConfidenceImpact = "reduced"
	}
	if wellness.HRV.Samples > 0 && wellness.HRV.Ratio >= 1 {
		result.PositiveSignals = append(result.PositiveSignals, "hrv_at_or_above_mean")
	}
	if wellness.RestingHR.Samples > 0 && wellness.RestingHR.Delta <= 0 {
		result.PositiveSignals = append(result.PositiveSignals, "resting_hr_not_elevated")
	}
	if wellness.Sleep.Samples > 0 && wellness.Sleep.Latest >= sleepWatchScore {
		result.PositiveSignals = append(result.PositiveSignals, "sleep_score_ok")
	}
	if wellness.HRV.Samples == 0 {
		result.MissingSignals = append(result.MissingSignals, "hrv")
	}
	if wellness.RestingHR.Samples == 0 {
		result.MissingSignals = append(result.MissingSignals, "resting_hr")
	}
	if wellness.Sleep.Samples == 0 {
		result.MissingSignals = append(result.MissingSignals, "sleep")
	}
	result.NegativeSignals = append(result.NegativeSignals, wellness.State.Reasons...)

	return result
}

func microcycleFatigueDurability(cycling *CyclingAnalysis) MicrocycleFatigueDurability {
	state := microcycleStatusDataLimited
	var evidence []string
	if cycling.Scope.CyclingActivities > 0 {
		state = "productive_or_absorbing"
		evidence = append(evidence, "completed cycling activities available")
	}
	if cycling.Durability.HighDecouplingActivities > 0 {
		state = "fatigue_watch"
		evidence = append(evidence, "high decoupling activities detected")
	}
	if cycling.Load.LatestForm <= negativeFormWatchThreshold && cycling.Load.LatestForm != 0 {
		state = stateLoadPressure
		evidence = append(evidence, "latest form is meaningfully negative")
	}

	return MicrocycleFatigueDurability{
		State:               state,
		LongRides:           countLongRides(cycling.Sessions),
		HighDecouplingRides: cycling.Durability.HighDecouplingActivities,
		AverageDecoupling:   cycling.Durability.AverageDecoupling,
		LatestForm:          cycling.Load.LatestForm,
		Evidence:            evidence,
	}
}

func microcycleRisks(analysis *MicrocycleAnalysis) []MicrocycleRisk {
	var risks []MicrocycleRisk
	if analysis.Intensity.Z4PlusSessions > microcycleZ4RiskThreshold {
		risks = append(risks, MicrocycleRisk{
			Type:     "intensity_density",
			Severity: "medium",
			Message:  "More than 2 Z4+ sessions in the microcycle is a potential risk.",
			Evidence: "z4PlusSessions > 2",
		})
	}
	if analysis.FatigueDurability.State == stateLoadPressure {
		risks = append(risks, MicrocycleRisk{
			Type:     "load_pressure",
			Severity: "medium",
			Message:  "Latest form indicates elevated acute load relative to chronic load.",
			Evidence: "latestForm <= -10",
		})
	}
	if analysis.DataQuality.Activities == microcycleStatusMissing {
		risks = append(risks, MicrocycleRisk{
			Type:     microcycleStatusDataLimited,
			Severity: "low",
			Message:  "No completed activities are available for this microcycle.",
			Evidence: "activity_count = 0",
		})
	}

	return risks
}

func microcycleEvidence(analysis *MicrocycleAnalysis) []string {
	evidence := []string{
		"activities=" + stringInt(analysis.Load.ActivityCount),
		"actual_load=" + stringInt(analysis.Load.TSS),
		"z4_plus_sessions=" + stringInt(analysis.Intensity.Z4PlusSessions),
	}
	if analysis.PlannedVsActual.Available {
		evidence = append(evidence, "planned_sessions="+stringInt(analysis.PlannedVsActual.PlannedSessions))
	}
	if analysis.Wellness.Available {
		evidence = append(evidence, "wellness_state="+analysis.Wellness.State.State)
	}

	return evidence
}

func microcycleOpenQuestions(analysis *MicrocycleAnalysis) []string {
	var questions []string
	if analysis.DataQuality.Plan == microcycleStatusMissing || analysis.DataQuality.Plan == microcycleStatusExcluded {
		questions = append(questions, "planned workouts unavailable")
	}
	if analysis.DataQuality.Wellness == microcycleStatusMissing || analysis.DataQuality.Wellness == microcycleStatusExcluded || analysis.DataQuality.Wellness == microcycleStatusIncomplete {
		questions = append(questions, "wellness/readiness data incomplete")
	}
	if analysis.DataQuality.Zones == microcycleStatusMissing {
		questions = append(questions, "sport zone settings unavailable")
	}
	if analysis.DataQuality.Power == microcycleStatusMissing {
		questions = append(questions, "power data unavailable")
	}
	if analysis.DataQuality.HeartRate == microcycleStatusMissing {
		questions = append(questions, "heart-rate data unavailable")
	}

	return questions
}

func microcycleConfidence(analysis *MicrocycleAnalysis) string {
	penalty := 0
	for _, status := range []string{
		analysis.DataQuality.Activities,
		analysis.DataQuality.Plan,
		analysis.DataQuality.Power,
		analysis.DataQuality.HeartRate,
		analysis.DataQuality.Wellness,
		analysis.DataQuality.FTP,
		analysis.DataQuality.Zones,
	} {
		switch status {
		case microcycleStatusMissing:
			penalty += 2
		case microcycleStatusExcluded:
			penalty += 2
		case microcycleStatusPartial, microcycleStatusIncomplete:
			penalty++
		}
	}

	switch {
	case penalty >= microcycleLowPenalty:
		return microcycleConfidenceLow
	case penalty >= 2:
		return microcycleConfidenceMedium
	default:
		return microcycleConfidenceHigh
	}
}

func microcycleAdaptationSignal(analysis *MicrocycleAnalysis) MicrocycleAdaptationSignal {
	signal := MicrocycleAdaptationSignal{
		Value:      "experimental_uncertain",
		Confidence: analysis.Confidence,
		Rationale:  []string{"experimental local diagnostic classification"},
	}

	switch {
	case analysis.Load.ActivityCount == 0:
		signal.Value = microcycleStatusDataLimited
		signal.MainRisk = "no completed activity data"
	case analysis.FatigueDurability.State == stateLoadPressure || analysis.Intensity.IntensityDensity == microcycleStatusHigh:
		signal.Value = "overloaded"
		signal.MainRisk = "load or intensity density is elevated"
	case analysis.Load.TSS == 0:
		signal.Value = "underloaded"
		signal.MainRisk = "no training load recorded"
	case analysis.Load.TSS > 0 && analysis.Intensity.IntensityDensity == "acceptable":
		signal.Value = "on_track"
		signal.MainPositiveSignal = "training load recorded with acceptable intensity density"
	}
	if analysis.Confidence == microcycleConfidenceLow && signal.Value != microcycleStatusDataLimited {
		signal.Value = microcycleStatusDataLimited
	}

	signal.SupportingEvidence = append(signal.SupportingEvidence, analysis.Evidence...)
	if signal.MainPositiveSignal == "" && analysis.Load.TSS > 0 {
		signal.MainPositiveSignal = "completed training load is available"
	}
	if signal.MainRisk == "" && len(analysis.Risks) > 0 {
		signal.MainRisk = analysis.Risks[0].Message
	}
	if signal.MainRisk == "" {
		signal.MainRisk = "no major risk detected from available data"
	}

	return signal
}

func microcycleHumanNotes(_ *MicrocycleAnalysis) []string {
	return []string{
		"No plan changes were made.",
		"This diagnostic is suitable as structured input for an LLM or weekly report layer when confidence is not low.",
	}
}

func filterActivitiesByDate(activities []Activity, start, end string) []Activity {
	var filtered []Activity
	for index := range activities {
		date := activityDate(&activities[index])
		if dateWithinInclusive(date, start, end) {
			filtered = append(filtered, activities[index])
		}
	}

	return filtered
}

func dateWithinInclusive(date, start, end string) bool {
	if date == "" {
		return false
	}
	if start != "" && date < start {
		return false
	}
	if end != "" && date > end {
		return false
	}

	return true
}

func availability(ok bool) string {
	if ok {
		return microcycleStatusAvailable
	}

	return microcycleStatusMissing
}

func powerAvailability(activities []Activity) string {
	if len(activities) == 0 {
		return microcycleStatusMissing
	}
	withPower := 0
	for index := range activities {
		if activities[index].WeightedAvgPower > 0 || activities[index].AveragePower > 0 || len(activities[index].ZoneTimes) > 0 {
			withPower++
		}
	}

	return partialAvailability(withPower, len(activities))
}

func heartRateAvailability(activities []Activity) string {
	if len(activities) == 0 {
		return microcycleStatusMissing
	}
	withHR := 0
	for index := range activities {
		if activities[index].AverageHeartRate > 0 || activities[index].MaxHeartRate > 0 || len(activities[index].HRZoneTimes) > 0 || activities[index].HasHeartRate {
			withHR++
		}
	}

	return partialAvailability(withHR, len(activities))
}

func partialAvailability(samples, total int) string {
	switch {
	case samples == 0:
		return microcycleStatusMissing
	case samples == total:
		return microcycleStatusAvailable
	default:
		return microcycleStatusPartial
	}
}

func wellnessAvailability(wellness *WellnessAnalysis) string {
	if wellness == nil || wellness.Scope.Records == 0 {
		return microcycleStatusMissing
	}
	if wellness.State.Confidence == microcycleConfidenceLow {
		return microcycleStatusIncomplete
	}

	return microcycleStatusAvailable
}

func ftpAvailability(activities []Activity, settings *SportSettings) string {
	if settings != nil && settings.FTP > 0 {
		return microcycleStatusAvailable
	}
	for index := range activities {
		if activities[index].FTP > 0 || activities[index].FTPWatts > 0 || activities[index].RollingFTP > 0 {
			return microcycleStatusAvailable
		}
	}

	return microcycleStatusMissing
}

func zonesAvailability(_ []Activity, settings *SportSettings) string {
	if settings != nil && (len(settings.PowerZones) > 0 || len(settings.HRZones) > 0) {
		return microcycleStatusAvailable
	}

	return microcycleStatusMissing
}

func countWorkoutEvents(events []Event, options *MicrocycleOptions) int {
	return len(plannedWorkoutEvents(events, options))
}

func plannedWorkoutEvents(events []Event, options *MicrocycleOptions) []Event {
	var workouts []Event
	for index := range events {
		event := events[index]
		if normalizedCategory(event.Category) != microcyclePlanCategoryWorkout {
			continue
		}
		if dateWithinInclusive(eventDate(&event), options.StartDate, options.EndDate) {
			workouts = append(workouts, event)
		}
	}

	return workouts
}

func remainingWorkoutEvents(events []Event, options *MicrocycleOptions) int {
	today := options.Now.Format(microcycleDateLayout)
	remaining := 0
	for index := range events {
		if eventDate(&events[index]) > today {
			remaining++
		}
	}

	return remaining
}

func missedWorkoutEvents(events []Event, activities []Activity, options *MicrocycleOptions) int {
	activityDates := map[string]int{}
	for index := range activities {
		activityDates[activityDate(&activities[index])]++
	}

	today := options.Now.Format(microcycleDateLayout)
	missed := 0
	for index := range events {
		date := eventDate(&events[index])
		if date > today {
			continue
		}
		if activityDates[date] == 0 {
			missed++
			continue
		}
		activityDates[date]--
	}

	return missed
}

func activityLoad(activities []Activity) int {
	total := 0
	for index := range activities {
		total += activities[index].TrainingLoad
	}

	return total
}

func activityDuration(activities []Activity) int {
	total := 0
	for index := range activities {
		total += activities[index].MovingTime
	}

	return total
}

func eventLoad(events []Event) int {
	total := 0
	for index := range events {
		total += events[index].TrainingLoad
	}

	return total
}

func eventDuration(events []Event) int {
	total := 0
	for index := range events {
		total += events[index].MovingTime
	}

	return total
}

func countZ4PlusSessions(sessions []CyclingSession) int {
	count := 0
	for index := range sessions {
		session := sessions[index]
		if session.Intensity >= microcycleZ4IntensityThreshold || sessionHasZ4Plus(session.ZoneTimes) {
			count++
		}
	}

	return count
}

func sessionHasZ4Plus(zones []ZoneTime) bool {
	for index := range zones {
		if zones[index].Secs > 0 && (zones[index].ID == "Z4" || zones[index].ID == "Z5" || zones[index].ID == "Z6" || zones[index].ID == "Z7") {
			return true
		}
	}

	return false
}

func intensityDistribution(zones map[string]float64) string {
	if len(zones) == 0 {
		return unknownState
	}

	low := zones["Z1"] + zones["Z2"]
	mid := zones["Z3"]
	high := zones["Z4"] + zones["Z5"] + zones["Z6"] + zones["Z7"]

	switch {
	case high >= microcycleHighZonePercent:
		return "intensity_heavy"
	case mid >= high && mid >= microcycleHighZonePercent:
		return "threshold_heavy"
	case low >= 75 && high > mid:
		return "polarized"
	case low >= microcyclePyramidalLowPercent:
		return "pyramidal"
	default:
		return "mixed"
	}
}

func countLongRides(sessions []CyclingSession) int {
	count := 0
	for index := range sessions {
		if sessions[index].MovingTimeSecs >= microcycleLongRideSeconds {
			count++
		}
	}

	return count
}

func previousLoad(activities []Activity, start string, days int) int {
	startDate, err := time.Parse(microcycleDateLayout, start)
	if err != nil {
		return 0
	}
	previousStart := startDate.AddDate(0, 0, -days).Format(microcycleDateLayout)
	previousEnd := startDate.AddDate(0, 0, -1).Format(microcycleDateLayout)

	// Current microcycle load is cycling-only; compare against cycling history
	// so runs/swims do not inflate the baseline and mislabel the week.
	filtered := filterActivitiesByDate(activities, previousStart, previousEnd)
	cycling := make([]Activity, 0, len(filtered))
	for index := range filtered {
		if IsCyclingActivityType(filtered[index].Type) {
			cycling = append(cycling, filtered[index])
		}
	}

	return activityLoad(cycling)
}

func compareLoad(current, previous int) string {
	if previous == 0 {
		return "unavailable"
	}
	ratio := float64(current) / float64(previous)
	switch {
	case ratio >= microcycleLoadAboveRatio:
		return "above"
	case ratio <= microcycleLoadBelowRatio:
		return "below"
	default:
		return "within_expected_range"
	}
}
