package icu

import (
	"encoding/json"
)

type eventAlias Event

func (event *Event) UnmarshalJSON(data []byte) error {
	var alias eventAlias

	return unmarshalWithSnakeFields(
		data,
		event,
		&alias,
		func(e eventAlias) Event { return Event(e) },
		(*Event).applySnakeFields,
		func(e *Event) { e.Intensity = normalizeActivityIntensity(e.Intensity) },
	)
}

func (event *Event) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&event.StartDateLocal, raw, "start_date_local")
	copyRawIfSet(&event.EndDateLocal, raw, "end_date_local")
	copyRawIfSet(&event.TrainingLoad, raw, "icu_training_load")
	copyRawIfSet(&event.MovingTime, raw, "moving_time")
	copyRawIfSet(&event.FTP, raw, "icu_ftp")
	copyRawIfSet(&event.ATL, raw, "icu_atl")
	copyRawIfSet(&event.CTL, raw, "icu_ctl")
	copyRawIfSet(&event.CalendarID, raw, "calendar_id")
	copyRawIfSet(&event.ExternalID, raw, "external_id")
	copyRawIfSet(&event.HideFromAthlete, raw, "hide_from_athlete")
	copyRawIfSet(&event.AthleteCannotEdit, raw, "athlete_cannot_edit")
	copyRawIfSet(&event.Intensity, raw, "icu_intensity")
	copyRawIfSet(&event.StrainScore, raw, "strain_score")
	copyRawIfSet(&event.PlanApplied, raw, "plan_applied")
	copyRawIfSet(&event.CreatedByID, raw, "created_by_id")
	copyRawIfSet(&event.SharedEventID, raw, "shared_event_id")
	copyRawIfSet(&event.LoadTarget, raw, "load_target")
	copyRawIfSet(&event.TimeTarget, raw, "time_target")
	copyRawIfSet(&event.DistanceTarget, raw, "distance_target")
	copyRawIfSet(&event.TrainingAvailability, raw, "training_availability")
	copyRawIfSet(&event.MaxTrainingTime, raw, "max_training_time")
	copyRawValueIfSet(&event.CanTrainSports, raw, "can_train_sports")
	copyRawAnyIfSet(&event.WorkoutDoc, raw, "workout_doc")
	copyRawIfSet(&event.WorkoutFileBase64, raw, "workout_file_base64")
	copyRawIfSet(&event.WorkoutFilename, raw, "workout_filename")
}
