package icu_test

import (
	"encoding/json"
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
