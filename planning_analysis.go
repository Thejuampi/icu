package icu

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	planWeekDays               = 7
	planBuildProgressionRatio  = 1.08
	planDeloadRatio            = 0.8
	planBelowToleranceRatio    = 0.85
	planAbovePeakRatio         = 1.15
	planTempoIntensity         = 0.7
	planRecoveryIntensity      = 0.65
	planWorkoutCategory        = "WORKOUT"
	planNoteCategory           = "NOTE"
	planSourcePlannedLoad      = "planned_event_load_heuristic"
	planAlignmentBelow         = "below_recent_tolerance"
	planAlignmentWithin        = "within_recent_tolerance"
	planAlignmentAboveAverage  = "above_recent_average"
	planAlignmentAbovePeak     = "above_recent_peak"
	planPhaseBuild             = "build"
	planPhaseRecovery          = "recovery"
	planConfidenceLow          = "low"
	planConfidenceModerate     = "moderate"
	planMinimumTrendWeeks      = 2
	planBuildWithDeloadWeeks   = 4
	planCueDefaultWarmupSecs   = 600
	planCueMinimumFinishSecs   = 600
	planCueHalfwayDivisor      = 2
	planMinutesPerHour         = 60
	planLongTitleMinutes       = 120
	planTitleRoundMinutes      = 15
	planTitleRoundHalfMinutes  = 7
	planZ2MicroWorkSeconds     = 240
	planZ2MicroRecoverySeconds = 40
	planZ2MicroRepeatMinutes   = 8
	planZ2MicroRepeatStep      = 4
	planZ2MicroMinRepeats      = 8
	planZ2MicroMaxRepeats      = 24
	planZ2ProfileBlockCount    = 4
	planZ2WarmupSeconds        = 900
	planZ2CooldownSeconds      = 600
	planForecastCtlTauDays     = 42
	planForecastAtlTauDays     = 7
	planForecastWatchTSB       = -10
	planForecastRedTSB         = -20
	planForecastStatusRed      = "red"
	planForecastStatusWatch    = "watch"
	planTargetRaceA            = "RACE_A"
	planTargetRaceB            = "RACE_B"
	planTargetRaceC            = "RACE_C"
	planTargetCategory         = "TARGET"
	planWellnessStateWatch     = "WATCH"
	planWellnessStateRed       = "RED"
	planDirectiveRecovery      = "recovery_priority"
	planDirectiveProtectTarget = "protect_target_event"
	planDirectiveConditional   = "conditional_build"
	planDirectiveBuild         = "progressive_build"
	planDecisionBaseScore      = 100
	planDecisionWatchPenalty   = 10
	planDecisionRedPenalty     = 25
	planDecisionLoadPenalty    = 15
	planDecisionPeakPenalty    = 20
	planDecisionAdaptPenalty   = 15
	planHeatSessionWatch       = 1
	planDriftWatchThreshold    = 5
	plannedClassificationRest  = "rest"
	plannedClassificationNote  = "note"
	plannedClassificationHigh  = "high_intensity"
	plannedClassificationTempo = "tempo_threshold"
	plannedClassificationLong  = "long_endurance"
	plannedClassificationEasy  = "recovery"
	plannedClassificationZ2    = "aerobic"
	plannedClassificationOpen  = "opener"
)

type TrainingPlanOptions struct {
	HistoryStartDate string `json:"historyStartDate,omitempty"`
	HistoryEndDate   string `json:"historyEndDate,omitempty"`
	PlanStartDate    string `json:"planStartDate,omitempty"`
	PlanEndDate      string `json:"planEndDate,omitempty"`
}

type TrainingPlanAnalysis struct {
	Scope           TrainingPlanScope             `json:"scope"`
	History         TrainingPlanHistory           `json:"history"`
	Anchors         TrainingPlanSportAnchors      `json:"anchors"`
	Phase           TrainingPlanPhase             `json:"phase"`
	Load            TrainingPlanLoad              `json:"load"`
	TargetStatus    TrainingPlanEventTargetStatus `json:"targetStatus"`
	Forecast        TrainingPlanForecast          `json:"forecast"`
	Decision        TrainingPlanDecisionGuidance  `json:"decision"`
	Sessions        TrainingPlanSessionSummary    `json:"sessions"`
	DayAdjustments  []TrainingPlanDayAdjustment   `json:"dayAdjustments,omitempty"`
	Weeks           []TrainingPlanWeek            `json:"weeks,omitempty"`
	PlannedSessions []TrainingPlanSession         `json:"plannedSessions,omitempty"`
	Warnings        []string                      `json:"warnings,omitempty"`
}

type TrainingPlanContext struct {
	SportSettings *SportSettings             `json:"-"`
	Wellness      *WellnessAnalysis          `json:"-"`
	Adaptation    *CyclingAdaptationAnalysis `json:"-"`
}

type TrainingPlanScope struct {
	HistoryStartDate string `json:"historyStartDate,omitempty"`
	HistoryEndDate   string `json:"historyEndDate,omitempty"`
	PlanStartDate    string `json:"planStartDate,omitempty"`
	PlanEndDate      string `json:"planEndDate,omitempty"`
	Events           int    `json:"events"`
	WorkoutEvents    int    `json:"workoutEvents"`
	NoteEvents       int    `json:"noteEvents"`
}

type TrainingPlanHistory struct {
	CompletedWeeks       int                         `json:"completedWeeks"`
	AverageWeeklyLoad    float64                     `json:"averageWeeklyLoad"`
	PeakWeeklyLoad       int                         `json:"peakWeeklyLoad"`
	RecentWeeklyLoad     int                         `json:"recentWeeklyLoad"`
	AverageWeeklyHours   float64                     `json:"averageWeeklyHours"`
	CurrentStateLabel    string                      `json:"currentStateLabel,omitempty"`
	CurrentLoadPressure  float64                     `json:"currentLoadPressure,omitempty"`
	CurrentCTL           float64                     `json:"currentCtl,omitempty"`
	CurrentATL           float64                     `json:"currentAtl,omitempty"`
	CurrentTSB           float64                     `json:"currentTsb,omitempty"`
	CurrentDecoupling    float64                     `json:"currentDecoupling,omitempty"`
	CurrentHotSessions   int                         `json:"currentHotSessions,omitempty"`
	CurrentStateSource   string                      `json:"currentStateSource,omitempty"`
	PlannedLoadAlignment string                      `json:"plannedLoadAlignment,omitempty"`
	Weeks                []TrainingPlanCompletedWeek `json:"weeks,omitempty"`
}

type TrainingPlanSportAnchors struct {
	FTP       int    `json:"ftp,omitempty"`
	IndoorFTP int    `json:"indoorFtp,omitempty"`
	LTHR      int    `json:"lthr,omitempty"`
	MaxHR     int    `json:"maxHr,omitempty"`
	WPrime    int    `json:"wPrime,omitempty"`
	PMax      int    `json:"pMax,omitempty"`
	Source    string `json:"source,omitempty"`
}

type TrainingPlanEventTargetStatus struct {
	TargetEvents   int                       `json:"targetEvents"`
	RaceEvents     int                       `json:"raceEvents"`
	UpcomingEvents int                       `json:"upcomingEvents"`
	NextEventDate  string                    `json:"nextEventDate,omitempty"`
	Readiness      string                    `json:"readiness,omitempty"`
	Source         string                    `json:"source,omitempty"`
	Events         []TrainingPlanTargetEvent `json:"events,omitempty"`
}

type TrainingPlanTargetEvent struct {
	Date           string  `json:"date,omitempty"`
	Name           string  `json:"name,omitempty"`
	Category       string  `json:"category,omitempty"`
	Target         string  `json:"target,omitempty"`
	LoadTarget     int     `json:"loadTarget,omitempty"`
	TimeTarget     int     `json:"timeTarget,omitempty"`
	DistanceTarget float64 `json:"distanceTarget,omitempty"`
}

type TrainingPlanForecast struct {
	StartCTL   float64                     `json:"startCtl,omitempty"`
	StartATL   float64                     `json:"startAtl,omitempty"`
	StartTSB   float64                     `json:"startTsb,omitempty"`
	EndCTL     float64                     `json:"endCtl,omitempty"`
	EndATL     float64                     `json:"endAtl,omitempty"`
	EndTSB     float64                     `json:"endTsb,omitempty"`
	LowestTSB  float64                     `json:"lowestTsb,omitempty"`
	HighestTSB float64                     `json:"highestTsb,omitempty"`
	Status     string                      `json:"status,omitempty"`
	Source     string                      `json:"source,omitempty"`
	Points     []TrainingPlanForecastPoint `json:"points,omitempty"`
}

type TrainingPlanForecastPoint struct {
	Date        string  `json:"date,omitempty"`
	PlannedLoad int     `json:"plannedLoad"`
	CTL         float64 `json:"ctl,omitempty"`
	ATL         float64 `json:"atl,omitempty"`
	TSB         float64 `json:"tsb,omitempty"`
}

type TrainingPlanDayAdjustment struct {
	Date           string `json:"date,omitempty"`
	SessionName    string `json:"sessionName,omitempty"`
	Classification string `json:"classification,omitempty"`
	Severity       string `json:"severity,omitempty"`
	Condition      string `json:"condition,omitempty"`
	Action         string `json:"action,omitempty"`
	Source         string `json:"source,omitempty"`
}

type TrainingPlanDecisionGuidance struct {
	PrimaryDirective string   `json:"primaryDirective,omitempty"`
	ADEScore         int      `json:"adeScore,omitempty"`
	Penalties        []string `json:"penalties,omitempty"`
	TrainingGuidance string   `json:"trainingGuidance,omitempty"`
	Source           string   `json:"source,omitempty"`
}

type TrainingPlanCompletedWeek struct {
	ISOWeek               string  `json:"isoWeek"`
	StartDate             string  `json:"startDate,omitempty"`
	EndDate               string  `json:"endDate,omitempty"`
	Load                  int     `json:"load"`
	MovingTimeSecs        int     `json:"movingTimeSecs"`
	MovingTimeHours       float64 `json:"movingTimeHours"`
	Sessions              int     `json:"sessions"`
	HighIntensityDays     int     `json:"highIntensityDays"`
	LongEnduranceSessions int     `json:"longEnduranceSessions"`
	MeanIntensity         float64 `json:"meanIntensity,omitempty"`
	MeanDecoupling        float64 `json:"meanDecoupling,omitempty"`
	Tolerance             string  `json:"tolerance,omitempty"`
}

type TrainingPlanPhase struct {
	Label      string `json:"label,omitempty"`
	Pattern    string `json:"pattern,omitempty"`
	Intent     string `json:"intent,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Source     string `json:"source,omitempty"`
}

type TrainingPlanLoad struct {
	TotalPlannedLoad       int     `json:"totalPlannedLoad"`
	AverageWeeklyLoad      float64 `json:"averageWeeklyLoad"`
	PeakWeeklyLoad         int     `json:"peakWeeklyLoad"`
	LowestWeeklyLoad       int     `json:"lowestWeeklyLoad"`
	ProgressionPercent     float64 `json:"progressionPercent,omitempty"`
	FinalWeekDeloadPercent float64 `json:"finalWeekDeloadPercent,omitempty"`
	WeeklyLoadTrend        string  `json:"weeklyLoadTrend,omitempty"`
}

type TrainingPlanSessionSummary struct {
	Events                 int `json:"events"`
	Workouts               int `json:"workouts"`
	Notes                  int `json:"notes"`
	RestDays               int `json:"restDays"`
	HighIntensitySessions  int `json:"highIntensitySessions"`
	TempoThresholdSessions int `json:"tempoThresholdSessions"`
	LongEnduranceSessions  int `json:"longEnduranceSessions"`
	RecoverySessions       int `json:"recoverySessions"`
	AerobicSessions        int `json:"aerobicSessions"`
	OpenerSessions         int `json:"openerSessions"`
}

type TrainingPlanWeek struct {
	ISOWeek                string         `json:"isoWeek"`
	StartDate              string         `json:"startDate,omitempty"`
	EndDate                string         `json:"endDate,omitempty"`
	Role                   string         `json:"role,omitempty"`
	Focus                  string         `json:"focus,omitempty"`
	Events                 int            `json:"events"`
	Workouts               int            `json:"workouts"`
	Notes                  int            `json:"notes"`
	RestDays               int            `json:"restDays"`
	PlannedLoad            int            `json:"plannedLoad"`
	MovingTimeSecs         int            `json:"movingTimeSecs"`
	MovingTimeHours        float64        `json:"movingTimeHours"`
	HighIntensitySessions  int            `json:"highIntensitySessions"`
	TempoThresholdSessions int            `json:"tempoThresholdSessions"`
	LongEnduranceSessions  int            `json:"longEnduranceSessions"`
	RecoverySessions       int            `json:"recoverySessions"`
	AerobicSessions        int            `json:"aerobicSessions"`
	OpenerSessions         int            `json:"openerSessions"`
	SessionMix             map[string]int `json:"sessionMix,omitempty"`
	LoadDelta              int            `json:"loadDelta,omitempty"`
	LoadDeltaPercent       float64        `json:"loadDeltaPercent,omitempty"`
	Warnings               []string       `json:"warnings,omitempty"`
}

type TrainingPlanSession struct {
	Date              string                `json:"date,omitempty"`
	Day               string                `json:"day,omitempty"`
	Name              string                `json:"name,omitempty"`
	Category          string                `json:"category,omitempty"`
	Type              string                `json:"type,omitempty"`
	MovingTimeSecs    int                   `json:"movingTimeSecs"`
	MovingTimeMinutes float64               `json:"movingTimeMinutes"`
	TrainingLoad      int                   `json:"trainingLoad"`
	Intensity         float64               `json:"intensity,omitempty"`
	Classification    string                `json:"classification,omitempty"`
	WeekRole          string                `json:"weekRole,omitempty"`
	KeySession        bool                  `json:"keySession"`
	Warnings          []string              `json:"warnings,omitempty"`
	Execution         TrainingPlanExecution `json:"execution"`
}

type TrainingPlanExecution struct {
	RecommendedTitle string                     `json:"recommendedTitle,omitempty"`
	Intent           string                     `json:"intent,omitempty"`
	DeviceCuePolicy  string                     `json:"deviceCuePolicy,omitempty"`
	Cues             []TrainingPlanCue          `json:"cues,omitempty"`
	WorkoutProfile   TrainingPlanWorkoutProfile `json:"workoutProfile,omitempty"`
}

type TrainingPlanCue struct {
	OffsetSeconds int     `json:"offsetSeconds"`
	OffsetMinutes float64 `json:"offsetMinutes"`
	CueType       string  `json:"cueType,omitempty"`
	Tone          string  `json:"tone,omitempty"`
	Message       string  `json:"message"`
}

type TrainingPlanWorkoutProfile struct {
	Name        string                     `json:"name,omitempty"`
	Description string                     `json:"description,omitempty"`
	AppliesWhen string                     `json:"appliesWhen,omitempty"`
	Blocks      []TrainingPlanWorkoutBlock `json:"blocks,omitempty"`
}

type TrainingPlanWorkoutBlock struct {
	Name            string `json:"name,omitempty"`
	Repeat          int    `json:"repeat,omitempty"`
	WorkSeconds     int    `json:"workSeconds,omitempty"`
	RecoverySeconds int    `json:"recoverySeconds,omitempty"`
	WorkTarget      string `json:"workTarget,omitempty"`
	RecoveryTarget  string `json:"recoveryTarget,omitempty"`
	Cue             string `json:"cue,omitempty"`
}

type trainingPlanAccumulator struct {
	analysis TrainingPlanAnalysis
	weeks    map[string]*TrainingPlanWeek
}

type trainingPlanHistoryWeek struct {
	load                  int
	seconds               int
	sessions              int
	highIntensityDays     int
	longEnduranceSessions int
	intensity             numericAccumulator
	decoupling            numericAccumulator
	startDate             string
	endDate               string
}

func AnalyzeTrainingPlan(activities []Activity, events []Event, options TrainingPlanOptions) TrainingPlanAnalysis {
	return AnalyzeTrainingPlanWithContext(activities, events, options, TrainingPlanContext{
		SportSettings: nil,
		Wellness:      nil,
		Adaptation:    nil,
	})
}

func AnalyzeTrainingPlanWithContext(
	activities []Activity,
	events []Event,
	options TrainingPlanOptions,
	context TrainingPlanContext,
) TrainingPlanAnalysis {
	accumulator := newTrainingPlanAccumulator(events, options)

	for index := range events {
		accumulator.addEvent(&events[index], options)
	}

	analysis := accumulator.finish(options)
	analysis.History = summarizeTrainingPlanHistory(activities, options)
	analysis.History.PlannedLoadAlignment = plannedLoadAlignment(&analysis.History, &analysis.Load)
	analysis.Anchors = trainingPlanSportAnchors(context.SportSettings, activities)
	analysis.TargetStatus = trainingPlanTargetStatus(events, options, &analysis.History)
	analysis.Forecast = trainingPlanForecast(&analysis)
	analysis.DayAdjustments = trainingPlanDayAdjustments(&analysis, context.Wellness)
	analysis.Decision = trainingPlanDecisionGuidance(&analysis, context.Wellness, context.Adaptation)
	analysis.Warnings = trainingPlanWarnings(&analysis)

	return analysis
}

func newTrainingPlanAccumulator(events []Event, options TrainingPlanOptions) trainingPlanAccumulator {
	var (
		accumulator trainingPlanAccumulator
		scope       TrainingPlanScope
	)

	accumulator.weeks = map[string]*TrainingPlanWeek{}
	scope.HistoryStartDate = options.HistoryStartDate
	scope.HistoryEndDate = options.HistoryEndDate
	scope.PlanStartDate = options.PlanStartDate
	scope.PlanEndDate = options.PlanEndDate
	scope.Events = len(events)
	accumulator.analysis.Scope = scope

	accumulator.ensureWeeksForRange(options)

	return accumulator
}

func (accumulator *trainingPlanAccumulator) ensureWeeksForRange(options TrainingPlanOptions) {
	if options.PlanStartDate == "" || options.PlanEndDate == "" {
		return
	}

	start, err := time.Parse(analysisDateLayout, options.PlanStartDate)
	if err != nil {
		return
	}

	end, err := time.Parse(analysisDateLayout, options.PlanEndDate)
	if err != nil || end.Before(start) {
		return
	}

	for current := start; !current.After(end); current = current.AddDate(0, 0, planWeekDays) {
		weekStart := isoWeekStart(current)
		weekEnd := weekStart.AddDate(0, 0, planWeekDays-1)
		key := isoWeekKey(weekStart)

		accumulator.ensureWeek(key, weekStart.Format(analysisDateLayout), weekEnd.Format(analysisDateLayout))
	}
}

func (accumulator *trainingPlanAccumulator) addEvent(event *Event, options TrainingPlanOptions) {
	date := eventDate(event)

	if date == "" || !dateWithinRange(date, options.PlanStartDate, options.PlanEndDate) {
		return
	}

	parsed, err := time.Parse(analysisDateLayout, date)
	if err != nil {
		return
	}

	weekStart := isoWeekStart(parsed)
	weekEnd := weekStart.AddDate(0, 0, planWeekDays-1)
	key := isoWeekKey(parsed)

	week := accumulator.ensureWeek(key, weekStart.Format(analysisDateLayout), weekEnd.Format(analysisDateLayout))
	session := trainingPlanSessionFromEvent(event, parsed)

	accumulator.analysis.PlannedSessions = append(accumulator.analysis.PlannedSessions, session)
	accumulator.addSessionToSummary(&session)
	addSessionToWeek(week, &session)
}

func (accumulator *trainingPlanAccumulator) ensureWeek(key, startDate, endDate string) *TrainingPlanWeek {
	week, ok := accumulator.weeks[key]
	if ok {
		return week
	}

	var weekValue TrainingPlanWeek
	weekValue.ISOWeek = key
	weekValue.StartDate = startDate
	weekValue.EndDate = endDate
	weekValue.SessionMix = map[string]int{}
	week = &weekValue
	accumulator.weeks[key] = week

	return week
}

func (accumulator *trainingPlanAccumulator) addSessionToSummary(session *TrainingPlanSession) {
	accumulator.analysis.Sessions.Events++

	switch normalizedCategory(session.Category) {
	case planWorkoutCategory:
		accumulator.analysis.Scope.WorkoutEvents++
		accumulator.analysis.Sessions.Workouts++
	case planNoteCategory:
		accumulator.analysis.Scope.NoteEvents++
		accumulator.analysis.Sessions.Notes++
	}

	addClassificationToSummary(session.Classification, &accumulator.analysis.Sessions)
}

func (accumulator *trainingPlanAccumulator) finish(options TrainingPlanOptions) TrainingPlanAnalysis {
	accumulator.analysis.Weeks = sortedTrainingPlanWeeks(accumulator.weeks)
	finishTrainingPlanWeeks(accumulator.analysis.Weeks)
	sortTrainingPlanSessions(accumulator.analysis.PlannedSessions)
	assignSessionWeekRoles(accumulator.analysis.PlannedSessions, accumulator.analysis.Weeks)
	accumulator.analysis.Load = trainingPlanLoad(accumulator.analysis.Weeks)
	accumulator.analysis.Phase = classifyTrainingPlanPhase(accumulator.analysis.Weeks)

	if accumulator.analysis.Scope.PlanStartDate == "" {
		accumulator.analysis.Scope.PlanStartDate = inferredPlanStart(options, accumulator.analysis.Weeks)
	}

	if accumulator.analysis.Scope.PlanEndDate == "" {
		accumulator.analysis.Scope.PlanEndDate = inferredPlanEnd(options, accumulator.analysis.Weeks)
	}

	return accumulator.analysis
}

func trainingPlanSessionFromEvent(event *Event, parsed time.Time) TrainingPlanSession {
	classification := classifyPlannedEvent(event)

	var session TrainingPlanSession
	session.Date = parsed.Format(analysisDateLayout)
	session.Day = parsed.Weekday().String()
	session.Name = event.Name
	session.Category = event.Category
	session.Type = event.Type
	session.MovingTimeSecs = event.MovingTime
	session.MovingTimeMinutes = round2(float64(event.MovingTime) / secondsPerMinute)
	session.TrainingLoad = event.TrainingLoad
	session.Intensity = event.Intensity
	session.Classification = classification
	session.KeySession = isKeyTrainingPlanSession(classification)
	session.Execution = trainingPlanExecutionForEvent(event, classification)

	return session
}

func trainingPlanExecutionForEvent(event *Event, classification string) TrainingPlanExecution {
	var execution TrainingPlanExecution

	execution.RecommendedTitle = recommendedTrainingPlanTitle(event, classification)
	execution.Intent = trainingPlanExecutionIntent(classification)
	execution.DeviceCuePolicy = "use as workout text events when the target format supports device cues"
	execution.Cues = trainingPlanCues(event, classification, execution.RecommendedTitle)
	execution.WorkoutProfile = trainingPlanWorkoutProfile(event, classification)

	return execution
}

func trainingPlanWorkoutProfile(event *Event, classification string) TrainingPlanWorkoutProfile {
	if !isZ2VariationCandidate(event, classification) {
		var profile TrainingPlanWorkoutProfile

		return profile
	}

	var profile TrainingPlanWorkoutProfile
	profile.Name = "indoor_z2_micro_variation"
	profile.Description = "Varied indoor Z2 with 4m waves, 40s HR-control valleys, and max-Z2 caps."
	profile.AppliesWhen = "Use when this Z2 ride is done indoors; outdoor execution can stay terrain-led."
	profile.Blocks = indoorZ2VariationBlocks(event.MovingTime)

	return profile
}

func isZ2VariationCandidate(event *Event, classification string) bool {
	if !IsCyclingActivityType(event.Type) {
		return false
	}

	switch classification {
	case plannedClassificationLong, plannedClassificationZ2:
		return true
	default:
		return false
	}
}

func indoorZ2VariationBlocks(seconds int) []TrainingPlanWorkoutBlock {
	blocks := make([]TrainingPlanWorkoutBlock, 0, planZ2ProfileBlockCount)
	blocks = append(blocks, z2WarmupBlock(), z2MicroIntervalBlock(seconds), z2ShadowBlock(), z2CooldownBlock())

	return blocks
}

func z2WarmupBlock() TrainingPlanWorkoutBlock {
	var block TrainingPlanWorkoutBlock
	block.Name = "Progressive Z2 warmup"
	block.Repeat = 1
	block.WorkSeconds = planZ2WarmupSeconds
	block.WorkTarget = "Z1 to low Z2"
	block.Cue = "Let HR rise slowly before the first Z2 wave."

	return block
}

func z2MicroIntervalBlock(seconds int) TrainingPlanWorkoutBlock {
	var block TrainingPlanWorkoutBlock
	block.Name = "Z2 wave micro-intervals"
	block.Repeat = indoorZ2MicroRepeats(seconds)
	block.WorkSeconds = planZ2MicroWorkSeconds
	block.RecoverySeconds = planZ2MicroRecoverySeconds
	block.WorkTarget = "rotate low Z2, mid Z2 shadow, high Z2, cap at max Z2"
	block.RecoveryTarget = "low Z2 or high Z1 until HR settles"
	block.Cue = "Use the 40s valleys to control HR drift, not to coast fully."

	return block
}

func z2ShadowBlock() TrainingPlanWorkoutBlock {
	var block TrainingPlanWorkoutBlock
	block.Name = "Middle shadows"
	block.Repeat = 3
	block.WorkSeconds = planCueDefaultWarmupSecs
	block.RecoverySeconds = planZ2MicroRecoverySeconds
	block.WorkTarget = "mid Z2 with short high-Z2 shadows"
	block.RecoveryTarget = "low Z2"
	block.Cue = "Float between low and high Z2 without turning it into tempo."

	return block
}

func z2CooldownBlock() TrainingPlanWorkoutBlock {
	var block TrainingPlanWorkoutBlock
	block.Name = "Aerobic cooldown"
	block.Repeat = 1
	block.WorkSeconds = planZ2CooldownSeconds
	block.WorkTarget = "low Z2 to Z1"
	block.Cue = "Bring HR down before stepping off."

	return block
}

func indoorZ2MicroRepeats(seconds int) int {
	minutes := seconds / secondsPerMinute
	repeats := minutes / planZ2MicroRepeatMinutes
	repeats = (repeats / planZ2MicroRepeatStep) * planZ2MicroRepeatStep

	if repeats < planZ2MicroMinRepeats {
		return planZ2MicroMinRepeats
	}

	if repeats > planZ2MicroMaxRepeats {
		return planZ2MicroMaxRepeats
	}

	return repeats
}

func recommendedTrainingPlanTitle(event *Event, classification string) string {
	name := strings.ToLower(event.Name)
	pattern := extractIntervalPattern(name)

	if strings.Contains(name, "40/20") || strings.Contains(name, "40-20") {
		return "40/20 VO2Max"
	}

	switch classification {
	case plannedClassificationHigh:
		return intervalOrDurationTitle(pattern, event.MovingTime, "VO2Max")
	case plannedClassificationTempo:
		return intervalOrDurationTitle(pattern, event.MovingTime, tempoTitleLabel(name))
	case plannedClassificationLong:
		return durationTrainingPlanTitle(event.MovingTime, "Z2")
	case plannedClassificationEasy:
		return durationTrainingPlanTitle(event.MovingTime, "Recovery")
	case plannedClassificationZ2:
		return durationTrainingPlanTitle(event.MovingTime, "Z2")
	case plannedClassificationOpen:
		return "VO2 Openers"
	case plannedClassificationRest:
		return "Rest Day"
	default:
		return event.Name
	}
}

func tempoTitleLabel(name string) string {
	if strings.Contains(name, "sweet") || strings.Contains(name, "ss") {
		return "SS"
	}

	if strings.Contains(name, "threshold") {
		return "Threshold"
	}

	return "Tempo"
}

func intervalOrDurationTitle(pattern string, seconds int, label string) string {
	if pattern != "" {
		return pattern + " " + label
	}

	return durationTrainingPlanTitle(seconds, label)
}

func durationTrainingPlanTitle(seconds int, label string) string {
	minutes := seconds / secondsPerMinute

	if minutes >= planLongTitleMinutes {
		return roundedDurationTrainingPlanTitle(minutes, label)
	}

	return fmt.Sprintf("%dm %s", minutes, label)
}

func roundedDurationTrainingPlanTitle(minutes int, label string) string {
	roundedMinutes := roundMinutesToStep(minutes, planTitleRoundMinutes)
	hours := roundedMinutes / planMinutesPerHour
	remainder := roundedMinutes % planMinutesPerHour

	if remainder == 0 {
		return fmt.Sprintf("%dh %s", hours, label)
	}

	return fmt.Sprintf("%dh%02d %s", hours, remainder, label)
}

func roundMinutesToStep(minutes, step int) int {
	if step == 0 {
		return minutes
	}

	return ((minutes + planTitleRoundHalfMinutes) / step) * step
}

func extractIntervalPattern(name string) string {
	for position := range len(name) {
		if !isASCIIDigit(name[position]) {
			continue
		}

		pattern, ok := intervalPatternAt(name, position)
		if ok {
			return pattern
		}
	}

	return ""
}

func intervalPatternAt(name string, position int) (string, bool) {
	leftStart := position
	leftEnd := scanDigits(name, leftStart)

	if leftEnd >= len(name) || name[leftEnd] != 'x' {
		return "", false
	}

	rightStart := leftEnd + 1
	rightEnd := scanDigits(name, rightStart)

	if rightStart == rightEnd {
		return "", false
	}

	return name[leftStart:leftEnd] + "x" + name[rightStart:rightEnd], true
}

func scanDigits(value string, start int) int {
	position := start

	for position < len(value) && isASCIIDigit(value[position]) {
		position++
	}

	return position
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func trainingPlanExecutionIntent(classification string) string {
	switch classification {
	case plannedClassificationHigh:
		return "raise aerobic ceiling with repeatable VO2 quality"
	case plannedClassificationTempo:
		return "build steady pressure without spiking fatigue"
	case plannedClassificationLong:
		return "extend aerobic durability with controlled drift"
	case plannedClassificationEasy:
		return "promote recovery and keep load easy"
	case plannedClassificationZ2:
		return "add low-intensity aerobic volume"
	case plannedClassificationOpen:
		return "prime the legs without spending matches"
	case plannedClassificationRest:
		return "absorb training load"
	default:
		return "support the planned training block"
	}
}

func trainingPlanCues(event *Event, classification, title string) []TrainingPlanCue {
	switch classification {
	case plannedClassificationHigh:
		return highIntensityCues(event.MovingTime, title)
	case plannedClassificationTempo:
		return tempoCues(event.MovingTime, title)
	case plannedClassificationLong:
		return longEnduranceCues(event.MovingTime, title)
	case plannedClassificationEasy:
		return recoveryCues(event.MovingTime, title)
	case plannedClassificationZ2:
		return aerobicCues(event.MovingTime, title)
	case plannedClassificationOpen:
		return openerCues(event.MovingTime, title)
	default:
		return nil
	}
}

func highIntensityCues(seconds int, title string) []TrainingPlanCue {
	var cues []TrainingPlanCue

	addTrainingPlanCue(&cues, 0, seconds, "preview", "focus", "Today: "+title+". Keep the first hard effort controlled.")
	addTrainingPlanCue(&cues, cueWarmupOffset(seconds), seconds, "work", "focus", "Hard work coming. Breathe first, then power.")
	addTrainingPlanCue(
		&cues,
		cueHalfwayOffset(seconds),
		seconds,
		"encouragement",
		"encouragement",
		"Good. Keep the next rep sharp and repeatable.",
	)
	addTrainingPlanCue(&cues, cueFinishOffset(seconds), seconds, "finish", "recovery", "Hard work done. Easy spin and bring HR down.")

	return cues
}

func tempoCues(seconds int, title string) []TrainingPlanCue {
	var cues []TrainingPlanCue

	addTrainingPlanCue(&cues, 0, seconds, "preview", "focus", "Today: "+title+". Settle, then hold steady pressure.")
	addTrainingPlanCue(&cues, cueWarmupOffset(seconds), seconds, "work", "focus", "Tempo is coming. Smooth cadence, no surges.")
	addTrainingPlanCue(
		&cues,
		cueHalfwayOffset(seconds),
		seconds,
		"encouragement",
		"encouragement",
		"Nice. Stay patient and make the back half steady.",
	)
	addTrainingPlanCue(&cues, cueFinishOffset(seconds), seconds, "finish", "recovery", "Release the pressure. Cool down cleanly.")

	return cues
}

func longEnduranceCues(seconds int, title string) []TrainingPlanCue {
	var cues []TrainingPlanCue

	addTrainingPlanCue(&cues, 0, seconds, "preview", "focus", "Today: "+title+". Z2 patience and fuel early.")
	addTrainingPlanCue(&cues, cueHalfwayOffset(seconds), seconds, "check", "restraint", "Check drift. If HR is climbing, ease off a little.")
	addTrainingPlanCue(&cues, cueFinishOffset(seconds), seconds, "finish", "encouragement", "Good endurance work. Keep the finish controlled.")

	return cues
}

func recoveryCues(seconds int, title string) []TrainingPlanCue {
	var cues []TrainingPlanCue

	addTrainingPlanCue(&cues, 0, seconds, "preview", "restraint", "Today: "+title+". Keep it very easy from the start.")
	addTrainingPlanCue(&cues, cueHalfwayOffset(seconds), seconds, "check", "restraint", "Still easy. If it feels like training, back off.")
	addTrainingPlanCue(&cues, cueFinishOffset(seconds), seconds, "finish", "recovery", "Done. Leave fresher than you started.")

	return cues
}

func aerobicCues(seconds int, title string) []TrainingPlanCue {
	var cues []TrainingPlanCue

	addTrainingPlanCue(&cues, 0, seconds, "preview", "focus", "Today: "+title+". Keep it conversational and economical.")
	addTrainingPlanCue(&cues, cueHalfwayOffset(seconds), seconds, "check", "restraint", "Stay aerobic. No need to chase power.")
	addTrainingPlanCue(&cues, cueFinishOffset(seconds), seconds, "finish", "recovery", "Smooth finish. Save the legs for the next key day.")

	return cues
}

func openerCues(seconds int, title string) []TrainingPlanCue {
	var cues []TrainingPlanCue

	addTrainingPlanCue(&cues, 0, seconds, "preview", "restraint", "Today: "+title+". Wake the legs, do not spend matches.")
	addTrainingPlanCue(&cues, cueWarmupOffset(seconds), seconds, "work", "focus", "Short efforts ahead. Snap, relax, recover fully.")
	addTrainingPlanCue(&cues, cueFinishOffset(seconds), seconds, "finish", "recovery", "Openers complete. Keep the rest easy.")

	return cues
}

func addTrainingPlanCue(cues *[]TrainingPlanCue, offset, duration int, cueType, tone, message string) {
	if duration > 0 && offset >= duration {
		return
	}

	var cue TrainingPlanCue
	cue.OffsetSeconds = offset
	cue.OffsetMinutes = round2(float64(offset) / secondsPerMinute)
	cue.CueType = cueType
	cue.Tone = tone
	cue.Message = message

	*cues = append(*cues, cue)
}

func cueWarmupOffset(seconds int) int {
	if seconds > planCueDefaultWarmupSecs {
		return planCueDefaultWarmupSecs
	}

	return 0
}

func cueHalfwayOffset(seconds int) int {
	return seconds / planCueHalfwayDivisor
}

func cueFinishOffset(seconds int) int {
	if seconds > planCueMinimumFinishSecs {
		return seconds - planCueMinimumFinishSecs
	}

	return 0
}

func addSessionToWeek(week *TrainingPlanWeek, session *TrainingPlanSession) {
	week.Events++
	week.PlannedLoad += session.TrainingLoad
	week.MovingTimeSecs += session.MovingTimeSecs
	week.SessionMix[session.Classification]++

	switch normalizedCategory(session.Category) {
	case planWorkoutCategory:
		week.Workouts++
	case planNoteCategory:
		week.Notes++
	}

	addClassificationToWeek(session.Classification, week)
}

func addClassificationToSummary(classification string, summary *TrainingPlanSessionSummary) {
	switch classification {
	case plannedClassificationRest:
		summary.RestDays++
	case plannedClassificationHigh:
		summary.HighIntensitySessions++
	case plannedClassificationTempo:
		summary.TempoThresholdSessions++
	case plannedClassificationLong:
		summary.LongEnduranceSessions++
		summary.AerobicSessions++
	case plannedClassificationEasy:
		summary.RecoverySessions++
	case plannedClassificationZ2:
		summary.AerobicSessions++
	case plannedClassificationOpen:
		summary.OpenerSessions++
	}
}

func addClassificationToWeek(classification string, week *TrainingPlanWeek) {
	switch classification {
	case plannedClassificationRest:
		week.RestDays++
	case plannedClassificationHigh:
		week.HighIntensitySessions++
	case plannedClassificationTempo:
		week.TempoThresholdSessions++
	case plannedClassificationLong:
		week.LongEnduranceSessions++
		week.AerobicSessions++
	case plannedClassificationEasy:
		week.RecoverySessions++
	case plannedClassificationZ2:
		week.AerobicSessions++
	case plannedClassificationOpen:
		week.OpenerSessions++
	}
}

func classifyPlannedEvent(event *Event) string {
	category := normalizedCategory(event.Category)

	if category != planWorkoutCategory {
		if isRestEvent(event) {
			return plannedClassificationRest
		}

		return plannedClassificationNote
	}

	name := strings.ToLower(event.Name)

	if strings.Contains(name, "opener") {
		return plannedClassificationOpen
	}

	if strings.Contains(name, "vo2") || event.Intensity >= highIntensityThreshold {
		return plannedClassificationHigh
	}

	if strings.Contains(name, "threshold") || strings.Contains(name, "tempo") || event.Intensity >= planTempoIntensity {
		return plannedClassificationTempo
	}

	if event.MovingTime >= longEnduranceSeconds || strings.Contains(name, "long") {
		return plannedClassificationLong
	}

	if strings.Contains(name, "recover") || strings.Contains(name, "easy") || event.Intensity > 0 && event.Intensity < planRecoveryIntensity {
		return plannedClassificationEasy
	}

	return plannedClassificationZ2
}

func isRestEvent(event *Event) bool {
	name := strings.ToLower(event.Name)

	return strings.Contains(name, "off") || strings.Contains(name, "rest") || strings.Contains(name, "recovery")
}

func isKeyTrainingPlanSession(classification string) bool {
	switch classification {
	case plannedClassificationHigh, plannedClassificationTempo, plannedClassificationLong:
		return true
	default:
		return false
	}
}

func finishTrainingPlanWeeks(weeks []TrainingPlanWeek) {
	peakIndex := peakTrainingPlanWeekIndex(weeks)

	for index := range weeks {
		weeks[index].MovingTimeHours = round2(float64(weeks[index].MovingTimeSecs) / secondsPerHour)
		weeks[index].Role = trainingPlanWeekRole(weeks, index, peakIndex)
		weeks[index].Focus = trainingPlanWeekFocus(&weeks[index])

		if index > 0 {
			previous := weeks[index-1].PlannedLoad
			weeks[index].LoadDelta = weeks[index].PlannedLoad - previous

			if previous > 0 {
				weeks[index].LoadDeltaPercent = round2(float64(weeks[index].LoadDelta) * percentScale / float64(previous))
			}
		}
	}
}

func trainingPlanWeekRole(weeks []TrainingPlanWeek, index, peakIndex int) string {
	if weeks[index].PlannedLoad == 0 && weeks[index].Workouts == 0 {
		return "empty"
	}

	if index > 0 && isDeloadWeek(weeks[index].PlannedLoad, weeks[index-1].PlannedLoad) {
		return "deload"
	}

	if index == peakIndex && index > 0 {
		return "overload"
	}

	if index == 0 && len(weeks) >= 3 {
		return "reentry"
	}

	if index > 0 && isBuildProgressionWeek(weeks[index].PlannedLoad, weeks[index-1].PlannedLoad) {
		return planPhaseBuild
	}

	return "steady"
}

func trainingPlanWeekFocus(week *TrainingPlanWeek) string {
	if week.Workouts == 0 {
		return "rest or notes only"
	}

	if week.HighIntensitySessions > 0 && week.TempoThresholdSessions > 0 && week.LongEnduranceSessions > 0 {
		return "mixed intensity and durability"
	}

	if week.HighIntensitySessions > 0 && week.LongEnduranceSessions > 0 {
		return "vo2 and endurance durability"
	}

	if week.TempoThresholdSessions > 0 && week.LongEnduranceSessions > 0 {
		return "threshold/tempo and endurance durability"
	}

	if week.LongEnduranceSessions > 0 {
		return "endurance durability"
	}

	if week.RecoverySessions >= week.Workouts {
		return "absorption"
	}

	return "general aerobic"
}

func classifyTrainingPlanPhase(weeks []TrainingPlanWeek) TrainingPlanPhase {
	var phase TrainingPlanPhase
	phase.Source = planSourcePlannedLoad

	if len(weeks) == 0 {
		phase.Label = unknownState
		phase.Pattern = "no_planned_weeks"
		phase.Intent = "planning data unavailable"
		phase.Confidence = planConfidenceLow

		return phase
	}

	if totalPlannedWeekLoad(weeks) == 0 {
		phase.Label = planPhaseRecovery
		phase.Pattern = "no_planned_load"
		phase.Intent = "absorption or unstructured period"
		phase.Confidence = planConfidenceModerate

		return phase
	}

	if isBuildWithDeload(weeks) {
		phase.Label = planPhaseBuild
		phase.Pattern = "build_with_deload"
		phase.Intent = "progressive build with planned absorption week"
		phase.Confidence = planConfidenceModerate

		return phase
	}

	if isProgressiveBuild(weeks) {
		phase.Label = planPhaseBuild
		phase.Pattern = "progressive_build"
		phase.Intent = "progressively increase training stimulus"
		phase.Confidence = planConfidenceModerate

		return phase
	}

	if endsWithDeload(weeks) {
		phase.Label = planPhaseRecovery
		phase.Pattern = "planned_deload"
		phase.Intent = "absorb prior loading"
		phase.Confidence = planConfidenceModerate

		return phase
	}

	phase.Label = "maintenance"
	phase.Pattern = "stable_load"
	phase.Intent = "maintain fitness with controlled variation"
	phase.Confidence = planConfidenceLow

	return phase
}

func trainingPlanLoad(weeks []TrainingPlanWeek) TrainingPlanLoad {
	var load TrainingPlanLoad

	if len(weeks) == 0 {
		return load
	}

	load.LowestWeeklyLoad = -1

	for index := range weeks {
		week := &weeks[index]
		load.TotalPlannedLoad += week.PlannedLoad

		if week.PlannedLoad > load.PeakWeeklyLoad {
			load.PeakWeeklyLoad = week.PlannedLoad
		}

		if load.LowestWeeklyLoad == -1 || week.PlannedLoad < load.LowestWeeklyLoad {
			load.LowestWeeklyLoad = week.PlannedLoad
		}
	}

	load.AverageWeeklyLoad = round2(float64(load.TotalPlannedLoad) / float64(len(weeks)))
	load.ProgressionPercent = plannedProgressionPercent(weeks)
	load.FinalWeekDeloadPercent = plannedFinalDeloadPercent(weeks)
	load.WeeklyLoadTrend = weeklyLoadTrend(weeks)

	return load
}

func summarizeTrainingPlanHistory(activities []Activity, options TrainingPlanOptions) TrainingPlanHistory {
	var history TrainingPlanHistory

	weeks := map[string]trainingPlanHistoryWeek{}

	for index := range activities {
		activity := &activities[index]

		if !IsCyclingActivityType(activity.Type) {
			continue
		}

		date := activityDate(activity)
		if date == "" || !dateWithinRange(date, options.HistoryStartDate, options.HistoryEndDate) {
			continue
		}

		parsed, err := time.Parse(analysisDateLayout, date)
		if err != nil {
			continue
		}

		key := isoWeekKey(parsed)
		weekStart := isoWeekStart(parsed)
		weekEnd := weekStart.AddDate(0, 0, planWeekDays-1)
		week := weeks[key]
		week.load += activity.TrainingLoad
		week.seconds += activity.MovingTime
		week.sessions++
		week.startDate = weekStart.Format(analysisDateLayout)
		week.endDate = weekEnd.Format(analysisDateLayout)
		week.intensity.addIfNonZero(activity.Intensity)
		week.decoupling.addIfNonZero(activity.Decoupling)

		if activity.Intensity >= highIntensityThreshold {
			week.highIntensityDays++
		}

		if activity.MovingTime >= longEnduranceSeconds {
			week.longEnduranceSessions++
		}

		weeks[key] = week
	}

	finishTrainingPlanHistory(&history, weeks)
	addCurrentTrainingStateToHistory(&history, activities, options)

	return history
}

func finishTrainingPlanHistory(history *TrainingPlanHistory, weeks map[string]trainingPlanHistoryWeek) {
	keys := sortedHistoryWeekKeys(weeks)
	history.CompletedWeeks = len(keys)

	if len(keys) == 0 {
		return
	}

	var (
		totalLoad    int
		totalSeconds int
	)

	for _, key := range keys {
		week := weeks[key]
		totalLoad += week.load
		totalSeconds += week.seconds

		if week.load > history.PeakWeeklyLoad {
			history.PeakWeeklyLoad = week.load
		}
	}

	history.AverageWeeklyLoad = round2(float64(totalLoad) / float64(len(keys)))
	history.AverageWeeklyHours = round2(float64(totalSeconds) / secondsPerHour / float64(len(keys)))
	history.RecentWeeklyLoad = weeks[keys[len(keys)-1]].load
	history.Weeks = completedHistoryWeeks(keys, weeks, history.AverageWeeklyLoad)
}

func completedHistoryWeeks(keys []string, weeks map[string]trainingPlanHistoryWeek, averageWeeklyLoad float64) []TrainingPlanCompletedWeek {
	completed := make([]TrainingPlanCompletedWeek, 0, len(keys))

	for _, key := range keys {
		week := weeks[key]
		completed = append(completed, TrainingPlanCompletedWeek{
			ISOWeek:               key,
			StartDate:             week.startDate,
			EndDate:               week.endDate,
			Load:                  week.load,
			MovingTimeSecs:        week.seconds,
			MovingTimeHours:       round2(float64(week.seconds) / secondsPerHour),
			Sessions:              week.sessions,
			HighIntensityDays:     week.highIntensityDays,
			LongEnduranceSessions: week.longEnduranceSessions,
			MeanIntensity:         round3(week.intensity.average()),
			MeanDecoupling:        round2(week.decoupling.average()),
			Tolerance:             completedWeekTolerance(week.load, averageWeeklyLoad),
		})
	}

	return completed
}

func completedWeekTolerance(load int, averageWeeklyLoad float64) string {
	if averageWeeklyLoad == 0 {
		return unknownState
	}

	if float64(load) > averageWeeklyLoad*planBuildProgressionRatio {
		return planAlignmentAboveAverage
	}

	if float64(load) < averageWeeklyLoad*planBelowToleranceRatio {
		return planAlignmentBelow
	}

	return planAlignmentWithin
}

func addCurrentTrainingStateToHistory(history *TrainingPlanHistory, activities []Activity, options TrainingPlanOptions) {
	analysis := AnalyzeCyclingActivities(activities, AnalysisOptions{
		StartDate: options.HistoryStartDate,
		EndDate:   options.HistoryEndDate,
	})

	history.CurrentStateLabel = analysis.State.StateLabel
	history.CurrentLoadPressure = analysis.State.LoadPressure
	history.CurrentCTL = analysis.Load.LatestCTL
	history.CurrentATL = analysis.Load.LatestATL
	history.CurrentTSB = analysis.Load.LatestForm
	history.CurrentDecoupling = analysis.Durability.AverageDecoupling
	history.CurrentHotSessions = analysis.Environment.HotSessions
	history.CurrentStateSource = analysis.State.Source
}

func plannedLoadAlignment(history *TrainingPlanHistory, load *TrainingPlanLoad) string {
	if history.CompletedWeeks == 0 || load.AverageWeeklyLoad == 0 {
		return unknownState
	}

	if load.PeakWeeklyLoad > 0 && history.PeakWeeklyLoad > 0 &&
		float64(load.PeakWeeklyLoad) > float64(history.PeakWeeklyLoad)*planAbovePeakRatio {
		return planAlignmentAbovePeak
	}

	if load.AverageWeeklyLoad < history.AverageWeeklyLoad*planBelowToleranceRatio {
		return planAlignmentBelow
	}

	if load.AverageWeeklyLoad > history.AverageWeeklyLoad*planBuildProgressionRatio {
		return planAlignmentAboveAverage
	}

	return planAlignmentWithin
}

func trainingPlanWarnings(analysis *TrainingPlanAnalysis) []string {
	var warnings []string

	if analysis.History.CurrentStateLabel == stateLabelLoadPressure && analysis.Sessions.HighIntensitySessions > 1 {
		warnings = append(warnings, "current load pressure with multiple planned high-intensity sessions; make intensity conditional")
	}

	if analysis.History.PlannedLoadAlignment == planAlignmentAbovePeak {
		warnings = append(warnings, "planned peak load exceeds recent peak tolerance")
	}

	return warnings
}

func trainingPlanSportAnchors(settings *SportSettings, activities []Activity) TrainingPlanSportAnchors {
	if settings != nil {
		anchors := TrainingPlanSportAnchors{
			FTP:       settings.FTP,
			IndoorFTP: settings.IndoorFTP,
			LTHR:      settings.LTHR,
			MaxHR:     settings.MaxHR,
			WPrime:    settings.WPrime,
			PMax:      settings.PMax,
			Source:    "",
		}

		if anchors.FTP != 0 || anchors.IndoorFTP != 0 || anchors.LTHR != 0 ||
			anchors.MaxHR != 0 || anchors.WPrime != 0 || anchors.PMax != 0 {
			anchors.Source = "sport_settings"

			return anchors
		}
	}

	var (
		anchors    TrainingPlanSportAnchors
		latestDate string
	)

	for index := range activities {
		activity := &activities[index]
		if !IsCyclingActivityType(activity.Type) {
			continue
		}

		date := activityDate(activity)
		if date < latestDate {
			continue
		}

		latestDate = date
		anchors.FTP = activity.FTP
		anchors.LTHR = activity.LTHR
		anchors.MaxHR = activity.AthleteMaxHR
		anchors.WPrime = activity.WPrime
		anchors.PMax = activity.PMax
		anchors.Source = "latest_activity_model_fields"
	}

	return anchors
}

func trainingPlanTargetStatus(
	events []Event,
	options TrainingPlanOptions,
	history *TrainingPlanHistory,
) TrainingPlanEventTargetStatus {
	var status TrainingPlanEventTargetStatus
	status.Source = "calendar_event_targets"

	for index := range events {
		event := &events[index]
		date := eventDate(event)

		if date == "" || !dateWithinRange(date, options.PlanStartDate, options.PlanEndDate) {
			continue
		}

		if !isTargetEventCategory(event.Category) {
			continue
		}

		status.TargetEvents++
		if isRaceEventCategory(event.Category) {
			status.RaceEvents++
		}

		if status.NextEventDate == "" || date < status.NextEventDate {
			status.NextEventDate = date
		}

		status.Events = append(status.Events, TrainingPlanTargetEvent{
			Date:           date,
			Name:           event.Name,
			Category:       event.Category,
			Target:         event.Target,
			LoadTarget:     event.LoadTarget,
			TimeTarget:     event.TimeTarget,
			DistanceTarget: event.DistanceTarget,
		})
	}

	status.UpcomingEvents = len(status.Events)
	sort.Slice(status.Events, func(left, right int) bool {
		return status.Events[left].Date < status.Events[right].Date
	})

	status.Readiness = targetReadiness(&status, history)

	return status
}

func isTargetEventCategory(category string) bool {
	normalized := normalizedCategory(category)

	return normalized == planTargetCategory || isRaceEventCategory(normalized)
}

func isRaceEventCategory(category string) bool {
	normalized := normalizedCategory(category)

	return normalized == planTargetRaceA || normalized == planTargetRaceB || normalized == planTargetRaceC
}

func targetReadiness(status *TrainingPlanEventTargetStatus, history *TrainingPlanHistory) string {
	if status.UpcomingEvents == 0 {
		return "no_target_events"
	}

	if history.CurrentStateLabel == stateLabelLoadPressure ||
		history.PlannedLoadAlignment == planAlignmentAbovePeak {
		return "at_risk"
	}

	return "on_track"
}

func trainingPlanForecast(analysis *TrainingPlanAnalysis) TrainingPlanForecast {
	var forecast TrainingPlanForecast
	forecast.StartCTL = analysis.History.CurrentCTL
	forecast.StartATL = analysis.History.CurrentATL
	forecast.StartTSB = initialForecastTSB(&analysis.History)
	forecast.LowestTSB = forecast.StartTSB
	forecast.HighestTSB = forecast.StartTSB
	forecast.Source = "planned_load_impulse_model"

	if forecast.StartCTL == 0 && forecast.StartATL == 0 {
		return forecast
	}

	start, startErr := time.Parse(analysisDateLayout, analysis.Scope.PlanStartDate)
	end, endErr := time.Parse(analysisDateLayout, analysis.Scope.PlanEndDate)

	if startErr != nil || endErr != nil || end.Before(start) {
		return forecast
	}

	loadByDate := plannedDailyLoad(analysis.PlannedSessions)
	ctl := forecast.StartCTL
	atl := forecast.StartATL

	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		date := current.Format(analysisDateLayout)
		load := loadByDate[date]

		ctl = ewmaLoadStep(ctl, float64(load), planForecastCtlTauDays)
		atl = ewmaLoadStep(atl, float64(load), planForecastAtlTauDays)
		tsb := round2(ctl - atl)

		forecast.Points = append(forecast.Points, TrainingPlanForecastPoint{
			Date:        date,
			PlannedLoad: load,
			CTL:         round2(ctl),
			ATL:         round2(atl),
			TSB:         tsb,
		})

		if tsb < forecast.LowestTSB {
			forecast.LowestTSB = tsb
		}

		if tsb > forecast.HighestTSB {
			forecast.HighestTSB = tsb
		}
	}

	forecast.EndCTL = round2(ctl)
	forecast.EndATL = round2(atl)
	forecast.EndTSB = round2(ctl - atl)
	forecast.Status = forecastStatus(forecast.LowestTSB)

	return forecast
}

func initialForecastTSB(history *TrainingPlanHistory) float64 {
	if history.CurrentTSB != 0 {
		return history.CurrentTSB
	}

	if history.CurrentCTL == 0 && history.CurrentATL == 0 {
		return 0
	}

	return round2(history.CurrentCTL - history.CurrentATL)
}

func plannedDailyLoad(sessions []TrainingPlanSession) map[string]int {
	loadByDate := map[string]int{}

	for index := range sessions {
		session := &sessions[index]
		if session.Date == "" {
			continue
		}

		loadByDate[session.Date] += session.TrainingLoad
	}

	return loadByDate
}

func ewmaLoadStep(previous, todayLoad float64, tauDays int) float64 {
	if tauDays <= 0 {
		return todayLoad
	}

	return previous + ((todayLoad - previous) / float64(tauDays))
}

func forecastStatus(lowestTSB float64) string {
	if lowestTSB <= planForecastRedTSB {
		return planForecastStatusRed
	}

	if lowestTSB <= planForecastWatchTSB {
		return planForecastStatusWatch
	}

	return "stable"
}

func trainingPlanDayAdjustments(
	analysis *TrainingPlanAnalysis,
	wellness *WellnessAnalysis,
) []TrainingPlanDayAdjustment {
	rules := make([]TrainingPlanDayAdjustment, 0, len(analysis.PlannedSessions))
	forecastByDate := map[string]TrainingPlanForecastPoint{}

	for index := range analysis.Forecast.Points {
		point := analysis.Forecast.Points[index]
		forecastByDate[point.Date] = point
	}

	for index := range analysis.PlannedSessions {
		session := analysis.PlannedSessions[index]
		if normalizedCategory(session.Category) != planWorkoutCategory {
			continue
		}

		if point, ok := forecastByDate[session.Date]; ok {
			if rule, valid := adjustmentRuleForForecast(&session, point); valid {
				rules = append(rules, rule)
			}
		}

		if rule, valid := adjustmentRuleForDurability(&session, analysis.History.CurrentDecoupling); valid {
			rules = append(rules, rule)
		}

		if rule, valid := adjustmentRuleForHeat(&session, analysis.History.CurrentHotSessions); valid {
			rules = append(rules, rule)
		}

		if rule, valid := adjustmentRuleForWellness(&session, wellness); valid {
			rules = append(rules, rule)
		}

		if rule, valid := adjustmentRuleForSleep(&session, wellness); valid {
			rules = append(rules, rule)
		}

		if rule, valid := adjustmentRuleForRestingHR(&session, wellness); valid {
			rules = append(rules, rule)
		}
	}

	return rules
}

func adjustmentRuleForForecast(
	session *TrainingPlanSession,
	point TrainingPlanForecastPoint,
) (TrainingPlanDayAdjustment, bool) {
	if !isIntensityClassification(session.Classification) {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if point.TSB <= planForecastRedTSB {
		return TrainingPlanDayAdjustment{
			Date:           session.Date,
			SessionName:    session.Name,
			Classification: session.Classification,
			Severity:       "red",
			Condition:      "forecast_tsb_red",
			Action:         "replace with aerobic session or reduce load by 20%",
			Source:         "planned_load_impulse_model",
		}, true
	}

	if point.TSB <= planForecastWatchTSB {
		return TrainingPlanDayAdjustment{
			Date:           session.Date,
			SessionName:    session.Name,
			Classification: session.Classification,
			Severity:       "watch",
			Condition:      "forecast_tsb_watch",
			Action:         "make key intensity conditional on readiness",
			Source:         "planned_load_impulse_model",
		}, true
	}

	return emptyTrainingPlanDayAdjustment(), false
}

func adjustmentRuleForDurability(
	session *TrainingPlanSession,
	meanDecoupling float64,
) (TrainingPlanDayAdjustment, bool) {
	if session.Classification != plannedClassificationLong {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if meanDecoupling < planDriftWatchThreshold {
		return emptyTrainingPlanDayAdjustment(), false
	}

	return TrainingPlanDayAdjustment{
		Date:           session.Date,
		SessionName:    session.Name,
		Classification: session.Classification,
		Severity:       "watch",
		Condition:      "recent_decoupling_high",
		Action:         "cap endurance duration and monitor drift every 30 minutes",
		Source:         "recent_completed_sessions",
	}, true
}

func adjustmentRuleForHeat(
	session *TrainingPlanSession,
	hotSessions int,
) (TrainingPlanDayAdjustment, bool) {
	if hotSessions < planHeatSessionWatch {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if session.Classification != plannedClassificationLong && session.Classification != plannedClassificationZ2 {
		return emptyTrainingPlanDayAdjustment(), false
	}

	return TrainingPlanDayAdjustment{
		Date:           session.Date,
		SessionName:    session.Name,
		Classification: session.Classification,
		Severity:       "watch",
		Condition:      "recent_heat_load",
		Action:         "start early, cap power by one zone if heat rises",
		Source:         "activity_environment_fields",
	}, true
}

func adjustmentRuleForWellness(
	session *TrainingPlanSession,
	wellness *WellnessAnalysis,
) (TrainingPlanDayAdjustment, bool) {
	if wellness == nil {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if !session.KeySession {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if wellness.State.State != planWellnessStateWatch && wellness.State.State != planWellnessStateRed {
		return emptyTrainingPlanDayAdjustment(), false
	}

	severity := strings.ToLower(wellness.State.State)

	return TrainingPlanDayAdjustment{
		Date:           session.Date,
		SessionName:    session.Name,
		Classification: session.Classification,
		Severity:       severity,
		Condition:      "wellness_state_" + strings.ToLower(wellness.State.State),
		Action:         "switch to aerobic if morning readiness is not improved",
		Source:         wellness.State.Source,
	}, true
}

func adjustmentRuleForSleep(
	session *TrainingPlanSession,
	wellness *WellnessAnalysis,
) (TrainingPlanDayAdjustment, bool) {
	if wellness == nil || !session.KeySession || wellness.Sleep.Latest == 0 {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if wellness.Sleep.Latest < sleepRedScore {
		return TrainingPlanDayAdjustment{
			Date:           session.Date,
			SessionName:    session.Name,
			Classification: session.Classification,
			Severity:       planForecastStatusRed,
			Condition:      "sleep_score_red",
			Action:         "replace intensity with recovery unless sleep rebounds",
			Source:         "wellness_sleep",
		}, true
	}

	if wellness.Sleep.Latest < sleepWatchScore {
		return TrainingPlanDayAdjustment{
			Date:           session.Date,
			SessionName:    session.Name,
			Classification: session.Classification,
			Severity:       planForecastStatusWatch,
			Condition:      "sleep_score_watch",
			Action:         "cap first hard block and continue only if RPE is normal",
			Source:         "wellness_sleep",
		}, true
	}

	return emptyTrainingPlanDayAdjustment(), false
}

func adjustmentRuleForRestingHR(
	session *TrainingPlanSession,
	wellness *WellnessAnalysis,
) (TrainingPlanDayAdjustment, bool) {
	if wellness == nil || !session.KeySession {
		return emptyTrainingPlanDayAdjustment(), false
	}

	if wellness.RestingHR.Delta >= restingHRRedDelta {
		return TrainingPlanDayAdjustment{
			Date:           session.Date,
			SessionName:    session.Name,
			Classification: session.Classification,
			Severity:       planForecastStatusRed,
			Condition:      "resting_hr_delta_red",
			Action:         "skip intensity and keep only easy aerobic load",
			Source:         "wellness_resting_hr",
		}, true
	}

	if wellness.RestingHR.Delta >= restingHRWatchDelta {
		return TrainingPlanDayAdjustment{
			Date:           session.Date,
			SessionName:    session.Name,
			Classification: session.Classification,
			Severity:       planForecastStatusWatch,
			Condition:      "resting_hr_delta_watch",
			Action:         "make intensity conditional and extend warmup check",
			Source:         "wellness_resting_hr",
		}, true
	}

	return emptyTrainingPlanDayAdjustment(), false
}

func emptyTrainingPlanDayAdjustment() TrainingPlanDayAdjustment {
	return TrainingPlanDayAdjustment{
		Date:           "",
		SessionName:    "",
		Classification: "",
		Severity:       "",
		Condition:      "",
		Action:         "",
		Source:         "",
	}
}

func isIntensityClassification(classification string) bool {
	return classification == plannedClassificationHigh || classification == plannedClassificationTempo
}

func trainingPlanDecisionGuidance(
	analysis *TrainingPlanAnalysis,
	wellness *WellnessAnalysis,
	adaptation *CyclingAdaptationAnalysis,
) TrainingPlanDecisionGuidance {
	var decision TrainingPlanDecisionGuidance
	decision.Source = "local_decision_engine_v1"
	decision.PrimaryDirective = primaryDirective(analysis, wellness, adaptation)
	decision.ADEScore = planDecisionBaseScore

	if analysis.Forecast.Status == planForecastStatusRed {
		decision.ADEScore -= planDecisionRedPenalty
		decision.Penalties = append(decision.Penalties, "forecast_red")
	} else if analysis.Forecast.Status == planForecastStatusWatch {
		decision.ADEScore -= planDecisionWatchPenalty
		decision.Penalties = append(decision.Penalties, "forecast_watch")
	}

	if analysis.History.CurrentStateLabel == stateLabelLoadPressure {
		decision.ADEScore -= planDecisionLoadPenalty
		decision.Penalties = append(decision.Penalties, "load_pressure")
	}

	if analysis.History.PlannedLoadAlignment == planAlignmentAbovePeak {
		decision.ADEScore -= planDecisionPeakPenalty
		decision.Penalties = append(decision.Penalties, "above_recent_peak")
	}

	if wellness != nil {
		if wellness.State.State == planWellnessStateRed {
			decision.ADEScore -= planDecisionRedPenalty
			decision.Penalties = append(decision.Penalties, "wellness_red")
		}

		if wellness.State.State == planWellnessStateWatch {
			decision.ADEScore -= planDecisionWatchPenalty
			decision.Penalties = append(decision.Penalties, "wellness_watch")
		}
	}

	if adaptation != nil && adaptation.SystemStatus.Status == adaptationStatusDeclining {
		decision.ADEScore -= planDecisionAdaptPenalty
		decision.Penalties = append(decision.Penalties, "adaptation_declining")
	}

	if decision.ADEScore < 0 {
		decision.ADEScore = 0
	}

	decision.TrainingGuidance = decisionGuidanceText(decision.PrimaryDirective)

	return decision
}

func primaryDirective(
	analysis *TrainingPlanAnalysis,
	wellness *WellnessAnalysis,
	adaptation *CyclingAdaptationAnalysis,
) string {
	if analysis.History.CurrentStateLabel == stateLabelLoadPressure ||
		analysis.Forecast.Status == planForecastStatusRed {
		return planDirectiveRecovery
	}

	if wellness != nil && wellness.State.State == planWellnessStateRed {
		return planDirectiveRecovery
	}

	if analysis.TargetStatus.Readiness == "at_risk" {
		return planDirectiveProtectTarget
	}

	if adaptation != nil && adaptation.SystemStatus.Status == adaptationStatusDeclining {
		return planDirectiveProtectTarget
	}

	if analysis.Forecast.Status == planForecastStatusWatch ||
		(wellness != nil && wellness.State.State == planWellnessStateWatch) {
		return planDirectiveConditional
	}

	return planDirectiveBuild
}

func decisionGuidanceText(directive string) string {
	switch directive {
	case planDirectiveRecovery:
		return "Protect recovery bandwidth, reduce intensity density, and preserve only one key stimulus."
	case planDirectiveProtectTarget:
		return "Keep race-specific sessions while trimming non-essential load around target events."
	case planDirectiveConditional:
		return "Continue build structure, but gate intensity by morning readiness and forecasted form."
	default:
		return "Continue progressive build with normal monitoring and recovery spacing."
	}
}

func sortedTrainingPlanWeeks(weekMap map[string]*TrainingPlanWeek) []TrainingPlanWeek {
	keys := make([]string, 0, len(weekMap))

	for key := range weekMap {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	weeks := make([]TrainingPlanWeek, 0, len(keys))

	for _, key := range keys {
		weeks = append(weeks, *weekMap[key])
	}

	return weeks
}

func sortTrainingPlanSessions(sessions []TrainingPlanSession) {
	sort.Slice(sessions, func(left, right int) bool {
		return sessions[left].Date < sessions[right].Date
	})
}

func assignSessionWeekRoles(sessions []TrainingPlanSession, weeks []TrainingPlanWeek) {
	roles := map[string]string{}

	for index := range weeks {
		week := &weeks[index]
		roles[week.ISOWeek] = week.Role
	}

	for index := range sessions {
		parsed, err := time.Parse(analysisDateLayout, sessions[index].Date)
		if err != nil {
			continue
		}

		sessions[index].WeekRole = roles[isoWeekKey(parsed)]
	}
}

func sortedHistoryWeekKeys(weeks map[string]trainingPlanHistoryWeek) []string {
	keys := make([]string, 0, len(weeks))

	for key := range weeks {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func peakTrainingPlanWeekIndex(weeks []TrainingPlanWeek) int {
	var (
		peakIndex int
		peakLoad  int
	)

	for index := range weeks {
		week := &weeks[index]

		if week.PlannedLoad > peakLoad {
			peakLoad = week.PlannedLoad
			peakIndex = index
		}
	}

	return peakIndex
}

func plannedProgressionPercent(weeks []TrainingPlanWeek) float64 {
	if len(weeks) < planMinimumTrendWeeks || weeks[0].PlannedLoad == 0 {
		return 0
	}

	var peak int

	for index := range weeks {
		week := &weeks[index]
		if week.PlannedLoad > peak {
			peak = week.PlannedLoad
		}
	}

	return round2(float64(peak-weeks[0].PlannedLoad) * percentScale / float64(weeks[0].PlannedLoad))
}

func plannedFinalDeloadPercent(weeks []TrainingPlanWeek) float64 {
	if len(weeks) < planMinimumTrendWeeks {
		return 0
	}

	previous := weeks[len(weeks)-planMinimumTrendWeeks].PlannedLoad
	if previous == 0 || weeks[len(weeks)-1].PlannedLoad >= previous {
		return 0
	}

	return round2(float64(previous-weeks[len(weeks)-1].PlannedLoad) * percentScale / float64(previous))
}

func weeklyLoadTrend(weeks []TrainingPlanWeek) string {
	if isBuildWithDeload(weeks) {
		return "progressive_then_deload"
	}

	if isProgressiveBuild(weeks) {
		return "increasing"
	}

	if endsWithDeload(weeks) {
		return "declining"
	}

	return "stable_or_mixed"
}

func isBuildWithDeload(weeks []TrainingPlanWeek) bool {
	if len(weeks) < planBuildWithDeloadWeeks {
		return false
	}

	lastIndex := len(weeks) - 1

	return weeks[1].PlannedLoad >= weeks[0].PlannedLoad &&
		weeks[2].PlannedLoad >= weeks[1].PlannedLoad &&
		isDeloadWeek(weeks[lastIndex].PlannedLoad, weeks[lastIndex-1].PlannedLoad)
}

func isProgressiveBuild(weeks []TrainingPlanWeek) bool {
	if len(weeks) < planMinimumTrendWeeks {
		return false
	}

	for index := 1; index < len(weeks); index++ {
		if weeks[index].PlannedLoad < weeks[index-1].PlannedLoad {
			return false
		}
	}

	return true
}

func endsWithDeload(weeks []TrainingPlanWeek) bool {
	if len(weeks) < planMinimumTrendWeeks {
		return false
	}

	lastIndex := len(weeks) - 1

	return isDeloadWeek(weeks[lastIndex].PlannedLoad, weeks[lastIndex-1].PlannedLoad)
}

func totalPlannedWeekLoad(weeks []TrainingPlanWeek) int {
	var total int

	for index := range weeks {
		total += weeks[index].PlannedLoad
	}

	return total
}

func isBuildProgressionWeek(current, previous int) bool {
	if previous == 0 {
		return current > 0
	}

	return float64(current) >= float64(previous)*planBuildProgressionRatio
}

func isDeloadWeek(current, previous int) bool {
	if previous == 0 {
		return false
	}

	return float64(current) <= float64(previous)*planDeloadRatio
}

func eventDate(event *Event) string {
	if len(event.StartDateLocal) >= len(analysisDateLayout) {
		return event.StartDateLocal[:len(analysisDateLayout)]
	}

	return ""
}

func dateWithinRange(date, startDate, endDate string) bool {
	if startDate != "" && date < startDate {
		return false
	}

	if endDate != "" && date > endDate {
		return false
	}

	return true
}

func isoWeekStart(date time.Time) time.Time {
	var weekday int
	weekday = int(date.Weekday())

	if weekday == 0 {
		weekday = planWeekDays
	}

	return date.AddDate(0, 0, -(weekday - 1))
}

func isoWeekKey(date time.Time) string {
	year, week := date.ISOWeek()

	return fmt.Sprintf("%04d-W%02d", year, week)
}

func normalizedCategory(category string) string {
	return strings.ToUpper(category)
}

func inferredPlanStart(options TrainingPlanOptions, weeks []TrainingPlanWeek) string {
	if options.PlanStartDate != "" || len(weeks) == 0 {
		return options.PlanStartDate
	}

	return weeks[0].StartDate
}

func inferredPlanEnd(options TrainingPlanOptions, weeks []TrainingPlanWeek) string {
	if options.PlanEndDate != "" || len(weeks) == 0 {
		return options.PlanEndDate
	}

	return weeks[len(weeks)-1].EndDate
}
