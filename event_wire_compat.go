package icu

import (
	"encoding/json"
	"fmt"
)

// eventWireV1 mirrors Event field-for-field, in the same declaration order and
// with identical JSON tags, except Indoor stays a plain bool. It exists only
// so Event.MarshalJSON (below) can reproduce the pre-wave-1a wire format
// byte-for-byte while Event.Indoor is *bool internally (D-25, T1a.1).
//
// A shadowed/embedded-field trick was considered and rejected: encoding/json
// emits a field that shadows an embedded one at the *shadowing* field's
// declaration position, not the shadowed field's original position, which
// would move "indoor" to the end of the object and break byte-identity with
// the pre-1a wire form. A full field mirror is the only way that preserves
// key order under Go's encoding/json rules.
//
// Removed in wave-1b under the schema bump it already carries (D-7b).
type eventWireV1 struct {
	ID                   int      `json:"id,omitempty"`
	StartDateLocal       string   `json:"startDateLocal,omitempty"`
	EndDateLocal         string   `json:"endDateLocal,omitempty"`
	Category             string   `json:"category,omitempty"`
	Type                 string   `json:"type,omitempty"`
	Name                 string   `json:"name,omitempty"`
	Description          string   `json:"description,omitempty"`
	TrainingLoad         int      `json:"icuTrainingLoad,omitempty"`
	MovingTime           int      `json:"movingTime,omitempty"`
	Distance             float64  `json:"distance,omitempty"`
	Color                string   `json:"color,omitempty"`
	Indoor               bool     `json:"indoor,omitempty"`
	FTP                  int      `json:"icuFtp,omitempty"`
	ATL                  float64  `json:"icuAtl,omitempty"`
	CTL                  float64  `json:"icuCtl,omitempty"`
	Target               string   `json:"target,omitempty"`
	UID                  string   `json:"uid,omitempty"`
	CalendarID           int      `json:"calendarId,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
	ExternalID           string   `json:"externalId,omitempty"`
	HideFromAthlete      bool     `json:"hideFromAthlete,omitempty"`
	AthleteCannotEdit    bool     `json:"athleteCannotEdit,omitempty"`
	Intensity            float64  `json:"icuIntensity,omitempty"`
	StrainScore          float64  `json:"strainScore,omitempty"`
	PlanApplied          bool     `json:"planApplied,omitempty"`
	CreatedByID          string   `json:"createdById,omitempty"`
	SharedEventID        int      `json:"sharedEventId,omitempty"`
	LoadTarget           int      `json:"loadTarget,omitempty"`
	TimeTarget           int      `json:"timeTarget,omitempty"`
	DistanceTarget       float64  `json:"distanceTarget,omitempty"`
	TrainingAvailability string   `json:"trainingAvailability,omitempty"`
	MaxTrainingTime      int      `json:"maxTrainingTime,omitempty"`
	CanTrainSports       []string `json:"canTrainSports,omitempty"`
	WorkoutDoc           any      `json:"workoutDoc,omitempty"`
	WorkoutFileBase64    string   `json:"workoutFileBase64,omitempty"`
	WorkoutFilename      string   `json:"workoutFilename,omitempty"`
}

// MarshalJSON reproduces the pre-wave-1a wire format byte-for-byte: nil and
// false both omit the "indoor" key (unknown and confirmed-outdoor read the
// same on the wire); only a resolved true emits it (D-25's truth table).
//
// Value receiver only — on *Event this method is silently skipped for
// non-addressable values (a map[string]Event entry, an Event held in an
// interface, an Event field of a returned value), so output would differ in
// some containers and not others. A value receiver is reachable from every
// container shape (W1a-R03).
func (event Event) MarshalJSON() ([]byte, error) {
	wire := eventWireV1{
		ID:                   event.ID,
		StartDateLocal:       event.StartDateLocal,
		EndDateLocal:         event.EndDateLocal,
		Category:             event.Category,
		Type:                 event.Type,
		Name:                 event.Name,
		Description:          event.Description,
		TrainingLoad:         event.TrainingLoad,
		MovingTime:           event.MovingTime,
		Distance:             event.Distance,
		Color:                event.Color,
		Indoor:               event.Indoor != nil && *event.Indoor,
		FTP:                  event.FTP,
		ATL:                  event.ATL,
		CTL:                  event.CTL,
		Target:               event.Target,
		UID:                  event.UID,
		CalendarID:           event.CalendarID,
		Tags:                 event.Tags,
		ExternalID:           event.ExternalID,
		HideFromAthlete:      event.HideFromAthlete,
		AthleteCannotEdit:    event.AthleteCannotEdit,
		Intensity:            event.Intensity,
		StrainScore:          event.StrainScore,
		PlanApplied:          event.PlanApplied,
		CreatedByID:          event.CreatedByID,
		SharedEventID:        event.SharedEventID,
		LoadTarget:           event.LoadTarget,
		TimeTarget:           event.TimeTarget,
		DistanceTarget:       event.DistanceTarget,
		TrainingAvailability: event.TrainingAvailability,
		MaxTrainingTime:      event.MaxTrainingTime,
		CanTrainSports:       event.CanTrainSports,
		WorkoutDoc:           event.WorkoutDoc,
		WorkoutFileBase64:    event.WorkoutFileBase64,
		WorkoutFilename:      event.WorkoutFilename,
	}

	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal Event: %w", err)
	}

	return data, nil
}
