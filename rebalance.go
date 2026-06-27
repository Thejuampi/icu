package icu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	RebalanceSchemaVersion = "icu.rebalance.v2"

	RebalanceActionCreate = "create"
	RebalanceActionUpdate = "update"
	RebalanceActionCancel = "cancel"

	RebalanceStatusPending = "pending"
	RebalanceStatusSkipped = "skipped"
	RebalanceStatusApplied = "applied"
	RebalanceStatusFailed  = "failed"

	rebalanceHashPrefixLength       = 12
	rebalanceMinutesPerHour         = 60
	rebalanceSecondsPerMinute       = 60
	rebalanceSecondsPerHour         = 3600
	rebalanceDescriptionStepMinutes = 5
	rebalanceRobustZLimit           = 3.5
	rebalanceMadScale               = 0.6745
	rebalanceSportZoneDivisor       = 100.0
	rebalanceWattsLowOffset         = 0.03
	rebalanceWattsLowFloor          = 0.5
	rebalanceJSONIndent             = "  "
	rebalanceStrategyTargetZ2       = "target-preserving-z2"
	rebalanceWorkoutTargetPower     = "POWER"
	rebalanceAllocationExplicit     = "explicit_equal"
)

type RebalanceInput struct {
	AthleteID     string                `json:"athleteId,omitempty"`
	GeneratedAt   string                `json:"generatedAt,omitempty"`
	NowDate       string                `json:"nowDate,omitempty"`
	Activities    []Activity            `json:"-"`
	Events        []Event               `json:"-"`
	SportSettings *SportSettings        `json:"-"`
	Wellness      *WellnessAnalysis     `json:"-"`
	Plan          *TrainingPlanAnalysis `json:"-"`
	Scope         RebalanceScope        `json:"scope"`
	Request       RebalanceRequest      `json:"request"`
	Constraints   RebalanceConstraints  `json:"constraints"`
}

type RebalanceProposalFile struct {
	SchemaVersion    string                     `json:"schemaVersion"`
	ProposalID       string                     `json:"proposalId"`
	GeneratedAt      string                     `json:"generatedAt,omitempty"`
	AthleteID        string                     `json:"athleteId,omitempty"`
	Scope            RebalanceScope             `json:"scope"`
	Request          RebalanceRequest           `json:"request"`
	Constraints      RebalanceConstraints       `json:"constraints"`
	Context          RebalanceContext           `json:"context"`
	Baseline         RebalanceEvaluation        `json:"baseline"`
	Options          []RebalanceOption          `json:"options"`
	SelectedOptionID string                     `json:"selectedOptionId"`
	Operations       []RebalanceOperation       `json:"operations"`
	Validation       RebalanceValidation        `json:"validation"`
	Apply            *RebalanceApplySummary     `json:"apply,omitempty"`
	Policy           *RebalancePolicy           `json:"policy,omitempty"`
	History          *RebalanceHistoryReport    `json:"history,omitempty"`
	Envelope         *RebalanceEnvelopeReport   `json:"envelope,omitempty"`
	Projection       *RebalanceProjectionReport `json:"projection,omitempty"`
	Approve          *RebalanceApprove          `json:"approve,omitempty"`
	Notes            []string                   `json:"notes,omitempty"`
}

type RebalanceScope struct {
	StartDate      string `json:"startDate,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	Week           string `json:"week,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	TimezoneSource string `json:"timezoneSource,omitempty"`
}

type RebalanceRequest struct {
	Strategy          string   `json:"strategy,omitempty"`
	ReplaceIntensity  []string `json:"replaceIntensity,omitempty"`
	IncludeAdaptation bool     `json:"includeAdaptation,omitempty"`
	DryRun            bool     `json:"dryRun"`
	File              string   `json:"file,omitempty"`
}

type RebalanceConstraints struct {
	KeepCompleted       bool     `json:"keepCompleted"`
	AllowToday          bool     `json:"allowToday"`
	AllowPast           bool     `json:"allowPast"`
	AllowedDays         []string `json:"allowedDays,omitempty"`
	OffDays             []string `json:"offDays,omitempty"`
	AllowCreate         bool     `json:"allowCreate"`
	AllowUpdate         bool     `json:"allowUpdate"`
	AllowCancel         bool     `json:"allowCancel"`
	SportType           string   `json:"sportType,omitempty"`
	WorkoutTarget       string   `json:"workoutTarget,omitempty"`
	StartTime           string   `json:"startTime,omitempty"`
	MinSessionMinutes   int      `json:"minSessionMinutes,omitempty"`
	DurationStepMinutes int      `json:"durationStepMinutes,omitempty"`
	AllocationBasis     string   `json:"allocationBasis,omitempty"`
	MaxSessionMinutes   int      `json:"maxSessionMinutes,omitempty"`
	TargetLoad          int      `json:"targetLoad"`
	TargetTolerance     int      `json:"targetTolerance"`
	MaxIntensity        float64  `json:"maxIntensity,omitempty"`
	MaxWatts            int      `json:"maxWatts,omitempty"`
	MaxHR               int      `json:"maxHr,omitempty"`
	Z1IF                float64  `json:"z1If,omitempty"`
	Z2IF                float64  `json:"z2If,omitempty"`
	GateMode            string   `json:"gateMode,omitempty"`
	Note                string   `json:"note,omitempty"`
	Level               string   `json:"level,omitempty"`
	Mode                string   `json:"mode,omitempty"`
}

type RebalanceContext struct {
	FTP             int              `json:"ftp,omitempty"`
	LTHR            int              `json:"lthr,omitempty"`
	MaxHR           int              `json:"maxHr,omitempty"`
	WellnessState   string           `json:"wellnessState,omitempty"`
	WellnessReasons []string         `json:"wellnessReasons,omitempty"`
	PlanDecision    string           `json:"planDecision,omitempty"`
	PlanForecast    string           `json:"planForecast,omitempty"`
	DynamicTargets  RebalanceTargets `json:"dynamicTargets"`
	Warnings        []string         `json:"warnings,omitempty"`
}

type RebalanceTargets struct {
	Z1IF                  float64 `json:"z1If,omitempty"`
	Z2IF                  float64 `json:"z2If,omitempty"`
	MaxIntensity          float64 `json:"maxIntensity,omitempty"`
	LongRideMinutes       int     `json:"longRideMinutes,omitempty"`
	DailyLoadLimit        int     `json:"dailyLoadLimit,omitempty"`
	TargetTolerance       int     `json:"targetTolerance,omitempty"`
	TargetToleranceSource string  `json:"targetToleranceSource,omitempty"`
	Source                string  `json:"source,omitempty"`
}

type RebalanceEvaluation struct {
	WeeklyLoad             int     `json:"weeklyLoad"`
	CompletedLoad          int     `json:"completedLoad"`
	LockedPlannedLoad      int     `json:"lockedPlannedLoad"`
	MutablePlannedLoad     int     `json:"mutablePlannedLoad"`
	TargetLoad             int     `json:"targetLoad"`
	TargetDelta            int     `json:"targetDelta"`
	MovingTimeSecs         int     `json:"movingTimeSecs"`
	MovingTimeHours        float64 `json:"movingTimeHours,omitempty"`
	HighIntensitySessions  int     `json:"highIntensitySessions"`
	TempoThresholdSessions int     `json:"tempoThresholdSessions"`
	LongEnduranceSessions  int     `json:"longEnduranceSessions"`
	AerobicSessions        int     `json:"aerobicSessions"`
	RecoverySessions       int     `json:"recoverySessions"`
	WeekendLoad            int     `json:"weekendLoad"`
	WeekendLoadPercent     float64 `json:"weekendLoadPercent,omitempty"`
	Feasible               bool    `json:"feasible"`
	Score                  int     `json:"score,omitempty"`
	Source                 string  `json:"source,omitempty"`
}

type RebalanceOption struct {
	ID         string              `json:"id"`
	Label      string              `json:"label"`
	Evaluation RebalanceEvaluation `json:"evaluation"`
	Sessions   []RebalanceSession  `json:"sessions"`
	Warnings   []string            `json:"warnings,omitempty"`
}

type RebalanceSession struct {
	ID                   string  `json:"id,omitempty"`
	Date                 string  `json:"date,omitempty"`
	Day                  string  `json:"day,omitempty"`
	StartTime            string  `json:"startTime,omitempty"`
	StartTimeSource      string  `json:"startTimeSource,omitempty"`
	EventID              int     `json:"eventId,omitempty"`
	Action               string  `json:"action"`
	SportType            string  `json:"sportType,omitempty"`
	SportTypeSource      string  `json:"sportTypeSource,omitempty"`
	WorkoutTarget        string  `json:"workoutTarget,omitempty"`
	WorkoutTargetSource  string  `json:"workoutTargetSource,omitempty"`
	Name                 string  `json:"name,omitempty"`
	Classification       string  `json:"classification,omitempty"`
	ClassificationSource string  `json:"classificationSource,omitempty"`
	AllocationSource     string  `json:"allocationSource,omitempty"`
	TargetLoad           int     `json:"targetLoad"`
	DurationSeconds      int     `json:"durationSeconds"`
	DurationMinutes      float64 `json:"durationMinutes,omitempty"`
	DurationSource       string  `json:"durationSource,omitempty"`
	IntensityFactor      float64 `json:"intensityFactor,omitempty"`
	IntensitySource      string  `json:"intensitySource,omitempty"`
	WattsRange           string  `json:"wattsRange,omitempty"`
	Description          string  `json:"description,omitempty"`
	SourceHash           string  `json:"sourceHash,omitempty"`
	Reason               string  `json:"reason,omitempty"`
}

type RebalanceOperation struct {
	ID                     string  `json:"id"`
	Action                 string  `json:"action"`
	EventID                int     `json:"eventId,omitempty"`
	SourceHash             string  `json:"sourceHash,omitempty"`
	CancelMode             string  `json:"cancelMode,omitempty"`
	Body                   EventEx `json:"body,omitempty"`
	Reason                 string  `json:"reason,omitempty"`
	ExpectedLoad           int     `json:"expectedLoad,omitempty"`
	ExpectedMovingTimeSecs int     `json:"expectedMovingTimeSecs,omitempty"`
	ExpectedIF             float64 `json:"expectedIntensityFactor,omitempty"`
	ExpectedWatts          string  `json:"expectedWatts,omitempty"`
	Status                 string  `json:"status,omitempty"`
	AppliedEventID         int     `json:"appliedEventId,omitempty"`
	Error                  string  `json:"error,omitempty"`
}

type RebalanceValidation struct {
	Feasible bool            `json:"feasible"`
	Blocking bool            `json:"blocking"`
	Warnings []string        `json:"warnings,omitempty"`
	Errors   []string        `json:"errors,omitempty"`
	Gates    []RebalanceGate `json:"gates,omitempty"`
}

type RebalanceGate struct {
	Severity string `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Action   string `json:"action,omitempty"`
	Source   string `json:"source,omitempty"`
}

type RebalanceApplySummary struct {
	Applied  int                    `json:"applied"`
	Skipped  int                    `json:"skipped"`
	Failed   int                    `json:"failed"`
	Results  []RebalanceApplyResult `json:"results,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
}

type RebalanceApplyResult struct {
	OperationID string `json:"operationId"`
	Action      string `json:"action"`
	EventID     int    `json:"eventId,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

type rebalancePlan struct {
	baseline    RebalanceEvaluation
	targets     RebalanceTargets
	mutable     []Event
	locked      []Event
	operations  []RebalanceOperation
	sessions    []RebalanceSession
	warnings    []string
	targetLoad  int
	plannedLoad int
}

type rebalanceSlot struct {
	date  string
	event *Event
}

type rebalanceEventSplit struct {
	mutable []Event
	locked  []Event
}

func DefaultRebalanceConstraints() RebalanceConstraints {
	return RebalanceConstraints{
		KeepCompleted: true,
		AllowCreate:   true,
		AllowUpdate:   true,
		AllowCancel:   true,
		GateMode:      "warn",
	}
}

func BuildRebalanceProposal(input *RebalanceInput) RebalanceProposalFile {
	if input == nil {
		input = &RebalanceInput{}
	}
	working := normalizeRebalanceInput(input)
	if working.Request.Strategy == RebalanceStrategyAdaptiveBidirectional {
		return buildRebalanceProposalV2(&working)
	}
	plan := buildRebalancePlan(&working)
	option := rebalanceOption(&working, &plan)
	proposal := RebalanceProposalFile{
		SchemaVersion:    RebalanceSchemaVersion,
		ProposalID:       rebalanceProposalID(&working),
		GeneratedAt:      working.GeneratedAt,
		AthleteID:        working.AthleteID,
		Scope:            working.Scope,
		Request:          working.Request,
		Constraints:      working.Constraints,
		Context:          rebalanceContext(&working, &plan.targets, plan.warnings),
		Baseline:         plan.baseline,
		Options:          []RebalanceOption{option},
		SelectedOptionID: option.ID,
		Operations:       plan.operations,
		Notes:            rebalanceNotes(&working),
	}
	proposal.Validation = ValidateRebalanceProposal(&proposal)

	return proposal
}

func ValidateRebalanceProposal(proposal *RebalanceProposalFile) RebalanceValidation {
	validation := RebalanceValidation{Feasible: true}
	if proposal == nil {
		return RebalanceValidation{Blocking: true, Errors: []string{"proposal is required"}}
	}
	validateRebalanceHeader(proposal, &validation)
	validateRebalanceDecisionSources(proposal, &validation)
	for index := range proposal.Operations {
		validateRebalanceOperation(&proposal.Operations[index], &validation)
	}
	validation.Blocking = len(validation.Errors) > 0
	validation.Feasible = !validation.Blocking

	return validation
}

func RebalanceEventHash(event *Event) string {
	if event == nil {
		return ""
	}
	snapshot := struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		StartDateLocal string `json:"startDateLocal"`
		Description    string `json:"description"`
		TrainingLoad   int    `json:"trainingLoad"`
		MovingTime     int    `json:"movingTime"`
	}{
		ID:             event.ID,
		Name:           event.Name,
		StartDateLocal: event.StartDateLocal,
		Description:    event.Description,
		TrainingLoad:   event.TrainingLoad,
		MovingTime:     event.MovingTime,
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])[:rebalanceHashPrefixLength]
}

func MarshalRebalanceProposal(proposal *RebalanceProposalFile) ([]byte, error) {
	data, err := json.MarshalIndent(proposal, "", rebalanceJSONIndent)
	if err != nil {
		return nil, fmt.Errorf("marshal rebalance proposal: %w", err)
	}

	return append(data, '\n'), nil
}

func DynamicRebalanceTargets(input *RebalanceInput) RebalanceTargets {
	if input == nil {
		return missingRebalanceTargets()
	}
	if targets, ok := sportSettingsRebalanceTargets(input.SportSettings); ok {
		targets.MaxIntensity = explicitOrDynamicMax(input, targets.Z2IF)
		targets.LongRideMinutes = dynamicLongRideMinutes(input.Activities)
		targets.DailyLoadLimit = dynamicDailyLoadLimit(input.Activities)
		targets.TargetTolerance = input.Constraints.TargetTolerance
		targets.TargetToleranceSource = rebalanceTargetToleranceSource(input)

		return targets
	}
	if targets, ok := historicalRebalanceTargets(input.Activities); ok {
		targets.MaxIntensity = explicitOrDynamicMax(input, targets.Z2IF)
		targets.LongRideMinutes = dynamicLongRideMinutes(input.Activities)
		targets.DailyLoadLimit = dynamicDailyLoadLimit(input.Activities)
		targets.TargetTolerance = input.Constraints.TargetTolerance
		targets.TargetToleranceSource = rebalanceTargetToleranceSource(input)

		return targets
	}

	targets := missingRebalanceTargets()
	targets.LongRideMinutes = dynamicLongRideMinutes(input.Activities)
	targets.DailyLoadLimit = dynamicDailyLoadLimit(input.Activities)
	targets.TargetTolerance = input.Constraints.TargetTolerance
	targets.TargetToleranceSource = rebalanceTargetToleranceSource(input)

	return targets
}

func normalizeRebalanceInput(input *RebalanceInput) RebalanceInput {
	working := *input
	defaults := DefaultRebalanceConstraints()
	mergeRebalanceDefaults(&working.Constraints, &defaults)
	if working.Request.Strategy == "" {
		working.Request.Strategy = rebalanceStrategyTargetZ2
	}
	targets := DynamicRebalanceTargets(&working)
	if working.Constraints.Z1IF == 0 && targets.Z1IF > 0 {
		working.Constraints.Z1IF = targets.Z1IF
	}
	if working.Constraints.Z2IF == 0 && targets.Z2IF > 0 {
		working.Constraints.Z2IF = targets.Z2IF
	}
	if working.Constraints.TargetTolerance == 0 {
		working.Constraints.TargetTolerance = dynamicTargetTolerance(working.Activities)
	}
	if working.Constraints.MinSessionMinutes == 0 {
		working.Constraints.MinSessionMinutes = dynamicMinSessionMinutes(working.Activities, working.Events)
	}
	if working.Constraints.DurationStepMinutes == 0 {
		working.Constraints.DurationStepMinutes = dynamicDurationStepMinutes(working.Activities, working.Events)
	}
	if working.Constraints.StartTime == "" {
		working.Constraints.StartTime = historicalStartTime(working.Activities, "")
	}
	if working.Constraints.TargetLoad == 0 && working.Request.Strategy != RebalanceStrategyAdaptiveBidirectional {
		baseline := rebalanceBaseline(&working)
		working.Constraints.TargetLoad = baseline.WeeklyLoad
	}

	return working
}

func mergeRebalanceDefaults(constraints, defaults *RebalanceConstraints) {
	if !constraints.KeepCompleted {
		constraints.KeepCompleted = defaults.KeepCompleted
	}
	if !constraints.AllowCreate && !constraints.AllowUpdate && !constraints.AllowCancel {
		constraints.AllowCreate = defaults.AllowCreate
		constraints.AllowUpdate = defaults.AllowUpdate
		constraints.AllowCancel = defaults.AllowCancel
	}
	if constraints.GateMode == "" {
		constraints.GateMode = defaults.GateMode
	}
}

func buildRebalancePlan(input *RebalanceInput) rebalancePlan {
	baseline := rebalanceBaseline(input)
	if reasons := rebalanceBlockingReasons(input); len(reasons) > 0 {
		return rebalancePlan{baseline: baseline, targets: DynamicRebalanceTargets(input), warnings: reasons, targetLoad: input.Constraints.TargetLoad, plannedLoad: baseline.WeeklyLoad}
	}
	target := input.Constraints.TargetLoad
	events := splitRebalanceEvents(input)
	preserved := preservedMutableLoad(input, events.mutable)
	remaining := maxInt(0, target-baseline.CompletedLoad-baseline.LockedPlannedLoad-preserved)
	var slots []rebalanceSlot
	if remaining > 0 {
		slots = rebalanceSlots(input, events.mutable)
	}
	loads := distributeRebalanceLoad(input, remaining, slots)
	plan := rebalancePlan{
		baseline:    baseline,
		targets:     DynamicRebalanceTargets(input),
		mutable:     events.mutable,
		locked:      events.locked,
		targetLoad:  target,
		plannedLoad: baseline.CompletedLoad + baseline.LockedPlannedLoad + preserved,
	}
	if remaining > 0 && len(slots) == 0 {
		plan.warnings = append(plan.warnings, "no mutable or creatable slots available")
	}
	if remaining > 0 && len(slots) > 0 && len(loads) == 0 {
		plan.warnings = append(plan.warnings, "no historical or explicit allocation basis available")
	}
	updated := map[int]bool{}
	for index := range loads {
		plan.addSlotOperation(input, &slots[index], loads[index])
		if slots[index].event != nil && loads[index] > 0 {
			updated[slots[index].event.ID] = true
		}
	}
	plan.addPreservedMutableLoad(input, updated)
	plan.addCancelOperations(input, slots)

	return plan
}

func (plan *rebalancePlan) addPreservedMutableLoad(input *RebalanceInput, updated map[int]bool) {
	if input.Constraints.AllowCancel || !input.Constraints.AllowUpdate {
		return
	}
	for index := range plan.mutable {
		event := &plan.mutable[index]
		if updated[event.ID] {
			continue
		}
		plan.plannedLoad += rebalanceEventLoad(event, input)
	}
}

func preservedMutableLoad(input *RebalanceInput, mutable []Event) int {
	if input.Constraints.AllowUpdate || input.Constraints.AllowCancel {
		return 0
	}
	var total int
	for index := range mutable {
		total += rebalanceEventLoad(&mutable[index], input)
	}

	return total
}

func (plan *rebalancePlan) addSlotOperation(input *RebalanceInput, slot *rebalanceSlot, load int) {
	if load <= 0 {
		return
	}
	session := buildRebalanceSession(input, slot, load)
	plan.sessions = append(plan.sessions, session)
	plan.plannedLoad += session.TargetLoad
	operation := buildRebalanceOperation(&session)
	if operation.Action != "" {
		plan.operations = append(plan.operations, operation)
	}
}

func (plan *rebalancePlan) addCancelOperations(input *RebalanceInput, slots []rebalanceSlot) {
	used := map[int]bool{}
	for index := range slots {
		if slots[index].event != nil {
			used[slots[index].event.ID] = true
		}
	}
	for index := range plan.mutable {
		event := &plan.mutable[index]
		if used[event.ID] || !input.Constraints.AllowCancel {
			continue
		}
		session := RebalanceSession{
			ID:         "cancel-" + strconv.Itoa(event.ID),
			Date:       eventDateOnly(event.StartDateLocal),
			EventID:    event.ID,
			Action:     RebalanceActionCancel,
			Name:       event.Name,
			SourceHash: RebalanceEventHash(event),
			Reason:     "remaining load already allocated to earlier slots",
		}
		plan.sessions = append(plan.sessions, session)
		plan.operations = append(plan.operations, buildRebalanceOperation(&session))
	}
}

func rebalanceOption(input *RebalanceInput, plan *rebalancePlan) RebalanceOption {
	evaluation := evaluateRebalancePlan(input, plan)

	return RebalanceOption{
		ID:         "target_preserving_z2",
		Label:      "Target-preserving Z1/Z2 rebalance",
		Evaluation: evaluation,
		Sessions:   plan.sessions,
		Warnings:   plan.warnings,
	}
}

func buildRebalanceOperation(session *RebalanceSession) RebalanceOperation {
	operation := RebalanceOperation{
		ID:                     session.ID,
		Action:                 session.Action,
		EventID:                session.EventID,
		SourceHash:             session.SourceHash,
		Reason:                 session.Reason,
		ExpectedLoad:           session.TargetLoad,
		ExpectedMovingTimeSecs: session.DurationSeconds,
		ExpectedIF:             session.IntensityFactor,
		ExpectedWatts:          session.WattsRange,
		Status:                 RebalanceStatusPending,
	}
	if session.Action == RebalanceActionCancel {
		operation.CancelMode = "delete"

		return operation
	}
	operation.Body = EventEx{
		StartDateLocal: session.dateTimeLocal(),
		Category:       "WORKOUT",
		Type:           sessionType(session),
		Name:           session.Name,
		Description:    session.Description,
		TrainingLoad:   session.TargetLoad,
		MovingTime:     session.DurationSeconds,
		Target:         sessionTarget(session),
	}

	return operation
}

func buildRebalanceSession(input *RebalanceInput, slot *rebalanceSlot, load int) RebalanceSession {
	intensity := effectiveRebalanceIF(input)
	duration := durationForLoad(input, load, intensity)
	if input.Constraints.MaxSessionMinutes > 0 {
		duration = minInt(duration, input.Constraints.MaxSessionMinutes*rebalanceSecondsPerMinute)
	}
	load = estimateLoadFromDurationIF(duration, intensity)
	action := RebalanceActionCreate
	eventID := 0
	var sourceHash string
	if slot.event != nil {
		action = RebalanceActionUpdate
		eventID = slot.event.ID
		sourceHash = RebalanceEventHash(slot.event)
	}
	startTime := rebalanceSessionStartTime(input, slot)
	startTimeSource := rebalanceSessionStartTimeSource(input, slot)
	sportType := rebalanceSessionSportType(input, slot)
	workoutTarget := rebalanceSessionWorkoutTarget(input, slot)
	classification := classifyGeneratedRebalanceSession(input, duration, intensity)
	minutes := int(math.Round(float64(duration) / rebalanceSecondsPerMinute))
	name := fmt.Sprintf("Rebalance %s Z2 %d", slot.date, load)

	return RebalanceSession{
		ID:                   action + "-" + slot.date,
		Date:                 slot.date,
		Day:                  rebalanceWeekday(slot.date),
		StartTime:            startTime,
		StartTimeSource:      startTimeSource,
		EventID:              eventID,
		Action:               action,
		SportType:            sportType,
		SportTypeSource:      rebalanceSessionSportTypeSource(input, slot),
		WorkoutTarget:        workoutTarget,
		WorkoutTargetSource:  rebalanceSessionWorkoutTargetSource(input, slot),
		Name:                 name,
		Classification:       classification.label,
		ClassificationSource: classification.source,
		AllocationSource:     rebalanceAllocationSource(input, slot),
		TargetLoad:           load,
		DurationSeconds:      duration,
		DurationMinutes:      round2(float64(duration) / rebalanceSecondsPerMinute),
		DurationSource:       rebalanceSessionDurationSource(input),
		IntensityFactor:      intensity,
		IntensitySource:      rebalanceSessionIntensitySource(input),
		WattsRange:           wattsRange(input, intensity),
		Description:          rebalanceWorkoutDescription(minutes, intensity),
		SourceHash:           sourceHash,
		Reason:               "target-preserving Z1/Z2 redistribution",
	}
}

func rebalanceBaseline(input *RebalanceInput) RebalanceEvaluation {
	completed := completedRebalanceLoad(input)
	completedDates := completedRebalanceDates(input)
	var locked int
	var mutable int
	var moving int
	var high int
	var tempo int
	var long int
	var aerobic int
	var recovery int
	var weekend int
	for index := range input.Events {
		event := &input.Events[index]
		if !isWorkoutEvent(event) {
			continue
		}
		if completedDates[eventDateOnly(event.StartDateLocal)] {
			continue
		}
		load := rebalanceEventLoad(event, input)
		moving += event.MovingTime
		if isImmutableEvent(event, input) {
			locked += load
		} else {
			mutable += load
		}
		if isWeekendDate(eventDateOnly(event.StartDateLocal)) {
			weekend += load
		}
		classification := unknownState
		high += boolInt(classification == "high_intensity")
		tempo += boolInt(classification == "tempo_threshold")
		long += boolInt(classification == "long_endurance")
		aerobic += boolInt(classification == plannedClassificationZ2)
		recovery += boolInt(classification == plannedStepKindRecovery)
	}
	weekly := completed + locked + mutable
	target := input.Constraints.TargetLoad
	if target == 0 {
		target = weekly
	}

	return RebalanceEvaluation{
		WeeklyLoad:             weekly,
		CompletedLoad:          completed,
		LockedPlannedLoad:      locked,
		MutablePlannedLoad:     mutable,
		TargetLoad:             target,
		TargetDelta:            weekly - target,
		MovingTimeSecs:         moving,
		MovingTimeHours:        round2(float64(moving) / rebalanceSecondsPerHour),
		HighIntensitySessions:  high,
		TempoThresholdSessions: tempo,
		LongEnduranceSessions:  long,
		AerobicSessions:        aerobic,
		RecoverySessions:       recovery,
		WeekendLoad:            weekend,
		WeekendLoadPercent:     percentOf(weekend, weekly),
		Feasible:               true,
		Source:                 "completed_activity_and_calendar_event_load",
	}
}

func completedRebalanceDates(input *RebalanceInput) map[string]bool {
	dates := map[string]bool{}
	for index := range input.Activities {
		date := activityDate(&input.Activities[index])
		if date >= input.Scope.StartDate && date <= input.Scope.EndDate {
			dates[date] = true
		}
	}

	return dates
}

func evaluateRebalancePlan(input *RebalanceInput, plan *rebalancePlan) RebalanceEvaluation {
	load := plan.plannedLoad
	var moving int
	var weekend int
	var recovery int
	var aerobic int
	var long int
	for index := range plan.sessions {
		session := &plan.sessions[index]
		if session.Action == RebalanceActionCancel {
			continue
		}
		moving += session.DurationSeconds
		if isWeekendDate(session.Date) {
			weekend += session.TargetLoad
		}
		aerobic++
		if session.DurationSeconds >= longRideSeconds(&plan.targets) {
			long++
		}
		if session.IntensityFactor <= input.Constraints.Z1IF {
			recovery++
		}
	}
	feasible := absInt(load-plan.targetLoad) <= input.Constraints.TargetTolerance

	return RebalanceEvaluation{
		WeeklyLoad:             load,
		CompletedLoad:          plan.baseline.CompletedLoad,
		LockedPlannedLoad:      plan.baseline.LockedPlannedLoad,
		MutablePlannedLoad:     load - plan.baseline.CompletedLoad - plan.baseline.LockedPlannedLoad,
		TargetLoad:             plan.targetLoad,
		TargetDelta:            load - plan.targetLoad,
		MovingTimeSecs:         moving,
		MovingTimeHours:        round2(float64(moving) / rebalanceSecondsPerHour),
		LongEnduranceSessions:  long,
		AerobicSessions:        aerobic,
		RecoverySessions:       recovery,
		WeekendLoad:            weekend,
		WeekendLoadPercent:     percentOf(weekend, load),
		Feasible:               feasible,
		Source:                 "rebalance_operations",
		HighIntensitySessions:  plan.baseline.HighIntensitySessions,
		TempoThresholdSessions: 0,
	}
}

func splitRebalanceEvents(input *RebalanceInput) rebalanceEventSplit {
	events := rebalanceEventSplit{
		mutable: make([]Event, 0, len(input.Events)),
		locked:  make([]Event, 0, len(input.Events)),
	}
	for index := range input.Events {
		event := input.Events[index]
		if !isWorkoutEvent(&event) {
			continue
		}
		if isImmutableEvent(&event, input) {
			events.locked = append(events.locked, event)
			continue
		}
		events.mutable = append(events.mutable, event)
	}
	sort.Slice(events.mutable, func(left, right int) bool {
		return events.mutable[left].StartDateLocal < events.mutable[right].StartDateLocal
	})

	return events
}

func rebalanceSlots(input *RebalanceInput, mutable []Event) []rebalanceSlot {
	slots := make([]rebalanceSlot, 0, len(mutable)+len(dateRangeInclusive(input.Scope.StartDate, input.Scope.EndDate)))
	for index := range mutable {
		if input.Constraints.AllowUpdate {
			slots = append(slots, rebalanceSlot{date: eventDateOnly(mutable[index].StartDateLocal), event: &mutable[index]})
		}
	}
	if !input.Constraints.AllowCreate {
		return slots
	}
	occupied := occupiedRebalanceDates(input.Events)
	for _, date := range dateRangeInclusive(input.Scope.StartDate, input.Scope.EndDate) {
		if occupied[date] || !dateAllowed(input, date) || dateLocked(input, date) {
			continue
		}
		slots = append(slots, rebalanceSlot{date: date})
	}
	sort.Slice(slots, func(left, right int) bool { return slots[left].date < slots[right].date })

	return slots
}

func occupiedRebalanceDates(events []Event) map[string]bool {
	occupied := map[string]bool{}
	for index := range events {
		date := eventDateOnly(events[index].StartDateLocal)
		if date != "" {
			occupied[date] = true
		}
	}

	return occupied
}

func distributeRebalanceLoad(input *RebalanceInput, total int, slots []rebalanceSlot) []int {
	if total <= 0 || len(slots) == 0 {
		return nil
	}
	weights := rebalanceSlotWeights(input, slots)
	if len(weights) == 0 {
		return nil
	}
	loads := make([]int, len(slots))
	var assigned int
	for index := range loads {
		if index == len(loads)-1 {
			loads[index] = total - assigned
			break
		}
		loads[index] = int(math.Round(float64(total) * weights[index]))
		assigned += loads[index]
	}

	return loads
}

func rebalanceSlotWeights(input *RebalanceInput, slots []rebalanceSlot) []float64 {
	if input.Constraints.AllocationBasis == rebalanceAllocationExplicit {
		return equalRebalanceWeights(len(slots))
	}
	if weights, ok := mutableEventWeights(input, slots); ok {
		return weights
	}
	return historicalDayWeights(input.Activities, slots)
}

func equalRebalanceWeights(count int) []float64 {
	if count <= 0 {
		return nil
	}
	weights := make([]float64, count)
	for index := range weights {
		weights[index] = 1 / float64(count)
	}

	return weights
}

func mutableEventWeights(input *RebalanceInput, slots []rebalanceSlot) ([]float64, bool) {
	weights := make([]float64, len(slots))
	var total int
	for index := range slots {
		if slots[index].event == nil {
			continue
		}
		load := rebalanceEventLoad(slots[index].event, input)
		if load <= 0 {
			return nil, false
		}
		weights[index] = float64(load)
		total += load
	}
	if total == 0 {
		return nil, false
	}
	for index := range weights {
		weights[index] /= float64(total)
	}

	return weights, true
}

func historicalDayWeights(activities []Activity, slots []rebalanceSlot) []float64 {
	loadByDay := map[string]int{}
	for index := range activities {
		date := activityDate(&activities[index])
		day := rebalanceWeekday(date)
		if day != "" && activities[index].TrainingLoad > 0 {
			loadByDay[day] += activities[index].TrainingLoad
		}
	}
	if len(loadByDay) < 3 {
		return nil
	}
	weights := make([]float64, len(slots))
	var total int
	for index := range slots {
		load := loadByDay[rebalanceWeekday(slots[index].date)]
		if load <= 0 {
			return nil
		}
		weights[index] = float64(load)
		total += load
	}
	if total == 0 {
		return nil
	}
	for index := range weights {
		weights[index] /= float64(total)
	}

	return weights
}

func validateRebalanceHeader(proposal *RebalanceProposalFile, validation *RebalanceValidation) {
	if proposal.SchemaVersion != RebalanceSchemaVersion {
		validation.Errors = append(validation.Errors, "unsupported schemaVersion")
	}
	if proposal.SelectedOptionID == "" {
		validation.Warnings = append(validation.Warnings, "selectedOptionId is empty")
	}
}

func validateRebalanceDecisionSources(proposal *RebalanceProposalFile, validation *RebalanceValidation) {
	if len(proposal.Options) == 0 && proposal.Baseline.Source == "" {
		return
	}
	validateRebalanceRequestSources(proposal, validation)
	validateRebalanceConstraintSources(proposal, validation)
	validateRebalanceOperationSources(proposal, validation)
}

func validateRebalanceRequestSources(proposal *RebalanceProposalFile, validation *RebalanceValidation) {
	if proposal.Request.Strategy != "" && proposal.Request.Strategy != rebalanceStrategyTargetZ2 && proposal.Request.Strategy != RebalanceStrategyAdaptiveBidirectional {
		validation.Errors = append(validation.Errors, "unsupported rebalance strategy: "+proposal.Request.Strategy)
	}
}

func validateRebalanceConstraintSources(proposal *RebalanceProposalFile, validation *RebalanceValidation) {
	if proposal.Constraints.TargetTolerance == 0 {
		validation.Errors = append(validation.Errors, "targetTolerance requires historical load variability or explicit input")
	}
	if proposal.Constraints.Z2IF == 0 {
		validation.Errors = append(validation.Errors, "z2If requires sport power zones, historical intensity, or explicit input")
	}
	if proposal.Constraints.MinSessionMinutes == 0 || proposal.Constraints.DurationStepMinutes == 0 {
		validation.Errors = append(validation.Errors, "duration constraints require history or explicit input")
	}
	if proposal.Constraints.AllowCreate && proposal.Constraints.SportType == "" && !proposalHasNonCancelType(proposal) {
		validation.Errors = append(validation.Errors, "sport type requires event context or explicit input")
	}
	if proposal.Constraints.AllowCreate && proposal.Constraints.WorkoutTarget == "" && !proposalHasNonCancelTarget(proposal) {
		validation.Errors = append(validation.Errors, "workout target requires event context or explicit input")
	}
}

func validateRebalanceOperationSources(proposal *RebalanceProposalFile, validation *RebalanceValidation) {
	for index := range proposal.Operations {
		validateRebalanceOperationSource(&proposal.Operations[index], validation)
	}
}

func validateRebalanceOperationSource(operation *RebalanceOperation, validation *RebalanceValidation) {
	if operation.Action == RebalanceActionCreate && operation.Body.StartDateLocal == eventDateOnly(operation.Body.StartDateLocal) {
		validation.Errors = append(validation.Errors, "create operation requires historical or explicit start time: "+operation.ID)
	}
	if operation.Action != RebalanceActionCancel && operation.Body.Type == "" {
		validation.Errors = append(validation.Errors, "operation body.type requires event context or explicit sport type: "+operation.ID)
	}
	if operation.Action != RebalanceActionCancel && operation.Body.Target == "" {
		validation.Errors = append(validation.Errors, "operation body.target requires event context or explicit workout target: "+operation.ID)
	}
}

func validateRebalanceOperation(operation *RebalanceOperation, validation *RebalanceValidation) {
	if operation.ID == "" {
		validation.Errors = append(validation.Errors, "operation id is required")
	}
	if !validRebalanceAction(operation.Action) {
		validation.Errors = append(validation.Errors, "operation action is unsupported: "+operation.Action)
	}
	if !validRebalanceStatus(operation.Status) {
		validation.Errors = append(validation.Errors, "operation status is unsupported: "+operation.Status)
	}
	if operation.Action != RebalanceActionCreate && operation.EventID == 0 {
		validation.Errors = append(validation.Errors, "eventId is required for "+operation.Action)
	}
	if operation.Action != RebalanceActionCancel && operation.Body.Name == "" {
		validation.Errors = append(validation.Errors, "body.name is required for "+operation.Action)
	}
}

func validRebalanceAction(action string) bool {
	switch action {
	case RebalanceActionCreate, RebalanceActionUpdate, RebalanceActionCancel:
		return true
	default:
		return false
	}
}

func validRebalanceStatus(status string) bool {
	switch status {
	case "", RebalanceStatusPending, RebalanceStatusSkipped, RebalanceStatusApplied, RebalanceStatusFailed:
		return true
	default:
		return false
	}
}

func rebalanceBlockingReasons(input *RebalanceInput) []string {
	var reasons []string
	reasons = append(reasons, rebalanceRequestBlockingReasons(input)...)
	reasons = append(reasons, rebalanceConstraintBlockingReasons(input)...)
	if input.Constraints.AllowCreate && input.Constraints.StartTime == "" && historicalStartTime(input.Activities, "") == "" {
		reasons = append(reasons, "created workouts require historical or explicit start time")
	}
	if input.Constraints.AllowCreate && input.Constraints.AllocationBasis == "" && len(input.Activities) == 0 {
		reasons = append(reasons, "created workout allocation requires history or explicit allocation basis")
	}
	if input.Constraints.SportType == "" && !hasMutableEventType(input.Events) {
		reasons = append(reasons, "sport type requires event context or explicit input")
	}
	if input.Constraints.WorkoutTarget == "" && !hasMutableEventTarget(input.Events) {
		reasons = append(reasons, "workout target requires event context or explicit input")
	}

	return reasons
}

func rebalanceRequestBlockingReasons(input *RebalanceInput) []string {
	if input.Request.Strategy != "" && input.Request.Strategy != rebalanceStrategyTargetZ2 && input.Request.Strategy != RebalanceStrategyAdaptiveBidirectional {
		return []string{"unsupported rebalance strategy: " + input.Request.Strategy}
	}

	return nil
}

func rebalanceConstraintBlockingReasons(input *RebalanceInput) []string {
	var reasons []string
	if input.Constraints.TargetTolerance == 0 {
		reasons = append(reasons, "target tolerance requires historical load variability or explicit input")
	}
	if input.Constraints.Z2IF == 0 {
		reasons = append(reasons, "intensity target requires sport power zones, historical intensity, or explicit input")
	}
	if input.Constraints.MinSessionMinutes == 0 || input.Constraints.DurationStepMinutes == 0 {
		reasons = append(reasons, "duration constraints require history or explicit input")
	}

	return reasons
}

func rebalanceContext(input *RebalanceInput, targets *RebalanceTargets, warnings []string) RebalanceContext {
	context := RebalanceContext{FTP: rebalanceFTP(input), Warnings: warnings}
	if targets != nil {
		context.DynamicTargets = *targets
	}
	if input.SportSettings != nil {
		context.LTHR = input.SportSettings.LTHR
		context.MaxHR = input.SportSettings.MaxHR
	}
	if input.Wellness != nil {
		context.WellnessState = input.Wellness.State.State
		context.WellnessReasons = input.Wellness.State.Reasons
	}
	if input.Plan != nil {
		context.PlanDecision = input.Plan.Decision.PrimaryDirective
		context.PlanForecast = input.Plan.Forecast.Status
	}

	return context
}

func rebalanceNotes(input *RebalanceInput) []string {
	notes := []string{"Edit selectedOptionId or pending operations before running icu rebalance accept --file <path>."}
	if input.Constraints.Note != "" {
		notes = append(notes, input.Constraints.Note)
	}

	return notes
}

func rebalanceProposalID(input *RebalanceInput) string {
	raw := input.AthleteID + "|" + input.Scope.StartDate + "|" + input.Scope.EndDate + "|" + strconv.Itoa(input.Constraints.TargetLoad)
	sum := sha256.Sum256([]byte(raw))

	return strings.Trim(input.Scope.Week+"-"+hex.EncodeToString(sum[:])[:rebalanceHashPrefixLength], "-")
}

func completedRebalanceLoad(input *RebalanceInput) int {
	var total int
	for index := range input.Activities {
		activity := &input.Activities[index]
		date := activityDate(activity)
		if date >= input.Scope.StartDate && date <= input.Scope.EndDate {
			total += activity.TrainingLoad
		}
	}

	return total
}

func isImmutableEvent(event *Event, input *RebalanceInput) bool {
	date := eventDateOnly(event.StartDateLocal)
	if date == "" {
		return true
	}
	if completedRebalanceDates(input)[date] {
		return true
	}
	if rebalanceDateLockedByTime(input, date) {
		return true
	}
	return strings.EqualFold(event.Category, planTargetCategory) || isRaceEventCategory(event.Category)
}

func isWorkoutEvent(event *Event) bool {
	return event != nil && strings.EqualFold(event.Category, planWorkoutCategory)
}

func rebalanceEventLoad(event *Event, input *RebalanceInput) int {
	if event.TrainingLoad > 0 {
		return event.TrainingLoad
	}
	if event.WorkoutDoc == nil || rebalanceFTP(input) == 0 {
		return 0
	}
	data, err := json.Marshal(event.WorkoutDoc)
	if err != nil {
		return 0
	}
	var doc WorkoutDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0
	}

	return EstimatePlannedLoad(&doc, rebalanceFTP(input)).TrainingLoad
}

func dateAllowed(input *RebalanceInput, date string) bool {
	day := rebalanceWeekday(date)
	allowed := daySet(input.Constraints.AllowedDays)
	off := daySet(input.Constraints.OffDays)
	if len(allowed) > 0 && !allowed[day] {
		return false
	}

	return !off[day]
}

func dateLocked(input *RebalanceInput, date string) bool {
	if completedRebalanceDates(input)[date] {
		return true
	}

	return rebalanceDateLockedByTime(input, date)
}

func effectiveRebalanceIF(input *RebalanceInput) float64 {
	base := input.Constraints.Z2IF
	if input.Constraints.Z2IF > 0 {
		return constrainedRebalanceIF(input, base)
	}

	return constrainedRebalanceIF(input, DynamicRebalanceTargets(input).Z2IF)
}

func constrainedRebalanceIF(input *RebalanceInput, base float64) float64 {
	intensity := base
	if input.Constraints.MaxIntensity > 0 {
		intensity = math.Min(intensity, input.Constraints.MaxIntensity)
	}
	ftp := rebalanceFTP(input)
	if input.Constraints.MaxWatts > 0 && ftp > 0 {
		intensity = math.Min(intensity, float64(input.Constraints.MaxWatts)/float64(ftp))
	}

	return round3(intensity)
}

func durationForLoad(input *RebalanceInput, load int, intensity float64) int {
	if load <= 0 || intensity <= 0 {
		return 0
	}
	hours := float64(load) / (rebalanceSportZoneDivisor * intensity * intensity)
	minutes := int(math.Round(hours * rebalanceMinutesPerHour))
	minMinutes := input.Constraints.MinSessionMinutes
	stepMinutes := input.Constraints.DurationStepMinutes
	if minMinutes == 0 || stepMinutes == 0 {
		return 0
	}
	minutes = roundMinutesToStep(maxInt(minutes, minMinutes), stepMinutes)

	return minutes * rebalanceSecondsPerMinute
}

func estimateLoadFromDurationIF(duration int, intensity float64) int {
	if duration <= 0 || intensity <= 0 {
		return 0
	}
	hours := float64(duration) / rebalanceSecondsPerHour

	return int(math.Round(hours * rebalanceSportZoneDivisor * intensity * intensity))
}

func rebalanceWorkoutDescription(minutes int, intensity float64) string {
	if minutes <= 0 {
		return ""
	}
	percent := int(math.Round(intensity * rebalanceSportZoneDivisor))
	low := maxInt(1, percent-int(math.Round(rebalanceWattsLowOffset*rebalanceSportZoneDivisor)))

	return fmt.Sprintf("- %dm %d-%d%%", minutes, low, percent)
}

func (session *RebalanceSession) dateTimeLocal() string {
	if session.Date == "" {
		return ""
	}
	if session.StartTime != "" {
		return session.Date + "T" + session.StartTime + ":00"
	}

	return session.Date
}

func sessionType(session *RebalanceSession) string {
	return session.SportType
}

func sessionTarget(session *RebalanceSession) string {
	return session.WorkoutTarget
}

func rebalanceSessionStartTime(input *RebalanceInput, slot *rebalanceSlot) string {
	if slot.event != nil {
		return eventTimeOnly(slot.event.StartDateLocal)
	}
	if input.Constraints.StartTime != "" {
		return input.Constraints.StartTime
	}

	return historicalStartTime(input.Activities, rebalanceWeekday(slot.date))
}

func rebalanceSessionStartTimeSource(input *RebalanceInput, slot *rebalanceSlot) string {
	if slot.event != nil {
		return "existing_event_local_time"
	}
	if input.Constraints.StartTime != "" {
		return "explicit_start_time"
	}
	if historicalStartTime(input.Activities, rebalanceWeekday(slot.date)) != "" {
		return "historical_same_weekday_median_start_time_mad_filtered"
	}

	return ""
}

func rebalanceSessionSportType(input *RebalanceInput, slot *rebalanceSlot) string {
	if slot.event != nil && slot.event.Type != "" {
		return slot.event.Type
	}

	return input.Constraints.SportType
}

func rebalanceSessionSportTypeSource(input *RebalanceInput, slot *rebalanceSlot) string {
	if slot.event != nil && slot.event.Type != "" {
		return "existing_event_type"
	}
	if input.Constraints.SportType != "" {
		return "explicit_sport_type"
	}

	return ""
}

func rebalanceSessionWorkoutTarget(input *RebalanceInput, slot *rebalanceSlot) string {
	if slot.event != nil && slot.event.Target != "" {
		return slot.event.Target
	}
	if input.Constraints.WorkoutTarget != "" {
		return input.Constraints.WorkoutTarget
	}

	return ""
}

func rebalanceSessionWorkoutTargetSource(input *RebalanceInput, slot *rebalanceSlot) string {
	if slot.event != nil && slot.event.Target != "" {
		return "existing_event_target"
	}
	if input.Constraints.WorkoutTarget != "" {
		return "explicit_workout_target"
	}

	return ""
}

func rebalanceSessionIntensitySource(input *RebalanceInput) string {
	if input.Constraints.Z2IF > 0 {
		return "explicit_or_normalized_z2_if"
	}

	return DynamicRebalanceTargets(input).Source
}

func rebalanceSessionDurationSource(input *RebalanceInput) string {
	if input.Constraints.MinSessionMinutes > 0 && input.Constraints.DurationStepMinutes > 0 {
		return "target_load_from_if_with_explicit_or_derived_duration_constraints"
	}

	return ""
}

func rebalanceAllocationSource(input *RebalanceInput, slot *rebalanceSlot) string {
	if input.Constraints.AllocationBasis == rebalanceAllocationExplicit {
		return "explicit_equal"
	}
	if slot.event != nil {
		return "existing_mutable_event_load_shape"
	}
	if len(input.Activities) > 0 {
		return "historical_weekday_load_distribution"
	}

	return ""
}

type rebalanceClassificationDecision struct {
	label  string
	source string
}

func classifyGeneratedRebalanceSession(input *RebalanceInput, durationSeconds int, intensity float64) rebalanceClassificationDecision {
	targets := DynamicRebalanceTargets(input)
	if input.Constraints.Z1IF > 0 && intensity <= input.Constraints.Z1IF {
		return rebalanceClassificationDecision{label: plannedStepKindRecovery, source: "dynamic_z1_if_threshold"}
	}
	if targets.LongRideMinutes > 0 && durationSeconds >= targets.LongRideMinutes*rebalanceSecondsPerMinute {
		return rebalanceClassificationDecision{label: "long_endurance", source: "historical_duration_distribution"}
	}
	if input.Constraints.Z2IF > 0 && intensity > input.Constraints.Z2IF {
		return rebalanceClassificationDecision{label: "tempo_threshold", source: "dynamic_z2_if_threshold"}
	}
	if intensity > 0 {
		return rebalanceClassificationDecision{label: plannedClassificationZ2, source: "dynamic_endurance_if_band"}
	}

	return rebalanceClassificationDecision{label: unknownState, source: "missing_classification_basis"}
}

func wattsRange(input *RebalanceInput, intensity float64) string {
	ftp := rebalanceFTP(input)
	if ftp == 0 || intensity == 0 {
		return ""
	}
	low := int(math.Round(float64(ftp) * math.Max(rebalanceWattsLowFloor, intensity-rebalanceWattsLowOffset)))
	high := int(math.Round(float64(ftp) * intensity))

	return fmt.Sprintf("%d-%dw", low, high)
}

func rebalanceFTP(input *RebalanceInput) int {
	if input == nil || input.SportSettings == nil {
		return 0
	}
	if input.SportSettings.IndoorFTP > 0 {
		return input.SportSettings.IndoorFTP
	}

	return input.SportSettings.FTP
}

func sportSettingsRebalanceTargets(settings *SportSettings) (RebalanceTargets, bool) {
	if settings == nil || len(settings.PowerZones) < 2 || settings.PowerZones[0] <= 0 || settings.PowerZones[1] <= settings.PowerZones[0] {
		return RebalanceTargets{}, false
	}
	z1 := float64(settings.PowerZones[0]) / rebalanceSportZoneDivisor
	z2 := (float64(settings.PowerZones[0]) + float64(settings.PowerZones[1])) / (rebalanceSportZoneDivisor * 2)

	return RebalanceTargets{Z1IF: round3(z1), Z2IF: round3(z2), Source: "sport_settings_power_zones"}, true
}

func historicalRebalanceTargets(activities []Activity) (RebalanceTargets, bool) {
	var intensities []float64
	for index := range activities {
		if activities[index].Intensity > 0 && IsCyclingActivityType(activities[index].Type) {
			intensities = append(intensities, activities[index].Intensity)
		}
	}
	filtered := robustFilteredFloats(intensities)
	if len(filtered) < 3 {
		return RebalanceTargets{}, false
	}
	z2 := medianFloat64(filtered)
	z1Candidates := make([]float64, 0, len(filtered))
	for _, intensity := range filtered {
		if intensity <= z2 {
			z1Candidates = append(z1Candidates, intensity)
		}
	}
	if len(z1Candidates) == 0 {
		return RebalanceTargets{}, false
	}

	return RebalanceTargets{
		Z1IF:   round3(medianFloat64(z1Candidates)),
		Z2IF:   round3(z2),
		Source: "recent_activity_intensity_median_mad_filtered",
	}, true
}

func missingRebalanceTargets() RebalanceTargets {
	return RebalanceTargets{Source: "missing_intensity_basis"}
}

func explicitOrDynamicMax(input *RebalanceInput, z2 float64) float64 {
	if input.Constraints.MaxIntensity > 0 {
		return input.Constraints.MaxIntensity
	}
	var intensities []float64
	for index := range input.Activities {
		if input.Activities[index].Intensity > 0 && IsCyclingActivityType(input.Activities[index].Type) {
			intensities = append(intensities, input.Activities[index].Intensity)
		}
	}
	filtered := robustFilteredFloats(intensities)
	if len(filtered) == 0 {
		return z2
	}
	median := medianFloat64(filtered)

	return round3(math.Max(z2, median+madFloat64(filtered, median)))
}

func dynamicLongRideMinutes(activities []Activity) int {
	var durations []float64
	for index := range activities {
		activity := &activities[index]
		if IsCyclingActivityType(activity.Type) && activity.MovingTime > 0 {
			durations = append(durations, float64(activity.MovingTime/rebalanceSecondsPerMinute))
		}
	}
	filtered := robustFilteredFloats(durations)
	if len(filtered) == 0 {
		return 0
	}
	median := medianFloat64(filtered)

	return roundMinutesToStep(int(math.Round(median+madFloat64(filtered, median))), rebalanceDescriptionStepMinutes)
}

func dynamicDailyLoadLimit(activities []Activity) int {
	loadsByDate := map[string]int{}
	for index := range activities {
		activity := &activities[index]
		date := activityDate(activity)
		if date != "" {
			loadsByDate[date] += activity.TrainingLoad
		}
	}
	loads := make([]float64, 0, len(loadsByDate))
	for _, load := range loadsByDate {
		if load > 0 {
			loads = append(loads, float64(load))
		}
	}
	filtered := robustFilteredFloats(loads)
	if len(filtered) == 0 {
		return 0
	}
	median := medianFloat64(filtered)

	return int(math.Round(median + madFloat64(filtered, median)))
}

func dynamicTargetTolerance(activities []Activity) int {
	loadsByWeek := map[string]int{}
	for index := range activities {
		date := activityDate(&activities[index])
		if date == "" || activities[index].TrainingLoad <= 0 {
			continue
		}
		week := isoWeekKeyFromDate(date)
		if week != "" {
			loadsByWeek[week] += activities[index].TrainingLoad
		}
	}
	loads := make([]float64, 0, len(loadsByWeek))
	for _, load := range loadsByWeek {
		if load > 0 {
			loads = append(loads, float64(load))
		}
	}
	filtered := robustFilteredFloats(loads)
	if len(filtered) < 3 {
		return 0
	}
	median := medianFloat64(filtered)

	return int(math.Round(madFloat64(filtered, median)))
}

func rebalanceTargetToleranceSource(input *RebalanceInput) string {
	if input == nil {
		return ""
	}
	if derived := dynamicTargetTolerance(input.Activities); derived > 0 && derived == input.Constraints.TargetTolerance {
		return "historical_weekly_load_mad"
	}
	if input.Constraints.TargetTolerance > 0 {
		return "explicit_target_tolerance"
	}

	return ""
}

func dynamicMinSessionMinutes(activities []Activity, events []Event) int {
	durations := rebalanceDurationsMinutes(activities, events)
	filtered := robustFilteredFloats(durations)
	if len(filtered) < 3 {
		return 0
	}
	minimum := int(math.Round(filtered[0]))
	for _, duration := range filtered[1:] {
		minimum = minInt(minimum, int(math.Round(duration)))
	}

	return minimum
}

func dynamicDurationStepMinutes(activities []Activity, events []Event) int {
	durations := rebalanceDurationsMinutes(activities, events)
	filtered := robustFilteredFloats(durations)
	if len(filtered) < 3 {
		return 0
	}
	step := 0
	for _, duration := range filtered {
		minutes := int(math.Round(duration))
		if minutes <= 0 {
			continue
		}
		if step == 0 {
			step = minutes
			continue
		}
		step = gcdInt(step, minutes)
	}

	return step
}

func rebalanceDurationsMinutes(activities []Activity, events []Event) []float64 {
	durations := make([]float64, 0, len(activities)+len(events))
	for index := range activities {
		if activities[index].MovingTime > 0 {
			durations = append(durations, float64(activities[index].MovingTime/rebalanceSecondsPerMinute))
		}
	}
	for index := range events {
		if events[index].MovingTime > 0 && isWorkoutEvent(&events[index]) {
			durations = append(durations, float64(events[index].MovingTime/rebalanceSecondsPerMinute))
		}
	}

	return durations
}

func historicalStartTime(activities []Activity, weekday string) string {
	var minutes []float64
	for index := range activities {
		date := activityDate(&activities[index])
		if date == "" || len(activities[index].StartDateLocal) < len("2006-01-02T15:04") {
			continue
		}
		if weekday != "" && rebalanceWeekday(date) != weekday {
			continue
		}
		minute := localTimeMinutes(activities[index].StartDateLocal)
		if minute >= 0 {
			minutes = append(minutes, float64(minute))
		}
	}
	filtered := robustFilteredFloats(minutes)
	if len(filtered) < 3 {
		return ""
	}

	return minutesToLocalTime(int(math.Round(medianFloat64(filtered))))
}

func localTimeMinutes(value string) int {
	if len(value) < len("2006-01-02T15:04") {
		return -1
	}
	hour, hourErr := strconv.Atoi(value[11:13])
	minute, minuteErr := strconv.Atoi(value[14:16])
	if hourErr != nil || minuteErr != nil {
		return -1
	}

	return hour*rebalanceMinutesPerHour + minute
}

func minutesToLocalTime(minutes int) string {
	hour := minutes / rebalanceMinutesPerHour
	minute := minutes % rebalanceMinutesPerHour

	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func isoWeekKeyFromDate(value string) string {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return ""
	}
	year, week := date.ISOWeek()

	return fmt.Sprintf("%04d-W%02d", year, week)
}

func robustFilteredFloats(values []float64) []float64 {
	if len(values) < rebalanceDescriptionStepMinutes {
		return append([]float64(nil), values...)
	}
	median := medianFloat64(values)
	mad := madFloat64(values, median)
	if mad == 0 {
		return append([]float64(nil), values...)
	}
	filtered := make([]float64, 0, len(values))
	for _, value := range values {
		robustZ := rebalanceMadScale * (value - median) / mad
		if math.Abs(robustZ) <= rebalanceRobustZLimit {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return append([]float64(nil), values...)
	}

	return filtered
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	midpoint := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[midpoint]
	}

	return (sorted[midpoint-1] + sorted[midpoint]) / 2
}

func madFloat64(values []float64, median float64) float64 {
	deviations := make([]float64, 0, len(values))
	for _, value := range values {
		deviations = append(deviations, math.Abs(value-median))
	}

	return medianFloat64(deviations)
}

func longRideSeconds(targets *RebalanceTargets) int {
	if targets == nil {
		return 0
	}
	if targets.LongRideMinutes > 0 {
		return targets.LongRideMinutes * rebalanceSecondsPerMinute
	}

	return 2 * rebalanceSecondsPerHour
}

func eventDateOnly(value string) string {
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}

	return value
}

func eventTimeOnly(value string) string {
	if len(value) >= len("2006-01-02T15:04") {
		return value[11:16]
	}

	return ""
}

func rebalanceWeekday(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}

	return strings.ToLower(parsed.Weekday().String()[:3])
}

func dateRangeInclusive(start, end string) []string {
	startDate, startErr := time.Parse("2006-01-02", start)
	endDate, endErr := time.Parse("2006-01-02", end)
	if startErr != nil || endErr != nil || endDate.Before(startDate) {
		return nil
	}
	var dates []string
	for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
		dates = append(dates, date.Format("2006-01-02"))
	}

	return dates
}

func daySet(days []string) map[string]bool {
	result := map[string]bool{}
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		if len(normalized) > len("mon") {
			normalized = normalized[:len("mon")]
		}
		if normalized != "" {
			result[normalized] = true
		}
	}

	return result
}

func isWeekendDate(date string) bool {
	day := rebalanceWeekday(date)

	return day == "sat" || day == "sun"
}

func percentOf(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return round2(float64(part) * rebalanceSportZoneDivisor / float64(total))
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}

	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}

	return right
}

func gcdInt(left, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	if left < 0 {
		return -left
	}

	return left
}

func hasMutableEventType(events []Event) bool {
	for index := range events {
		if isWorkoutEvent(&events[index]) && events[index].Type != "" {
			return true
		}
	}

	return false
}

func hasMutableEventTarget(events []Event) bool {
	for index := range events {
		if isWorkoutEvent(&events[index]) && events[index].Target != "" {
			return true
		}
	}

	return false
}

func proposalHasNonCancelType(proposal *RebalanceProposalFile) bool {
	for index := range proposal.Operations {
		if proposal.Operations[index].Action != RebalanceActionCancel && proposal.Operations[index].Body.Type != "" {
			return true
		}
	}

	return false
}

func proposalHasNonCancelTarget(proposal *RebalanceProposalFile) bool {
	for index := range proposal.Operations {
		if proposal.Operations[index].Action != RebalanceActionCancel && proposal.Operations[index].Body.Target != "" {
			return true
		}
	}

	return false
}

func rebalanceDateLockedByTime(input *RebalanceInput, date string) bool {
	if !input.Constraints.KeepCompleted || input.NowDate == "" {
		return false
	}
	if date < input.NowDate {
		return !input.Constraints.AllowPast
	}
	if date == input.NowDate {
		return !input.Constraints.AllowToday
	}

	return false
}
