package icu

import (
	"encoding/json"
)

type sportSettingsAlias SportSettings

func (settings *SportSettings) UnmarshalJSON(data []byte) error {
	var alias sportSettingsAlias

	return unmarshalWithSnakeFields(
		data,
		settings,
		&alias,
		func(a sportSettingsAlias) SportSettings { return SportSettings(a) },
		(*SportSettings).applySnakeFields,
		nil,
	)
}

func (settings *SportSettings) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&settings.AthleteID, raw, "athlete_id")
	copyRawIfSet(&settings.IndoorFTP, raw, "indoor_ftp")
	copyRawIfSet(&settings.WPrime, raw, "w_prime")
	copyRawIfSet(&settings.PMax, raw, "p_max")
	copyRawIfSet(&settings.MaxHR, raw, "max_hr")
	copyRawValueIfSet(&settings.PowerZones, raw, "power_zones")
	copyRawValueIfSet(&settings.HRZones, raw, "hr_zones")
	copyRawValueIfSet(&settings.PaceZones, raw, "pace_zones")
	copyRawIfSet(&settings.ThresholdPace, raw, "threshold_pace")
	copyRawIfSet(&settings.PaceUnits, raw, "pace_units")
	copyRawIfSet(&settings.HRLoadType, raw, "hr_load_type")
	copyRawIfSet(&settings.PaceLoadType, raw, "pace_load_type")
	copyRawIfSet(&settings.GapModel, raw, "gap_model")
}

type dataCurveAlias DataCurve

func (curve *DataCurve) UnmarshalJSON(data []byte) error {
	var alias dataCurveAlias

	return unmarshalWithSnakeFields(
		data,
		curve,
		&alias,
		func(a dataCurveAlias) DataCurve { return DataCurve(a) },
		(*DataCurve).applySnakeFields,
		nil,
	)
}

func (curve *DataCurve) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&curve.StartDate, raw, "start_date_local")
	copyRawIfSet(&curve.EndDate, raw, "end_date_local")
	copyRawIfSet(&curve.MovingTime, raw, "moving_time")
	copyRawIfSet(&curve.TrainingLoad, raw, "training_load")
	copyRawValueIfSet(&curve.InputPointIndexes, raw, "input_point_indexes")
}

type powerHRCurveAlias PowerHRCurve

func (curve *PowerHRCurve) UnmarshalJSON(data []byte) error {
	var alias powerHRCurveAlias

	return unmarshalWithSnakeFields(
		data,
		curve,
		&alias,
		func(a powerHRCurveAlias) PowerHRCurve { return PowerHRCurve(a) },
		(*PowerHRCurve).applySnakeFields,
		nil,
	)
}

func (curve *PowerHRCurve) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&curve.AthleteID, raw, "athlete_id")
	copyRawIfSet(&curve.MinWatts, raw, "min_watts")
	copyRawIfSet(&curve.MaxWatts, raw, "max_watts")
	copyRawIfSet(&curve.BucketSize, raw, "bucket_size")
	copyRawIfSet(&curve.MaxHR, raw, "max_hr")
}

type weatherSummaryAlias WeatherSummary

func (summary *WeatherSummary) UnmarshalJSON(data []byte) error {
	var alias weatherSummaryAlias

	return unmarshalWithSnakeFields(
		data,
		summary,
		&alias,
		func(a weatherSummaryAlias) WeatherSummary { return WeatherSummary(a) },
		(*WeatherSummary).applySnakeFields,
		nil,
	)
}

func (summary *WeatherSummary) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&summary.AvgTemp, raw, "average_temp")
	copyRawIfSet(&summary.MinTemp, raw, "min_temp")
	copyRawIfSet(&summary.MaxTemp, raw, "max_temp")
	copyRawIfSet(&summary.AvgFeelsLike, raw, "average_feels_like")
	copyRawIfSet(&summary.AvgWindSpeed, raw, "average_wind_speed")
	copyRawIfSet(&summary.AvgWindGust, raw, "average_wind_gust")
	copyRawIfSet(&summary.HeadwindPct, raw, "headwind_percent")
	copyRawIfSet(&summary.TailwindPct, raw, "tailwind_percent")
}

type customItemAlias CustomItem

func (item *CustomItem) UnmarshalJSON(data []byte) error {
	var alias customItemAlias

	return unmarshalWithSnakeFields(
		data,
		item,
		&alias,
		func(a customItemAlias) CustomItem { return CustomItem(a) },
		(*CustomItem).applySnakeFields,
		nil,
	)
}

func (item *CustomItem) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&item.AthleteID, raw, "athlete_id")
}
