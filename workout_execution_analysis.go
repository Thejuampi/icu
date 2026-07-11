package icu

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	workoutMatchConfidenceHigh     = "high"
	workoutMatchConfidenceModerate = "moderate"
	workoutMatchConfidenceLow      = "low"
	workoutMatchConfidenceNone     = "none"
	workoutCompletionMatched       = "matched"
	workoutCompletionPartial       = "partially_completed"
	workoutCompletionShortened     = "shortened"
	workoutCompletionPlanMissing   = "plan_missing"
	workoutDateLayout              = "2006-01-02"
	workoutTimeLayout              = "2006-01-02T15:04:05"
	workoutDefaultMatchWindowHours = 24
	workoutMatchCategoryWeight     = 40
	workoutMatchTypeWeight         = 20
	workoutMatchTimeWeight         = 20
	workoutMatchDurationWeight     = 10
	workoutMatchLoadWeight         = 10
	workoutMatchNameWeight         = 8
	workoutMatchStructuredWeight   = 5
	workoutMatchedThreshold        = 85
	workoutPartialThreshold        = 60
	workoutHighConfidenceScore     = 75
	workoutModerateConfidenceScore = 50
)

type WorkoutExecutionInputs struct {
	Activity      *Activity      `json:"activity,omitempty"`
	Streams       StreamData     `json:"-"`
	Intervals     *IntervalsDTO  `json:"intervals,omitempty"`
	Events        []Event        `json:"events,omitempty"`
	SportSettings *SportSettings `json:"sportSettings,omitempty"`
	PowerModel    *PowerModel    `json:"powerModel,omitempty"`
}

type WorkoutExecutionOptions struct {
	ExplicitEventID  int `json:"explicitEventId,omitempty"`
	MatchWindowHours int `json:"matchWindowHours,omitempty"`
}

type WorkoutExecutionAnalysis struct {
	Scope      WorkoutExecutionScope      `json:"scope"`
	Activity   CyclingSession             `json:"activity"`
	Match      WorkoutEventMatch          `json:"match"`
	Plan       WorkoutPlanSummary         `json:"plan"`
	Execution  WorkoutExecutionSummary    `json:"execution"`
	Comparison WorkoutExecutionComparison `json:"comparison"`
	WBal       *WorkoutWBalAnalysis       `json:"wbal,omitempty"`
	Warnings   []string                   `json:"warnings,omitempty"`
}

type WorkoutWBalAnalysis struct {
	ModelSource               string             `json:"modelSource"`
	CriticalPower             int                `json:"criticalPower"`
	WPrime                    int                `json:"wPrime"`
	OriginalDepletionJoules   int                `json:"originalDepletionJoules"`
	OriginalDepletionPct      float64            `json:"originalDepletionPct"`
	RecomputedDepletionJoules int                `json:"recomputedDepletionJoules"`
	RecomputedDepletionPct    float64            `json:"recomputedDepletionPct"`
	Artifacts                 []FlywheelArtifact `json:"artifacts,omitempty"`
	DataQualityWarnings       []string           `json:"dataQualityWarnings,omitempty"`
}

type WorkoutExecutionScope struct {
	ActivityID       string `json:"activityId,omitempty"`
	EventID          int    `json:"eventId,omitempty"`
	MatchWindowHours int    `json:"matchWindowHours,omitempty"`
}

type WorkoutEventMatchOptions struct {
	ExplicitEventID  int
	MatchWindowHours int
}

type WorkoutEventMatch struct {
	EventID     int                     `json:"eventId,omitempty"`
	Name        string                  `json:"name,omitempty"`
	Confidence  string                  `json:"confidence"`
	Score       float64                 `json:"score,omitempty"`
	Reasons     []string                `json:"reasons,omitempty"`
	Alternates  []WorkoutEventCandidate `json:"alternates,omitempty"`
	MatchedDate string                  `json:"matchedDate,omitempty"`
}

type WorkoutEventCandidate struct {
	EventID int     `json:"eventId,omitempty"`
	Name    string  `json:"name,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

type WorkoutPlanSummary struct {
	Available       bool                 `json:"available"`
	EventID         int                  `json:"eventId,omitempty"`
	Name            string               `json:"name,omitempty"`
	DurationSeconds int                  `json:"durationSeconds,omitempty"`
	TrainingLoad    int                  `json:"trainingLoad,omitempty"`
	Intensity       float64              `json:"intensity,omitempty"`
	Steps           []PlannedWorkoutStep `json:"steps,omitempty"`
	WorkSteps       int                  `json:"workSteps,omitempty"`
	RecoverySteps   int                  `json:"recoverySteps,omitempty"`
}

type WorkoutExecutionSummary struct {
	Micro          ActivityMicroAnalysis `json:"micro"`
	IntervalsFound int                   `json:"intervalsFound"`
	StreamsFound   []string              `json:"streamsFound,omitempty"`
}

type WorkoutExecutionComparison struct {
	Completion     string                  `json:"completion"`
	Duration       WorkoutMetricComparison `json:"duration"`
	TrainingLoad   WorkoutMetricComparison `json:"trainingLoad"`
	Intensity      WorkoutMetricComparison `json:"intensity"`
	RepCount       WorkoutCountComparison  `json:"repCount"`
	StepComparison []WorkoutStepComparison `json:"steps,omitempty"`
	Repeatability  *RepeatabilityAnalysis  `json:"repeatability,omitempty"`
	ZoneAlignment  ZoneAlignmentAnalysis   `json:"zoneAlignment,omitempty"`
}

type WorkoutMetricComparison struct {
	Planned float64 `json:"planned"`
	Actual  float64 `json:"actual"`
	Delta   float64 `json:"delta"`
	Ratio   float64 `json:"ratio,omitempty"`
}

type WorkoutCountComparison struct {
	Planned  int `json:"planned"`
	Executed int `json:"executed"`
	Delta    int `json:"delta"`
}

type WorkoutStepComparison struct {
	Index                int     `json:"index"`
	Kind                 string  `json:"kind"`
	PlannedDuration      int     `json:"plannedDurationSeconds,omitempty"`
	ExecutedDuration     int     `json:"executedDurationSeconds,omitempty"`
	PlannedTarget        float64 `json:"plannedTarget,omitempty"`
	ExecutedAveragePower float64 `json:"executedAveragePower,omitempty"`
	ExecutedAverageHR    float64 `json:"executedAverageHr,omitempty"`
	TargetDeviation      float64 `json:"targetDeviation,omitempty"`
	Status               string  `json:"status"`
}

type workoutEventScore struct {
	score   float64
	reasons []string
}

func AnalyzeWorkoutExecution(inputs WorkoutExecutionInputs, options WorkoutExecutionOptions) WorkoutExecutionAnalysis {
	if options.MatchWindowHours <= 0 {
		options.MatchWindowHours = workoutDefaultMatchWindowHours
	}

	var result WorkoutExecutionAnalysis
	result.Scope.MatchWindowHours = options.MatchWindowHours
	if inputs.Activity == nil {
		result.Match.Confidence = workoutMatchConfidenceNone
		result.Comparison.Completion = workoutCompletionPlanMissing
		result.Warnings = append(result.Warnings, "activity is missing")
		return result
	}

	result.Scope.ActivityID = inputs.Activity.ID
	result.Activity = cyclingSessionFromActivity(inputs.Activity)
	result.Match = MatchWorkoutEvent(inputs.Activity, inputs.Events, WorkoutEventMatchOptions(options))
	result.Scope.EventID = result.Match.EventID

	settings := inputs.SportSettings
	var ftp, lthr int
	if settings != nil {
		ftp = settings.FTP
		lthr = settings.LTHR
	}
	if ftp == 0 {
		ftp = inputs.Activity.FTP
	}
	if lthr == 0 {
		lthr = inputs.Activity.LTHR
	}

	result.Execution.Micro = AnalyzeActivityMicro(inputs.Activity, inputs.Streams, inputs.Intervals, ftp, lthr)
	if inputs.Intervals != nil {
		result.Execution.IntervalsFound = len(inputs.Intervals.Intervals)
	}
	result.Execution.StreamsFound = streamKeys(inputs.Streams)
	result.Warnings = append(result.Warnings, workoutInputWarnings(inputs, ftp, lthr)...)

	event := workoutMatchedEvent(result.Match.EventID, inputs.Events)
	result.Plan = workoutPlanSummary(event)
	result.Comparison = workoutComparison(inputs.Activity, event, &result.Plan, inputs.Intervals, &result.Execution.Micro, ftp)

	result.WBal = computeWorkoutWBal(inputs, &result.Activity)

	if result.Match.Confidence == workoutMatchConfidenceNone {
		result.Warnings = append(result.Warnings, "no planned workout event matched")
	}
	if result.Plan.Available && len(result.Plan.Steps) == 0 {
		result.Warnings = append(result.Warnings, "matched event has no structured workout steps")
	}

	return result
}

func computeWorkoutWBal(inputs WorkoutExecutionInputs, session *CyclingSession) *WorkoutWBalAnalysis {
	watts := streamValue(inputs.Streams, "watts")
	if len(watts) == 0 && inputs.PowerModel == nil {
		return nil
	}

	var globalModel PowerModel
	if inputs.PowerModel != nil {
		globalModel = *inputs.PowerModel
	}

	recomputedJoules, recomputedPct, detection, warnings := RecomputeWBalDepletion(
		watts,
		streamValue(inputs.Streams, "cadence"),
		streamValue(inputs.Streams, "heartrate"),
		globalModel,
		inputs.Activity,
	)

	cp := globalModel.CriticalPower
	wprime := globalModel.WPrime
	if cp == 0 && inputs.Activity != nil {
		cp = inputs.Activity.CriticalPower
	}
	if wprime == 0 && inputs.Activity != nil {
		wprime = inputs.Activity.WPrime
	}

	modelSource := "global"
	if globalModel.CriticalPower == 0 {
		modelSource = "activity"
	}

	return &WorkoutWBalAnalysis{
		ModelSource:               modelSource,
		CriticalPower:             cp,
		WPrime:                    wprime,
		OriginalDepletionJoules:   session.MaxWBalDepletion,
		OriginalDepletionPct:      session.WBalDepletionPct,
		RecomputedDepletionJoules: recomputedJoules,
		RecomputedDepletionPct:    recomputedPct,
		Artifacts:                 detection.Artifacts,
		DataQualityWarnings:       warnings,
	}
}

func MatchWorkoutEvent(activity *Activity, events []Event, options WorkoutEventMatchOptions) WorkoutEventMatch {
	var result WorkoutEventMatch
	result.Confidence = workoutMatchConfidenceNone
	if activity == nil {
		return result
	}
	if options.MatchWindowHours <= 0 {
		options.MatchWindowHours = workoutDefaultMatchWindowHours
	}

	var best WorkoutEventCandidate
	var alternates []WorkoutEventCandidate
	for i := range events {
		event := events[i]
		if options.ExplicitEventID > 0 && event.ID != options.ExplicitEventID {
			continue
		}
		eventScore := scoreWorkoutEvent(activity, &event, options)
		if eventScore.score <= 0 {
			continue
		}
		candidate := WorkoutEventCandidate{EventID: event.ID, Name: event.Name, Score: round2(eventScore.score)}
		if candidate.Score > best.Score {
			if best.EventID != 0 {
				alternates = append(alternates, best)
			}
			best = candidate
			result.Reasons = eventScore.reasons
		} else {
			alternates = append(alternates, candidate)
		}
	}

	if best.EventID == 0 {
		return result
	}

	result.EventID = best.EventID
	result.Name = best.Name
	result.Score = best.Score
	result.Confidence = workoutConfidence(best.Score)
	result.Alternates = alternates
	result.MatchedDate = activityLocalDate(activity)
	return result
}

func scoreWorkoutEvent(activity *Activity, event *Event, options WorkoutEventMatchOptions) workoutEventScore {
	var result workoutEventScore
	explicit := options.ExplicitEventID > 0 && event.ID == options.ExplicitEventID
	inWindow := withinWorkoutWindow(activity.StartDateLocal, event.StartDateLocal, options.MatchWindowHours)
	// Hard-require temporal proximity unless the user pinned an event ID.
	// Category/type/name alone must not match a workout days or weeks away.
	if !inWindow && !explicit {
		return result
	}
	scoreWorkoutIdentity(&result, activity, event, inWindow, explicit)
	scoreWorkoutSimilarity(&result, activity, event)
	return result
}

func scoreWorkoutIdentity(result *workoutEventScore, activity *Activity, event *Event, inWindow, explicit bool) {
	if event.Category == "WORKOUT" {
		result.score += workoutMatchCategoryWeight
		result.reasons = append(result.reasons, "category_workout")
	}
	if sameWorkoutType(activity.Type, event.Type) {
		result.score += workoutMatchTypeWeight
		result.reasons = append(result.reasons, "sport_type_match")
	}
	if inWindow {
		result.score += workoutMatchTimeWeight
		result.reasons = append(result.reasons, "time_window_match")
	}
	if explicit {
		result.score += workoutMatchCategoryWeight
		result.reasons = append(result.reasons, "explicit_event_id")
	}
}

func scoreWorkoutSimilarity(result *workoutEventScore, activity *Activity, event *Event) {
	if event.MovingTime > 0 && activity.MovingTime > 0 {
		result.score += similarityScore(float64(activity.MovingTime), float64(event.MovingTime), workoutMatchDurationWeight)
		result.reasons = append(result.reasons, "duration_similarity")
	}
	if event.TrainingLoad > 0 && activity.TrainingLoad > 0 {
		result.score += similarityScore(float64(activity.TrainingLoad), float64(event.TrainingLoad), workoutMatchLoadWeight)
		result.reasons = append(result.reasons, "load_similarity")
	}
	if workoutNameSimilarity(activity.Name, event.Name) {
		result.score += workoutMatchNameWeight
		result.reasons = append(result.reasons, "name_similarity")
	}
	if event.WorkoutDoc != nil || event.WorkoutFileBase64 != "" {
		result.score += workoutMatchStructuredWeight
		result.reasons = append(result.reasons, "structured_plan_available")
	}
}

func workoutComparison(activity *Activity, event *Event, plan *WorkoutPlanSummary, intervals *IntervalsDTO, micro *ActivityMicroAnalysis, ftp int) WorkoutExecutionComparison {
	var result WorkoutExecutionComparison
	result.Completion = workoutCompletionPlanMissing
	result.ZoneAlignment = micro.ZoneAlignment
	if micro.Intervals != nil {
		result.Repeatability = &micro.Intervals.Repeatability
	}
	if event == nil {
		return result
	}

	result.Duration = compareWorkoutMetric(float64(event.MovingTime), float64(activity.MovingTime))
	result.TrainingLoad = compareWorkoutMetric(float64(event.TrainingLoad), float64(activity.TrainingLoad))
	result.Intensity = compareWorkoutMetric(event.Intensity, activity.Intensity)
	plannedReps := countPlannedWorkSteps(plan.Steps)
	executedReps := countExecutedWorkIntervals(intervals)
	result.RepCount = WorkoutCountComparison{Planned: plannedReps, Executed: executedReps, Delta: executedReps - plannedReps}
	result.StepComparison = compareWorkoutSteps(plan.Steps, intervals, ftp)
	result.Completion = classifyWorkoutCompletion(result.Duration.Ratio, plannedReps, executedReps)
	return result
}

func workoutPlanSummary(event *Event) WorkoutPlanSummary {
	var result WorkoutPlanSummary
	if event == nil {
		return result
	}
	result.Available = true
	result.EventID = event.ID
	result.Name = event.Name
	result.DurationSeconds = event.MovingTime
	result.TrainingLoad = event.TrainingLoad
	result.Intensity = event.Intensity
	doc, err := DecodeWorkoutDoc(event.WorkoutDoc)
	if err == nil && doc != nil {
		result.Steps = ExpandWorkoutSteps(doc)
	}
	for i := range result.Steps {
		switch result.Steps[i].Kind {
		case plannedStepKindWork:
			result.WorkSteps++
		case plannedStepKindRecovery:
			result.RecoverySteps++
		}
	}
	return result
}

func compareWorkoutSteps(steps []PlannedWorkoutStep, intervals *IntervalsDTO, ftp int) []WorkoutStepComparison {
	if len(steps) == 0 {
		return nil
	}

	workIntervals := workoutWorkIntervals(intervals)
	result := make([]WorkoutStepComparison, 0, len(steps))
	var workIndex int
	for i := range steps {
		step := steps[i]
		comparison := WorkoutStepComparison{
			Index:           step.Index,
			Kind:            step.Kind,
			PlannedDuration: step.DurationSeconds,
			PlannedTarget:   plannedPowerWatts(step.Power, ftp),
			Status:          "not_matched",
		}
		if step.Kind == plannedStepKindWork && workIndex < len(workIntervals) {
			interval := workIntervals[workIndex]
			comparison.ExecutedDuration = interval.MovingTime
			comparison.ExecutedAveragePower = float64(interval.AvgPower)
			comparison.ExecutedAverageHR = float64(interval.AvgHR)
			comparison.TargetDeviation = round2(comparison.ExecutedAveragePower - comparison.PlannedTarget)
			comparison.Status = "matched"
			workIndex++
		}
		result = append(result, comparison)
	}
	return result
}

func plannedPowerWatts(target *WorkoutTarget, ftp int) float64 {
	if target == nil || ftp == 0 {
		return 0
	}
	value := targetReference(target)
	if strings.EqualFold(target.Units, "%ftp") {
		return round2(float64(ftp) * value / percentScale)
	}
	return value
}

func workoutInputWarnings(inputs WorkoutExecutionInputs, ftp, lthr int) []string {
	var warnings []string
	if len(inputs.Streams) == 0 {
		warnings = append(warnings, "streams are missing")
	}
	if inputs.Intervals == nil || len(inputs.Intervals.Intervals) == 0 {
		warnings = append(warnings, "intervals are missing")
	}
	if ftp == 0 {
		warnings = append(warnings, "ftp is missing")
	}
	if lthr == 0 {
		warnings = append(warnings, "lthr is missing")
	}
	return warnings
}

func workoutMatchedEvent(eventID int, events []Event) *Event {
	for i := range events {
		if events[i].ID == eventID {
			return &events[i]
		}
	}
	return nil
}

func compareWorkoutMetric(planned, actual float64) WorkoutMetricComparison {
	result := WorkoutMetricComparison{Planned: planned, Actual: actual, Delta: round2(actual - planned)}
	if planned > 0 {
		result.Ratio = round2(actual / planned * percentScale)
	}
	return result
}

func countPlannedWorkSteps(steps []PlannedWorkoutStep) int {
	var count int
	for i := range steps {
		if steps[i].Kind == plannedStepKindWork {
			count++
		}
	}
	return count
}

func countExecutedWorkIntervals(intervals *IntervalsDTO) int {
	return len(workoutWorkIntervals(intervals))
}

func workoutWorkIntervals(intervals *IntervalsDTO) []Interval {
	if intervals == nil {
		return nil
	}
	var result []Interval
	for i := range intervals.Intervals {
		if strings.EqualFold(intervals.Intervals[i].Type, "WORK") {
			result = append(result, intervals.Intervals[i])
		}
	}
	return result
}

func classifyWorkoutCompletion(durationRatio float64, plannedReps, executedReps int) string {
	if durationRatio >= workoutMatchedThreshold && (plannedReps == 0 || executedReps >= plannedReps) {
		return workoutCompletionMatched
	}
	if durationRatio >= workoutPartialThreshold || executedReps > 0 {
		return workoutCompletionPartial
	}
	return workoutCompletionShortened
}

func workoutConfidence(score float64) string {
	switch {
	case score >= workoutHighConfidenceScore:
		return workoutMatchConfidenceHigh
	case score >= workoutModerateConfidenceScore:
		return workoutMatchConfidenceModerate
	case score > 0:
		return workoutMatchConfidenceLow
	default:
		return workoutMatchConfidenceNone
	}
}

func similarityScore(actual, planned, weight float64) float64 {
	if actual <= 0 || planned <= 0 {
		return 0
	}
	diff := math.Abs(actual - planned)
	ratio := 1 - diff/math.Max(actual, planned)
	if ratio < 0 {
		return 0
	}
	return ratio * weight
}

func workoutNameSimilarity(activityName, eventName string) bool {
	activityParts := strings.Fields(strings.ToLower(activityName))
	event := strings.ToLower(eventName)
	var matches int
	for i := range activityParts {
		part := strings.Trim(activityParts[i], "-_:.,")
		if len(part) >= 3 && strings.Contains(event, part) {
			matches++
		}
	}
	return matches >= 2
}

func sameWorkoutType(activityType, eventType string) bool {
	if activityType == "" || eventType == "" {
		return false
	}
	if strings.EqualFold(activityType, eventType) {
		return true
	}
	return strings.EqualFold(activityType, "Virtual"+eventType) || strings.EqualFold(eventType, "Virtual"+activityType)
}

func withinWorkoutWindow(activityDate, eventDate string, hours int) bool {
	if activityDate == "" || eventDate == "" {
		return false
	}
	activityTime, err := parseWorkoutTime(activityDate)
	if err != nil {
		return activityLocalDate(&Activity{StartDateLocal: activityDate}) == eventLocalDate(eventDate)
	}
	eventTime, err := parseWorkoutTime(eventDate)
	if err != nil {
		return activityLocalDate(&Activity{StartDateLocal: activityDate}) == eventLocalDate(eventDate)
	}
	window := time.Duration(hours) * time.Hour
	return activityTime.Sub(eventTime) <= window && eventTime.Sub(activityTime) <= window
}

func parseWorkoutTime(value string) (time.Time, error) {
	if len(value) >= len(workoutTimeLayout) {
		parsed, err := time.Parse(workoutTimeLayout, value[:len(workoutTimeLayout)])
		if err != nil {
			return time.Time{}, fmt.Errorf("parse workout time: %w", err)
		}

		return parsed, nil
	}
	parsed, err := time.Parse(workoutDateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse workout date: %w", err)
	}

	return parsed, nil
}

func activityLocalDate(activity *Activity) string {
	if activity == nil || len(activity.StartDateLocal) < len(workoutDateLayout) {
		return ""
	}
	return activity.StartDateLocal[:len(workoutDateLayout)]
}

func eventLocalDate(value string) string {
	if len(value) < len(workoutDateLayout) {
		return ""
	}
	return value[:len(workoutDateLayout)]
}

func streamKeys(streams StreamData) []string {
	result := make([]string, 0, len(streams))
	for key := range streams {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
