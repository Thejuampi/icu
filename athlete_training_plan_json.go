package icu

import (
	"encoding/json"
	"fmt"
)

type athleteTrainingPlanAlias AthleteTrainingPlan

func (plan *AthleteTrainingPlan) UnmarshalJSON(data []byte) error {
	var alias athleteTrainingPlanAlias

	return unmarshalWithSnakeFields(
		data,
		plan,
		&alias,
		func(a athleteTrainingPlanAlias) AthleteTrainingPlan { return AthleteTrainingPlan(a) },
		(*AthleteTrainingPlan).applySnakeFields,
		nil,
	)
}

func (plan *AthleteTrainingPlan) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&plan.AthleteID, raw, "athlete_id")
	copyRawIfSet(&plan.PlanID, raw, "training_plan_id")
	copyRawIfSet(&plan.PlanStartDate, raw, "training_plan_start_date")
	copyRawIfSet(&plan.PlanLastApplied, raw, "training_plan_last_applied")
}

//nolint:tagliatelle // wire format matches external API snake_case contract
type athleteTrainingPlanUpdateWire struct {
	ID        string `json:"id,omitempty"`
	PlanID    int    `json:"training_plan_id,omitempty"`
	StartDate string `json:"training_plan_start_date,omitempty"`
	PlanAlias string `json:"training_plan_alias,omitempty"`
}

func (update AthleteTrainingPlanUpdate) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(athleteTrainingPlanUpdateWire(update))
	if err != nil {
		return nil, fmt.Errorf("marshal AthleteTrainingPlanUpdate: %w", err)
	}

	return data, nil
}
