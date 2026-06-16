package icu_test

import (
	"encoding/json"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const testPlanStartDate = "2026-06-01"

func TestAthleteTrainingPlanUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var plan icu.AthleteTrainingPlan

	data := []byte(`{"athlete_id":"i445643","training_plan_id":827693,` +
		`"training_plan_start_date":"` + testPlanStartDate + `",` +
		`"training_plan_last_applied":"2026-05-31T17:00:00Z"}`)

	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("json.Unmarshal AthleteTrainingPlan: %v", err)
	}

	if plan.AthleteID != "i445643" || plan.PlanID != 827693 ||
		plan.PlanStartDate != testPlanStartDate || plan.PlanLastApplied != "2026-05-31T17:00:00Z" {
		t.Fatalf("AthleteTrainingPlan = %+v, want snake-case fields", plan)
	}
}

func TestAthleteTrainingPlanUpdateMarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(icu.AthleteTrainingPlanUpdate{
		ID:        "i445643",
		PlanID:    827693,
		StartDate: testPlanStartDate,
		PlanAlias: "ALT Build Jun 2026",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)
	want := `{"id":"i445643","training_plan_id":827693,"training_plan_start_date":"` + testPlanStartDate + `","training_plan_alias":"ALT Build Jun 2026"}`

	if got != want {
		t.Fatalf("AthleteTrainingPlanUpdate JSON = %s, want %s", got, want)
	}
}
