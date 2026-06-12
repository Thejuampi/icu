package icu_test

import (
	"encoding/json"
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestEventUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var event icu.Event

	data := []byte(`{"start_date_local":"2026-06-04T00:00:00",` +
		`"end_date_local":"2026-06-05T00:00:00","icu_training_load":83,` +
		`"moving_time":5700,"icu_intensity":80.104095,"icu_ctl":51.354958,` +
		`"icu_atl":51.643852,"calendar_id":1,"external_id":"ext-1",` +
		`"athlete_cannot_edit":true}`)

	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("json.Unmarshal Event: %v", err)
	}

	if event.StartDateLocal != "2026-06-04T00:00:00" || event.TrainingLoad != 83 ||
		event.MovingTime != 5700 || event.Intensity != 0.801 {
		t.Fatalf("Event = %+v, want snake-case fields and normalized intensity", event)
	}
}

func TestEventUnmarshalTargetSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var event icu.Event

	data := []byte(`{"category":"RACE_A","start_date_local":"2026-07-05T08:00:00",` +
		`"plan_applied":true,"created_by_id":"i42","shared_event_id":771,` +
		`"load_target":240,"time_target":14400,"distance_target":165000,` +
		`"training_availability":"limited","max_training_time":5400,` +
		`"can_train_sports":["Ride","VirtualRide"]}`)

	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("json.Unmarshal Event: %v", err)
	}

	got := struct {
		PlanApplied          bool
		CreatedByID          string
		SharedEventID        int
		LoadTarget           int
		TimeTarget           int
		DistanceTarget       float64
		TrainingAvailability string
		MaxTrainingTime      int
		CanTrainSports       []string
	}{
		PlanApplied:          event.PlanApplied,
		CreatedByID:          event.CreatedByID,
		SharedEventID:        event.SharedEventID,
		LoadTarget:           event.LoadTarget,
		TimeTarget:           event.TimeTarget,
		DistanceTarget:       event.DistanceTarget,
		TrainingAvailability: event.TrainingAvailability,
		MaxTrainingTime:      event.MaxTrainingTime,
		CanTrainSports:       event.CanTrainSports,
	}
	want := struct {
		PlanApplied          bool
		CreatedByID          string
		SharedEventID        int
		LoadTarget           int
		TimeTarget           int
		DistanceTarget       float64
		TrainingAvailability string
		MaxTrainingTime      int
		CanTrainSports       []string
	}{true, "i42", 771, 240, 14400, 165000, "limited", 5400, []string{"Ride", "VirtualRide"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Event target fields = %+v, want %+v", got, want)
	}
}

func TestEventUnmarshalWorkoutDocSnakeField(t *testing.T) {
	t.Parallel()

	var event icu.Event

	data := []byte(`{"id":42,"workout_doc":{"name":"Test","duration":3600}}`)

	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
}
