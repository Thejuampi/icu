package icu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PlanSchemaVersion       = "icu.plan.v1"
	PlanIntentSchemaVersion = "icu.plan.intent.v1"
)

type PlanIntent struct {
	SchemaVersion string             `json:"schemaVersion"`
	Oldest        string             `json:"oldest,omitempty"`
	Newest        string             `json:"newest,omitempty"`
	FTP           int                `json:"ftp,omitempty"`
	WeeklyTargets []PlanWeeklyTarget `json:"weeklyTargets,omitempty"`
	StrictTargets bool               `json:"strictTargets,omitempty"`
	Constraints   PlanConstraints    `json:"constraints"`
	Sessions      []PlanSessionDraft `json:"sessions"`
	Notes         []string           `json:"notes,omitempty"`
}

type PlanWeeklyTarget struct {
	WeekStart  string `json:"weekStart"`
	TargetLoad int    `json:"targetLoad"`
	Tolerance  int    `json:"tolerance"`
}

type PlanConstraints struct {
	AllowCreate      bool     `json:"allowCreate"`
	AllowUpdate      bool     `json:"allowUpdate"`
	AllowCancel      bool     `json:"allowCancel"`
	AllowToday       bool     `json:"allowToday"`
	AllowPast        bool     `json:"allowPast"`
	DefaultStartTime string   `json:"defaultStartTime,omitempty"`
	DefaultType      string   `json:"defaultType,omitempty"`
	DefaultCategory  string   `json:"defaultCategory,omitempty"`
	DefaultTarget    string   `json:"defaultTarget,omitempty"`
	CancelCategories []string `json:"cancelCategories,omitempty"`
}

type PlanSessionDraft struct {
	Date      string `json:"date"`
	Name      string `json:"name"`
	Category  string `json:"category,omitempty"`
	Type      string `json:"type,omitempty"`
	StartTime string `json:"startTime,omitempty"`
	Desc      string `json:"desc,omitempty"`
	DescLocal string `json:"descLocal,omitempty"`
	Role      string `json:"role,omitempty"`
	UID       string `json:"uid,omitempty"`
	Target    string `json:"target,omitempty"`
}

type PlanInput struct {
	AthleteID     string
	GeneratedAt   string
	NowDate       string
	Activities    []Activity
	Events        []Event
	SportSettings *SportSettings
	Intent        PlanIntent
	AllowToday    bool
	AllowPast     bool
}

type PlanProposalFile struct {
	SchemaVersion string                `json:"schemaVersion"`
	ProposalID    string                `json:"proposalId"`
	GeneratedAt   string                `json:"generatedAt,omitempty"`
	AthleteID     string                `json:"athleteId,omitempty"`
	Scope         PlanScope             `json:"scope"`
	Intent        PlanIntent            `json:"intent"`
	Constraints   PlanConstraints       `json:"constraints"`
	Context       PlanContext           `json:"context"`
	Baseline      PlanBaseline          `json:"baseline"`
	Sessions      []PlanResolvedSession `json:"sessions"`
	Operations    []CalendarOperation   `json:"operations"`
	WeeklyLoads   []PlanWeekLoad        `json:"weeklyLoads,omitempty"`
	Validation    PlanValidation        `json:"validation"`
	Preview       *CalendarOpsPreview   `json:"preview,omitempty"`
	Apply         *CalendarApplySummary `json:"apply,omitempty"`
	Notes         []string              `json:"notes,omitempty"`
}

type PlanScope struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	NowDate   string `json:"nowDate,omitempty"`
}

type PlanContext struct {
	FTP       int      `json:"ftp,omitempty"`
	FTPSource string   `json:"ftpSource,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

type PlanBaseline struct {
	CompletedLoad      int `json:"completedLoad"`
	LockedPlannedLoad  int `json:"lockedPlannedLoad"`
	MutablePlannedLoad int `json:"mutablePlannedLoad"`
	WeeklyLoad         int `json:"weeklyLoad"`
}

type PlanResolvedSession struct {
	ID             string `json:"id"`
	Date           string `json:"date"`
	Name           string `json:"name"`
	Action         string `json:"action"`
	EventID        int    `json:"eventId,omitempty"`
	SourceHash     string `json:"sourceHash,omitempty"`
	Category       string `json:"category,omitempty"`
	Type           string `json:"type,omitempty"`
	StartTime      string `json:"startTime,omitempty"`
	Target         string `json:"target,omitempty"`
	Role           string `json:"role,omitempty"`
	UID            string `json:"uid,omitempty"`
	Desc           string `json:"desc,omitempty"`
	DescLocal      string `json:"descLocal,omitempty"`
	TrainingLoad   int    `json:"trainingLoad,omitempty"`
	MovingTimeSecs int    `json:"movingTimeSecs,omitempty"`
	LoadSource     string `json:"loadSource,omitempty"`
	MatchedBy      string `json:"matchedBy,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type PlanWeekLoad struct {
	WeekStart     string `json:"weekStart"`
	TargetLoad    int    `json:"targetLoad,omitempty"`
	Tolerance     int    `json:"tolerance,omitempty"`
	PlannedLoad   int    `json:"plannedLoad"`
	CompletedLoad int    `json:"completedLoad"`
	Delta         int    `json:"delta,omitempty"`
	WithinTarget  bool   `json:"withinTarget"`
}

type PlanValidation struct {
	Feasible bool     `json:"feasible"`
	Blocking bool     `json:"blocking"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

func DefaultPlanConstraints() PlanConstraints {
	return PlanConstraints{
		AllowCreate:     true,
		AllowUpdate:     true,
		AllowCancel:     false,
		DefaultCategory: planWorkoutCategory,
	}
}

func BuildPlanProposal(input *PlanInput) PlanProposalFile {
	if input == nil {
		input = &PlanInput{}
	}
	working := normalizePlanInput(input)
	baseline := planBaseline(&working)
	context := planContext(&working)
	sessions, operations, warnings, errors := buildPlanSessions(&working)
	weekly := evaluatePlanWeeklyLoads(&working, sessions, baseline)
	proposal := PlanProposalFile{
		SchemaVersion: PlanSchemaVersion,
		ProposalID:    planProposalID(&working),
		GeneratedAt:   working.GeneratedAt,
		AthleteID:     working.AthleteID,
		Scope: PlanScope{
			StartDate: working.Intent.Oldest,
			EndDate:   working.Intent.Newest,
			NowDate:   working.NowDate,
		},
		Intent:      working.Intent,
		Constraints: working.Intent.Constraints,
		Context:     context,
		Baseline:    baseline,
		Sessions:    sessions,
		Operations:  operations,
		WeeklyLoads: weekly,
		Notes:       append([]string{}, working.Intent.Notes...),
		Validation: PlanValidation{
			Errors:   append([]string{}, errors...),
			Warnings: append([]string{}, warnings...),
		},
	}
	preview := PreviewCalendarOperations(operations)
	proposal.Preview = &preview
	proposal.Validation.Warnings = appendUniqueStrings(proposal.Validation.Warnings, context.Warnings...)
	proposal.Validation = ValidatePlanProposal(&proposal)

	return proposal
}

func ValidatePlanProposal(proposal *PlanProposalFile) PlanValidation {
	if proposal == nil {
		return PlanValidation{Blocking: true, Errors: []string{"proposal is required"}}
	}
	seeded := proposal.Validation
	validation := PlanValidation{Feasible: true}
	if proposal.SchemaVersion != PlanSchemaVersion {
		validation.Errors = append(validation.Errors, "unsupported plan schemaVersion: "+proposal.SchemaVersion)
	}
	if proposal.Scope.StartDate == "" || proposal.Scope.EndDate == "" {
		validation.Errors = append(validation.Errors, "scope startDate and endDate are required")
	}
	if proposal.Scope.StartDate != "" && proposal.Scope.EndDate != "" && proposal.Scope.StartDate > proposal.Scope.EndDate {
		validation.Errors = append(validation.Errors, "scope startDate must be on or before endDate")
	}
	validation.Errors = append(validation.Errors, ValidateCalendarOperations(proposal.Operations)...)
	for index := range proposal.Operations {
		operation := &proposal.Operations[index]
		if operation.Action == CalendarActionCreate && operation.Body.StartDateLocal == eventDateOnly(operation.Body.StartDateLocal) {
			validation.Errors = append(validation.Errors, "create operation requires start time: "+operation.ID)
		}
		if operation.Action != CalendarActionCancel && operation.Body.Type == "" {
			validation.Errors = append(validation.Errors, "operation body.type is required: "+operation.ID)
		}
		if operation.Action != CalendarActionCancel && operation.Body.Category == "" {
			validation.Errors = append(validation.Errors, "operation body.category is required: "+operation.ID)
		}
		if planOperationRequiresLoad(operation) && operation.ExpectedLoad <= 0 {
			validation.Errors = append(validation.Errors, "workout operation requires expectedLoad > 0: "+operation.ID)
		}
		if date := eventDateOnly(operation.Body.StartDateLocal); date != "" && proposal.Scope.StartDate != "" && proposal.Scope.EndDate != "" {
			if date < proposal.Scope.StartDate || date > proposal.Scope.EndDate {
				validation.Errors = append(validation.Errors, "operation date outside scope: "+operation.ID)
			}
		}
	}
	validation.Errors = appendUniqueStrings(validation.Errors, seeded.Errors...)
	validation.Warnings = appendUniqueStrings(validation.Warnings, seeded.Warnings...)
	applyPlanWeeklyTargetValidationTo(&validation, proposal)
	if seeded.Blocking && len(validation.Errors) == 0 {
		validation.Errors = append(validation.Errors, "proposal validation is marked blocking")
	}
	validation.Blocking = len(validation.Errors) > 0
	validation.Feasible = !validation.Blocking

	return validation
}

func MarshalPlanProposal(proposal *PlanProposalFile) ([]byte, error) {
	data, err := json.MarshalIndent(proposal, "", rebalanceJSONIndent)
	if err != nil {
		return nil, fmt.Errorf("marshal plan proposal: %w", err)
	}

	return append(data, '\n'), nil
}

func MarshalPlanIntent(intent *PlanIntent) ([]byte, error) {
	data, err := json.MarshalIndent(intent, "", rebalanceJSONIndent)
	if err != nil {
		return nil, fmt.Errorf("marshal plan intent: %w", err)
	}

	return append(data, '\n'), nil
}

func normalizePlanInput(input *PlanInput) PlanInput {
	working := *input
	defaults := DefaultPlanConstraints()
	mergePlanConstraints(&working.Intent.Constraints, &defaults)
	if working.AllowToday {
		working.Intent.Constraints.AllowToday = true
	}
	if working.AllowPast {
		working.Intent.Constraints.AllowPast = true
	}
	if working.Intent.SchemaVersion == "" {
		working.Intent.SchemaVersion = PlanIntentSchemaVersion
	}
	if working.NowDate == "" {
		working.NowDate = working.Intent.Newest
	}
	if working.Intent.Oldest == "" || working.Intent.Newest == "" {
		working.Intent.Oldest, working.Intent.Newest = planSessionDateBounds(working.Intent.Sessions)
	}

	return working
}

func mergePlanConstraints(constraints, defaults *PlanConstraints) {
	if !constraints.AllowCreate && !constraints.AllowUpdate && !constraints.AllowCancel {
		constraints.AllowCreate = defaults.AllowCreate
		constraints.AllowUpdate = defaults.AllowUpdate
		constraints.AllowCancel = defaults.AllowCancel
	}
	if constraints.DefaultCategory == "" {
		constraints.DefaultCategory = defaults.DefaultCategory
	}
}

func planSessionDateBounds(sessions []PlanSessionDraft) (string, string) {
	var oldest string
	var newest string
	for index := range sessions {
		date := sessions[index].Date
		if date == "" {
			continue
		}
		if oldest == "" || date < oldest {
			oldest = date
		}
		if newest == "" || date > newest {
			newest = date
		}
	}

	return oldest, newest
}

func planContext(input *PlanInput) PlanContext {
	ftp, source := planFTP(input)
	context := PlanContext{FTP: ftp, FTPSource: source}
	if ftp <= 0 {
		context.Warnings = append(context.Warnings, "ftp is missing; planned load will not be calculated from workout descriptions")
	}

	return context
}

func planFTP(input *PlanInput) (int, string) {
	if input.Intent.FTP > 0 {
		return input.Intent.FTP, "intent"
	}
	if input.SportSettings != nil && input.SportSettings.FTP > 0 {
		return input.SportSettings.FTP, "sport_settings"
	}

	return 0, ""
}

func planBaseline(input *PlanInput) PlanBaseline {
	rebalanceInput := planAsRebalanceInput(input)
	evaluation := RebalanceBaseline(&rebalanceInput)

	return PlanBaseline{
		CompletedLoad:      evaluation.CompletedLoad,
		LockedPlannedLoad:  evaluation.LockedPlannedLoad,
		MutablePlannedLoad: evaluation.MutablePlannedLoad,
		WeeklyLoad:         evaluation.WeeklyLoad,
	}
}

func planAsRebalanceInput(input *PlanInput) RebalanceInput {
	return RebalanceInput{
		NowDate:    input.NowDate,
		Activities: input.Activities,
		Events:     input.Events,
		Scope: RebalanceScope{
			StartDate: input.Intent.Oldest,
			EndDate:   input.Intent.Newest,
		},
		Constraints: RebalanceConstraints{
			KeepCompleted: true,
			AllowToday:    input.Intent.Constraints.AllowToday,
			AllowPast:     input.Intent.Constraints.AllowPast,
			AllowCreate:   input.Intent.Constraints.AllowCreate,
			AllowUpdate:   input.Intent.Constraints.AllowUpdate,
			AllowCancel:   input.Intent.Constraints.AllowCancel,
		},
		SportSettings: input.SportSettings,
	}
}

func buildPlanSessions(input *PlanInput) ([]PlanResolvedSession, []CalendarOperation, []string, []string) {
	var sessions []PlanResolvedSession
	var operations []CalendarOperation
	var warnings []string
	var errors []string
	matched := map[int]bool{}
	ftp, _ := planFTP(input)
	for index := range input.Intent.Sessions {
		draft := &input.Intent.Sessions[index]
		session, operation, warning, errMsg := resolvePlanSession(input, draft, index, ftp, matched)
		if errMsg != "" {
			errors = append(errors, errMsg)
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if session.ID == "" {
			continue
		}
		sessions = append(sessions, session)
		if operation.ID != "" {
			operations = append(operations, operation)
		}
	}
	if input.Intent.Constraints.AllowCancel {
		cancelSessions, cancelOps := planCancelUnmatched(input, matched)
		sessions = append(sessions, cancelSessions...)
		operations = append(operations, cancelOps...)
	}
	sort.Slice(operations, func(left, right int) bool {
		return operations[left].ID < operations[right].ID
	})

	return sessions, operations, warnings, errors
}

type planSessionIssue struct {
	warning string
	errMsg  string
}

func resolvePlanSession(
	input *PlanInput,
	draft *PlanSessionDraft,
	index int,
	ftp int,
	matched map[int]bool,
) (PlanResolvedSession, CalendarOperation, string, string) {
	if draft == nil || draft.Date == "" || draft.Name == "" {
		return PlanResolvedSession{}, CalendarOperation{}, "", fmt.Sprintf("session[%d] requires date and name", index)
	}
	if input.Intent.Oldest != "" && input.Intent.Newest != "" {
		if draft.Date < input.Intent.Oldest || draft.Date > input.Intent.Newest {
			return PlanResolvedSession{}, CalendarOperation{}, "", fmt.Sprintf(
				"session %s on %s is outside intent range %s..%s",
				draft.Name, draft.Date, input.Intent.Oldest, input.Intent.Newest,
			)
		}
	}
	category := firstNonEmpty(draft.Category, input.Intent.Constraints.DefaultCategory, planWorkoutCategory)
	sportType := firstNonEmpty(draft.Type, input.Intent.Constraints.DefaultType)
	startTime := firstNonEmpty(draft.StartTime, input.Intent.Constraints.DefaultStartTime)
	target := firstNonEmpty(draft.Target, input.Intent.Constraints.DefaultTarget)
	if dateLockedForPlan(input, draft.Date) {
		return PlanResolvedSession{}, CalendarOperation{}, fmt.Sprintf("session %s on %s is locked (completed/past/today)", draft.Name, draft.Date), ""
	}
	event, matchedBy := matchPlanEvent(input.Events, draft, matched)
	loadDesc := firstNonEmpty(draft.DescLocal, draft.Desc)
	estimate, loadSource, loadIssue := estimatePlanSessionLoad(loadDesc, ftp, category)
	if loadIssue.errMsg != "" {
		return PlanResolvedSession{}, CalendarOperation{}, "", fmt.Sprintf("session %s on %s: %s", draft.Name, draft.Date, loadIssue.errMsg)
	}
	session := PlanResolvedSession{
		Date:           draft.Date,
		Name:           draft.Name,
		Category:       category,
		Type:           sportType,
		StartTime:      startTime,
		Target:         target,
		Role:           draft.Role,
		UID:            draft.UID,
		Desc:           draft.Desc,
		DescLocal:      draft.DescLocal,
		TrainingLoad:   estimate.TrainingLoad,
		MovingTimeSecs: estimate.DurationSeconds,
		LoadSource:     loadSource,
		MatchedBy:      matchedBy,
	}
	if event != nil {
		return planUpdateSession(input, &session, event, loadIssue.warning)
	}

	return planCreateSession(input, &session, loadIssue.warning)
}

func planCreateSession(input *PlanInput, session *PlanResolvedSession, loadWarning string) (PlanResolvedSession, CalendarOperation, string, string) {
	if !input.Intent.Constraints.AllowCreate {
		return PlanResolvedSession{}, CalendarOperation{}, fmt.Sprintf("create disabled for session %s on %s", session.Name, session.Date), ""
	}
	if session.StartTime == "" {
		return PlanResolvedSession{}, CalendarOperation{}, fmt.Sprintf("create requires startTime for session %s on %s", session.Name, session.Date), ""
	}
	if session.Type == "" {
		return PlanResolvedSession{}, CalendarOperation{}, fmt.Sprintf("create requires type for session %s on %s", session.Name, session.Date), ""
	}
	session.ID = "create-" + session.Date + "-" + sanitizePlanID(session.Name)
	session.Action = CalendarActionCreate
	session.Reason = "explicit intent session with no matching calendar event"
	operation := planOperationFromSession(session)

	return *session, operation, loadWarning, ""
}

func planUpdateSession(input *PlanInput, session *PlanResolvedSession, event *Event, loadWarning string) (PlanResolvedSession, CalendarOperation, string, string) {
	if !input.Intent.Constraints.AllowUpdate {
		return PlanResolvedSession{}, CalendarOperation{}, fmt.Sprintf("update disabled for matched session %s on %s", session.Name, session.Date), ""
	}
	if session.StartTime == "" {
		session.StartTime = eventTimeOnly(event.StartDateLocal)
	}
	if session.Type == "" {
		session.Type = event.Type
	}
	if session.Category == "" {
		session.Category = event.Category
	}
	if session.Target == "" {
		session.Target = event.Target
	}
	session.ID = "update-" + strconv.Itoa(event.ID)
	session.Action = CalendarActionUpdate
	session.EventID = event.ID
	session.SourceHash = EventSourceHash(event)
	session.Reason = "explicit intent session matched existing calendar event"
	operation := planOperationFromSession(session)

	return *session, operation, loadWarning, ""
}

func planOperationFromSession(session *PlanResolvedSession) CalendarOperation {
	operation := CalendarOperation{
		ID:                     session.ID,
		Action:                 session.Action,
		EventID:                session.EventID,
		SourceHash:             session.SourceHash,
		Reason:                 session.Reason,
		ExpectedLoad:           session.TrainingLoad,
		ExpectedMovingTimeSecs: session.MovingTimeSecs,
		Status:                 CalendarStatusPending,
	}
	if session.Action == CalendarActionCancel {
		operation.CancelMode = "delete"

		return operation
	}
	operation.Body = EventEx{
		StartDateLocal: planDateTimeLocal(session.Date, session.StartTime),
		Category:       session.Category,
		Type:           session.Type,
		Name:           session.Name,
		Description:    session.Desc,
		TrainingLoad:   session.TrainingLoad,
		MovingTime:     session.MovingTimeSecs,
		Target:         session.Target,
		ExternalID:     session.UID,
	}

	return operation
}

func planCancelUnmatched(input *PlanInput, matched map[int]bool) ([]PlanResolvedSession, []CalendarOperation) {
	allowed := cancelCategoriesAllowed(input.Intent.Constraints.CancelCategories)
	var sessions []PlanResolvedSession
	var operations []CalendarOperation
	rebalanceInput := planAsRebalanceInput(input)
	for index := range input.Events {
		event := &input.Events[index]
		if matched[event.ID] {
			continue
		}
		if !allowed[strings.ToUpper(event.Category)] {
			continue
		}
		if isImmutableEvent(event, &rebalanceInput) {
			continue
		}
		date := eventDateOnly(event.StartDateLocal)
		if date < input.Intent.Oldest || date > input.Intent.Newest {
			continue
		}
		session := PlanResolvedSession{
			ID:         "cancel-" + strconv.Itoa(event.ID),
			Date:       date,
			Name:       event.Name,
			Action:     CalendarActionCancel,
			EventID:    event.ID,
			SourceHash: EventSourceHash(event),
			Category:   event.Category,
			Type:       event.Type,
			Reason:     "unmatched mutable event with allowCancel",
		}
		sessions = append(sessions, session)
		operations = append(operations, planOperationFromSession(&session))
	}

	return sessions, operations
}

func matchPlanEvent(events []Event, draft *PlanSessionDraft, matched map[int]bool) (*Event, string) {
	if draft.UID != "" {
		for index := range events {
			event := &events[index]
			if matched[event.ID] {
				continue
			}
			if event.UID != "" && event.UID == draft.UID {
				matched[event.ID] = true

				return event, "uid"
			}
		}
	}
	for index := range events {
		event := &events[index]
		if matched[event.ID] {
			continue
		}
		if eventDateOnly(event.StartDateLocal) != draft.Date {
			continue
		}
		if !strings.EqualFold(event.Name, draft.Name) {
			continue
		}
		matched[event.ID] = true

		return event, "date_name"
	}

	return nil, ""
}

func estimatePlanSessionLoad(description string, ftp int, category string) (PlannedLoadEstimate, string, planSessionIssue) {
	requiresLoad := planCategoryRequiresLoad(category)
	if description == "" {
		if requiresLoad {
			return PlannedLoadEstimate{}, "", planSessionIssue{errMsg: "workout session requires a parseable description with expectedLoad > 0"}
		}

		return PlannedLoadEstimate{}, "", planSessionIssue{}
	}
	doc, err := ParseWorkoutDescription(description)
	if err != nil {
		if requiresLoad {
			return PlannedLoadEstimate{}, "", planSessionIssue{errMsg: "unparseable workout description: " + err.Error()}
		}

		return PlannedLoadEstimate{}, "", planSessionIssue{warning: "parse workout description: " + err.Error()}
	}
	estimate := EstimatePlannedLoad(doc, ftp)
	if requiresLoad && estimate.TrainingLoad <= 0 {
		message := "workout session expectedLoad is 0 after parsing description"
		if len(estimate.Warnings) > 0 {
			message += ": " + estimate.Warnings[0]
		}

		return estimate, estimate.Source, planSessionIssue{errMsg: message}
	}
	if len(estimate.Warnings) > 0 {
		return estimate, estimate.Source, planSessionIssue{warning: estimate.Warnings[0]}
	}

	return estimate, estimate.Source, planSessionIssue{}
}

func planCategoryRequiresLoad(category string) bool {
	return strings.EqualFold(strings.TrimSpace(category), planWorkoutCategory)
}

func planOperationRequiresLoad(operation *CalendarOperation) bool {
	if operation == nil {
		return false
	}
	if operation.Action == CalendarActionCancel {
		return false
	}
	status := operation.Status
	if status == "" {
		status = CalendarStatusPending
	}
	if status != CalendarStatusPending {
		return false
	}

	return planCategoryRequiresLoad(operation.Body.Category)
}

func evaluatePlanWeeklyLoads(input *PlanInput, sessions []PlanResolvedSession, baseline PlanBaseline) []PlanWeekLoad {
	if len(input.Intent.WeeklyTargets) == 0 {
		return nil
	}
	completedByWeek := map[string]int{}
	for index := range input.Activities {
		activity := &input.Activities[index]
		date := activityDate(activity)
		if date < input.Intent.Oldest || date > input.Intent.Newest {
			continue
		}
		week := planWeekStart(date)
		completedByWeek[week] += activity.TrainingLoad
	}
	plannedByWeek := map[string]int{}
	for week, load := range completedByWeek {
		plannedByWeek[week] = load
	}
	// locked planned on days without completed activity stays in baseline total only;
	// weekly targets compare completed + pending session ops in that week.
	for index := range sessions {
		session := &sessions[index]
		if session.Action == CalendarActionCancel {
			continue
		}
		week := planWeekStart(session.Date)
		plannedByWeek[week] += session.TrainingLoad
	}
	// Carry locked planned load that is not replaced: approximate by distributing
	// baseline locked load is already excluded from session ops when dates locked.
	_ = baseline
	loads := make([]PlanWeekLoad, 0, len(input.Intent.WeeklyTargets))
	for index := range input.Intent.WeeklyTargets {
		target := input.Intent.WeeklyTargets[index]
		week := target.WeekStart
		if week == "" {
			continue
		}
		planned := plannedByWeek[week]
		completed := completedByWeek[week]
		delta := planned - target.TargetLoad
		within := absInt(delta) <= target.Tolerance
		loads = append(loads, PlanWeekLoad{
			WeekStart:     week,
			TargetLoad:    target.TargetLoad,
			Tolerance:     target.Tolerance,
			PlannedLoad:   planned,
			CompletedLoad: completed,
			Delta:         delta,
			WithinTarget:  within,
		})
	}

	return loads
}

func applyPlanWeeklyTargetValidationTo(validation *PlanValidation, proposal *PlanProposalFile) {
	if validation == nil || proposal == nil {
		return
	}
	for index := range proposal.WeeklyLoads {
		week := &proposal.WeeklyLoads[index]
		if week.WithinTarget {
			continue
		}
		message := fmt.Sprintf(
			"week %s planned load %d outside target %d±%d (delta %d)",
			week.WeekStart, week.PlannedLoad, week.TargetLoad, week.Tolerance, week.Delta,
		)
		if proposal.Intent.StrictTargets {
			validation.Errors = appendUniqueStrings(validation.Errors, message)
			continue
		}
		validation.Warnings = appendUniqueStrings(validation.Warnings, message)
	}
}

func dateLockedForPlan(input *PlanInput, date string) bool {
	rebalanceInput := planAsRebalanceInput(input)

	return dateLocked(&rebalanceInput, date)
}

func planDateTimeLocal(date, startTime string) string {
	if date == "" {
		return ""
	}
	if startTime != "" {
		return date + "T" + startTime + ":00"
	}

	return date
}

func planProposalID(input *PlanInput) string {
	raw := input.AthleteID + "|" + input.Intent.Oldest + "|" + input.Intent.Newest + "|" + strconv.Itoa(len(input.Intent.Sessions))
	sum := sha256.Sum256([]byte(raw))

	return "plan-" + hex.EncodeToString(sum[:])[:rebalanceHashPrefixLength]
}

func planWeekStart(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	weekday := int(parsed.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := parsed.AddDate(0, 0, -(weekday - 1))

	return start.Format("2006-01-02")
}

func sanitizePlanID(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, name)
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return "session"
	}
	if len(cleaned) > 32 {
		return cleaned[:32]
	}

	return cleaned
}

func appendUniqueStrings(dst []string, values ...string) []string {
	seen := map[string]bool{}
	for _, value := range dst {
		seen[value] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		dst = append(dst, value)
	}

	return dst
}
