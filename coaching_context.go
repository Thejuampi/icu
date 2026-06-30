package icu

import "strings"

const (
	coachingCommand         = "analysis coaching"
	coachingNoteCategory    = "NOTE"
	coachingSourceAthlete   = "athlete"
	coachingSourceSport     = "sport-settings"
	coachingSourceActivity  = "activities"
	coachingSourceWellness  = "wellness"
	coachingSourceEvents    = "events"
	coachingSourcePlan      = "plan-analysis"
	coachingSourceAdapt     = "adaptation-analysis"
	coachingMissingAthlete  = "athlete"
	coachingMissingSport    = "sportSettings"
	coachingMissingCycling  = "analyses.cycling"
	coachingMissingWellness = "analyses.wellness"
	coachingMissingPlan     = "analyses.plan"
	coachingSourceCapacity  = 7
	coachingMissingCapacity = 5
)

type CoachingContextInputs struct {
	Athlete       *Athlete                   `json:"-"`
	SportSettings *SportSettings             `json:"-"`
	Cycling       *CyclingAnalysis           `json:"-"`
	Wellness      *WellnessAnalysis          `json:"-"`
	Plan          *TrainingPlanAnalysis      `json:"-"`
	Adaptation    *CyclingAdaptationAnalysis `json:"-"`
	Events        []Event                    `json:"-"`
}

type CoachingContextOptions struct {
	SportType         string `json:"sportType,omitempty"`
	HistoryStartDate  string `json:"historyStartDate,omitempty"`
	HistoryEndDate    string `json:"historyEndDate,omitempty"`
	PlanStartDate     string `json:"planStartDate,omitempty"`
	PlanEndDate       string `json:"planEndDate,omitempty"`
	Timezone          string `json:"timezone,omitempty"`
	TimezoneSource    string `json:"timezoneSource,omitempty"`
	IncludeAdaptation bool   `json:"includeAdaptation,omitempty"`
}

type CoachingContext struct {
	Scope         CoachingContextScope       `json:"scope"`
	Athlete       *Athlete                   `json:"athlete,omitempty"`
	SportSettings *SportSettings             `json:"sportSettings,omitempty"`
	Analyses      CoachingContextAnalyses    `json:"analyses"`
	Calendar      CoachingCalendarContext    `json:"calendar"`
	DataQuality   CoachingContextDataQuality `json:"dataQuality"`
	SideEffects   CoachingContextSideEffects `json:"sideEffects"`
}

type CoachingContextScope struct {
	Command          string `json:"command"`
	SportType        string `json:"sportType,omitempty"`
	HistoryStartDate string `json:"historyStartDate,omitempty"`
	HistoryEndDate   string `json:"historyEndDate,omitempty"`
	PlanStartDate    string `json:"planStartDate,omitempty"`
	PlanEndDate      string `json:"planEndDate,omitempty"`
	Timezone         string `json:"timezone,omitempty"`
	TimezoneSource   string `json:"timezoneSource,omitempty"`
}

type CoachingContextAnalyses struct {
	Cycling    *CyclingAnalysis           `json:"cycling,omitempty"`
	Wellness   *WellnessAnalysis          `json:"wellness,omitempty"`
	Plan       *TrainingPlanAnalysis      `json:"plan,omitempty"`
	Adaptation *CyclingAdaptationAnalysis `json:"adaptation,omitempty"`
}

type CoachingCalendarContext struct {
	Events []Event                `json:"events"`
	Notes  []CoachingCalendarNote `json:"notes"`
}

type CoachingCalendarNote struct {
	ID             int    `json:"id,omitempty"`
	Date           string `json:"date,omitempty"`
	StartDateLocal string `json:"startDateLocal,omitempty"`
	EndDateLocal   string `json:"endDateLocal,omitempty"`
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
}

type CoachingContextDataQuality struct {
	Sources  []string `json:"sources"`
	Warnings []string `json:"warnings"`
	Missing  []string `json:"missing"`
}

type CoachingContextSideEffects struct {
	MutatedPlan        bool `json:"mutatedPlan"`
	MutatedConfig      bool `json:"mutatedConfig"`
	SyncedExternalData bool `json:"syncedExternalData"`
}

func BuildCoachingContext(inputs CoachingContextInputs, options *CoachingContextOptions) CoachingContext {
	if options == nil {
		options = &CoachingContextOptions{}
	}

	context := CoachingContext{
		Scope: CoachingContextScope{
			Command:          coachingCommand,
			SportType:        options.SportType,
			HistoryStartDate: options.HistoryStartDate,
			HistoryEndDate:   options.HistoryEndDate,
			PlanStartDate:    options.PlanStartDate,
			PlanEndDate:      options.PlanEndDate,
			Timezone:         options.Timezone,
			TimezoneSource:   options.TimezoneSource,
		},
		Athlete:       inputs.Athlete,
		SportSettings: inputs.SportSettings,
		Analyses: CoachingContextAnalyses{
			Cycling:  inputs.Cycling,
			Wellness: inputs.Wellness,
			Plan:     inputs.Plan,
		},
		Calendar: CoachingCalendarContext{
			Events: append([]Event{}, inputs.Events...),
			Notes:  coachingNotes(inputs.Events),
		},
		DataQuality: CoachingContextDataQuality{
			Sources:  coachingSources(inputs, options),
			Warnings: coachingWarnings(inputs, options),
			Missing:  coachingMissing(inputs),
		},
		SideEffects: CoachingContextSideEffects{},
	}

	if options.IncludeAdaptation {
		context.Analyses.Adaptation = inputs.Adaptation
	}

	return context
}

func coachingNotes(events []Event) []CoachingCalendarNote {
	notes := make([]CoachingCalendarNote, 0)
	for index := range events {
		event := &events[index]
		if !strings.EqualFold(event.Category, coachingNoteCategory) {
			continue
		}

		notes = append(notes, CoachingCalendarNote{
			ID:             event.ID,
			Date:           coachingEventDate(event.StartDateLocal),
			StartDateLocal: event.StartDateLocal,
			EndDateLocal:   event.EndDateLocal,
			Name:           event.Name,
			Description:    event.Description,
		})
	}

	return notes
}

func coachingEventDate(value string) string {
	if len(value) < len("2006-01-02") {
		return value
	}

	return value[:len("2006-01-02")]
}

func coachingSources(inputs CoachingContextInputs, options *CoachingContextOptions) []string {
	sources := make([]string, 0, coachingSourceCapacity)
	if inputs.Athlete != nil {
		sources = append(sources, coachingSourceAthlete)
	}
	if inputs.SportSettings != nil {
		sources = append(sources, coachingSourceSport)
	}
	if inputs.Cycling != nil {
		sources = append(sources, coachingSourceActivity)
	}
	if inputs.Wellness != nil {
		sources = append(sources, coachingSourceWellness)
	}
	if len(inputs.Events) > 0 {
		sources = append(sources, coachingSourceEvents)
	}
	if inputs.Plan != nil {
		sources = append(sources, coachingSourcePlan)
	}
	if options.IncludeAdaptation && inputs.Adaptation != nil {
		sources = append(sources, coachingSourceAdapt)
	}

	return sources
}

func coachingWarnings(inputs CoachingContextInputs, options *CoachingContextOptions) []string {
	warnings := make([]string, 0)
	if options.IncludeAdaptation && inputs.Adaptation == nil {
		warnings = appendUniqueWarning(warnings, "adaptation requested but unavailable")
	}
	if len(inputs.Events) == 0 {
		warnings = appendUniqueWarning(warnings, "calendar events unavailable or empty")
	}
	if inputs.Cycling != nil {
		warnings = appendPrefixedWarnings(warnings, "cycling", inputs.Cycling.Warnings)
	}
	if inputs.Wellness != nil {
		warnings = appendPrefixedWarnings(warnings, "wellness", inputs.Wellness.Warnings)
	}
	if inputs.Plan != nil {
		warnings = appendPrefixedWarnings(warnings, "plan", inputs.Plan.Warnings)
	}
	if options.IncludeAdaptation && inputs.Adaptation != nil {
		warnings = appendPrefixedWarnings(warnings, "adaptation", inputs.Adaptation.Warnings)
	}

	return warnings
}

func appendPrefixedWarnings(destination []string, source string, warnings []string) []string {
	for _, warning := range warnings {
		destination = appendUniqueWarning(destination, source+": "+warning)
	}

	return destination
}

func appendUniqueWarning(warnings []string, candidate string) []string {
	for _, warning := range warnings {
		if warning == candidate {
			return warnings
		}
	}

	return append(warnings, candidate)
}

func coachingMissing(inputs CoachingContextInputs) []string {
	missing := make([]string, 0, coachingMissingCapacity)
	if inputs.Athlete == nil {
		missing = append(missing, coachingMissingAthlete)
	}
	if inputs.SportSettings == nil {
		missing = append(missing, coachingMissingSport)
	}
	if inputs.Cycling == nil {
		missing = append(missing, coachingMissingCycling)
	}
	if inputs.Wellness == nil {
		missing = append(missing, coachingMissingWellness)
	}
	if inputs.Plan == nil {
		missing = append(missing, coachingMissingPlan)
	}

	return missing
}
