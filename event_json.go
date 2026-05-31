package icu

import (
	"encoding/json"
	"fmt"
)

type eventAlias Event

func (event *Event) UnmarshalJSON(data []byte) error {
	var alias eventAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal event camel fields: %w", err)
	}

	*event = Event(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal event raw fields: %w", err)
	}

	event.applySnakeFields(raw)
	event.Intensity = normalizeActivityIntensity(event.Intensity)

	return nil
}

func (event *Event) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawStringIfSet(&event.StartDateLocal, raw, "start_date_local")
	copyRawStringIfSet(&event.EndDateLocal, raw, "end_date_local")
	copyRawIntIfSet(&event.TrainingLoad, raw, "icu_training_load")
	copyRawIntIfSet(&event.MovingTime, raw, "moving_time")
	copyRawIntIfSet(&event.FTP, raw, "icu_ftp")
	copyRawFloatIfSet(&event.ATL, raw, "icu_atl")
	copyRawFloatIfSet(&event.CTL, raw, "icu_ctl")
	copyRawIntIfSet(&event.CalendarID, raw, "calendar_id")
	copyRawStringIfSet(&event.ExternalID, raw, "external_id")
	copyRawBoolIfSet(&event.HideFromAthlete, raw, "hide_from_athlete")
	copyRawBoolIfSet(&event.AthleteCannotEdit, raw, "athlete_cannot_edit")
	copyRawFloatIfSet(&event.Intensity, raw, "icu_intensity")
	copyRawFloatIfSet(&event.StrainScore, raw, "strain_score")
	copyRawAnyIfSet(&event.WorkoutDoc, raw, "workout_doc")
	copyRawStringIfSet(&event.WorkoutFileBase64, raw, "workout_file_base64")
	copyRawStringIfSet(&event.WorkoutFilename, raw, "workout_filename")
}
