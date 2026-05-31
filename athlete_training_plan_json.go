package icu

import (
	"encoding/json"
	"fmt"
)

type athleteTrainingPlanAlias AthleteTrainingPlan

func (plan *AthleteTrainingPlan) UnmarshalJSON(data []byte) error {
	var alias athleteTrainingPlanAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal training plan camel fields: %w", err)
	}

	*plan = AthleteTrainingPlan(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal training plan raw fields: %w", err)
	}

	copyRawStringIfSet(&plan.AthleteID, raw, "athlete_id")
	copyRawIntIfSet(&plan.PlanID, raw, "training_plan_id")
	copyRawStringIfSet(&plan.PlanStartDate, raw, "training_plan_start_date")
	copyRawStringIfSet(&plan.PlanLastApplied, raw, "training_plan_last_applied")

	return nil
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
