package icu_test

import (
	"encoding/json"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestActivityUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"start_date_local":"2026-05-30T09:53:54","moving_time":8355,` +
		`"icu_training_load":124,"icu_intensity":72.98245,` +
		`"icu_weighted_avg_watts":208,"icu_ctl":53.70188,"icu_atl":67.18892}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if activity.StartDateLocal != "2026-05-30T09:53:54" || activity.MovingTime != 8355 ||
		activity.TrainingLoad != 124 || activity.Intensity != 0.73 {
		t.Fatalf("Activity = %+v, want snake-case fields and normalized intensity", activity)
	}
}
