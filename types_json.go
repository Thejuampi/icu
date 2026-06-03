package icu

import (
	"encoding/json"
	"fmt"
)

type sportSettingsAlias SportSettings

func (settings *SportSettings) UnmarshalJSON(data []byte) error {
	var alias sportSettingsAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal sport settings camel fields: %w", err)
	}

	*settings = SportSettings(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal sport settings raw fields: %w", err)
	}

	settings.applySnakeFields(raw)

	return nil
}

func (settings *SportSettings) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawStringIfSet(&settings.AthleteID, raw, "athlete_id")
	copyRawIntIfSet(&settings.IndoorFTP, raw, "indoor_ftp")
	copyRawIntIfSet(&settings.WPrime, raw, "w_prime")
	copyRawIntIfSet(&settings.PMax, raw, "p_max")
	copyRawIntIfSet(&settings.MaxHR, raw, "max_hr")
	copyRawValueIfSet(&settings.PowerZones, raw, "power_zones")
	copyRawValueIfSet(&settings.HRZones, raw, "hr_zones")
	copyRawValueIfSet(&settings.PaceZones, raw, "pace_zones")
	copyRawFloatIfSet(&settings.ThresholdPace, raw, "threshold_pace")
	copyRawStringIfSet(&settings.PaceUnits, raw, "pace_units")
	copyRawStringIfSet(&settings.HRLoadType, raw, "hr_load_type")
	copyRawStringIfSet(&settings.PaceLoadType, raw, "pace_load_type")
	copyRawStringIfSet(&settings.GapModel, raw, "gap_model")
}

type dataCurveAlias DataCurve

func (curve *DataCurve) UnmarshalJSON(data []byte) error {
	var alias dataCurveAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal data curve camel fields: %w", err)
	}

	*curve = DataCurve(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal data curve raw fields: %w", err)
	}

	curve.applySnakeFields(raw)

	return nil
}

func (curve *DataCurve) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawStringIfSet(&curve.StartDate, raw, "start_date_local")
	copyRawStringIfSet(&curve.EndDate, raw, "end_date_local")
	copyRawIntIfSet(&curve.MovingTime, raw, "moving_time")
	copyRawIntIfSet(&curve.TrainingLoad, raw, "training_load")
	copyRawValueIfSet(&curve.InputPointIndexes, raw, "input_point_indexes")
}

type powerHRCurveAlias PowerHRCurve

func (curve *PowerHRCurve) UnmarshalJSON(data []byte) error {
	var alias powerHRCurveAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal power hr curve camel fields: %w", err)
	}

	*curve = PowerHRCurve(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal power hr curve raw fields: %w", err)
	}

	copyRawStringIfSet(&curve.AthleteID, raw, "athlete_id")
	copyRawIntIfSet(&curve.MinWatts, raw, "min_watts")
	copyRawIntIfSet(&curve.MaxWatts, raw, "max_watts")
	copyRawIntIfSet(&curve.BucketSize, raw, "bucket_size")
	copyRawIntIfSet(&curve.MaxHR, raw, "max_hr")

	return nil
}

type weatherSummaryAlias WeatherSummary

func (summary *WeatherSummary) UnmarshalJSON(data []byte) error {
	var alias weatherSummaryAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal weather summary camel fields: %w", err)
	}

	*summary = WeatherSummary(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal weather summary raw fields: %w", err)
	}

	summary.applySnakeFields(raw)

	return nil
}

func (summary *WeatherSummary) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawFloatIfSet(&summary.AvgTemp, raw, "average_temp")
	copyRawFloatIfSet(&summary.MinTemp, raw, "min_temp")
	copyRawFloatIfSet(&summary.MaxTemp, raw, "max_temp")
	copyRawFloatIfSet(&summary.AvgFeelsLike, raw, "average_feels_like")
	copyRawFloatIfSet(&summary.AvgWindSpeed, raw, "average_wind_speed")
	copyRawFloatIfSet(&summary.AvgWindGust, raw, "average_wind_gust")
	copyRawFloatIfSet(&summary.HeadwindPct, raw, "headwind_percent")
	copyRawFloatIfSet(&summary.TailwindPct, raw, "tailwind_percent")
}

type customItemAlias CustomItem

func (item *CustomItem) UnmarshalJSON(data []byte) error {
	var alias customItemAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal custom item camel fields: %w", err)
	}

	*item = CustomItem(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal custom item raw fields: %w", err)
	}

	copyRawStringIfSet(&item.AthleteID, raw, "athlete_id")

	return nil
}
