package icu

import (
	"encoding/json"
	"fmt"
)

const intensityPercentThreshold = 2

type activityAlias Activity

func (activity *Activity) UnmarshalJSON(data []byte) error {
	var alias activityAlias

	return unmarshalWithSnakeFields(
		data,
		activity,
		&alias,
		func(a activityAlias) Activity { return Activity(a) },
		(*Activity).applySnakeFields,
		func(a *Activity) { a.Intensity = normalizeActivityIntensity(a.Intensity) },
	)
}

func (activity *Activity) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&activity.StartDateLocal, raw, "start_date_local")
	copyRawIfSet(&activity.StartDate, raw, "start_date")
	copyRawIfSet(&activity.MovingTime, raw, "moving_time")
	copyRawIfSet(&activity.ElapsedTime, raw, "elapsed_time")
	copyRawIfSet(&activity.TotalElevationGain, raw, "total_elevation_gain")
	copyRawIfSet(&activity.AverageHeartRate, raw, "average_heartrate")
	copyRawIfSet(&activity.MaxHeartRate, raw, "max_heartrate")
	copyRawIfSet(&activity.AveragePower, raw, "icu_average_watts")
	copyRawIfSet(&activity.WeightedAvgPower, raw, "icu_weighted_avg_watts")
	copyRawIfSet(&activity.TrainingLoad, raw, "icu_training_load")
	copyRawIfSet(&activity.HRLoad, raw, "hr_load")
	copyRawIfSet(&activity.HRLoadType, raw, "hr_load_type")
	copyRawIfSet(&activity.TRIMP, raw, "trimp")
	copyRawIfSet(&activity.Intensity, raw, "icu_intensity")
	copyRawIfSet(&activity.FTP, raw, "icu_ftp")
	copyRawIfSet(&activity.CriticalPower, raw, "icu_pm_cp")
	copyRawIfSet(&activity.WPrime, raw, "icu_pm_w_prime")
	copyRawIfSet(&activity.PMax, raw, "icu_pm_p_max")
	copyRawIfSet(&activity.FTPWatts, raw, "icu_pm_ftp")
	copyRawIfSet(&activity.RollingFTP, raw, "icu_rolling_ftp")
	copyRawIfSet(&activity.JoulesAboveFTP, raw, "icu_joules_above_ftp")
	copyRawIfSet(&activity.MaxWbalDepletion, raw, "icu_max_wbal_depletion")
	copyRawIfSet(&activity.EfficiencyFactor, raw, "icu_efficiency_factor")
	copyRawIfSet(&activity.VariabilityIndex, raw, "icu_variability_index")
	copyRawIfSet(&activity.PowerHR, raw, "icu_power_hr")
	copyRawIfSet(&activity.PowerHRZ2, raw, "icu_power_hr_z2")
	copyRawIfSet(&activity.PowerHRZ2Mins, raw, "icu_power_hr_z2_mins")
	copyRawIfSet(&activity.CadenceZ2, raw, "icu_cadence_z2")
	activity.applyEnvironmentSnakeFields(raw)
	copyRawIfSet(&activity.ATL, raw, "icu_atl")
	copyRawIfSet(&activity.CTL, raw, "icu_ctl")
	copyRawValueIfSet(&activity.ZoneTimes, raw, "icu_zone_times")
	copyRawValueIfSet(&activity.HRZoneTimes, raw, "icu_hr_zone_times")
}

func (activity *Activity) applyEnvironmentSnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&activity.AverageTemp, raw, "average_temp")
	copyRawIfSet(&activity.MinTemp, raw, "min_temp")
	copyRawIfSet(&activity.MaxTemp, raw, "max_temp")
	copyRawIfSet(&activity.AverageWeatherTemp, raw, "average_weather_temp")
	copyRawIfSet(&activity.MinWeatherTemp, raw, "min_weather_temp")
	copyRawIfSet(&activity.MaxWeatherTemp, raw, "max_weather_temp")
	copyRawIfSet(&activity.AverageFeelsLike, raw, "average_feels_like")
	copyRawIfSet(&activity.AverageWindSpeed, raw, "average_wind_speed")
	copyRawIfSet(&activity.AverageWindGust, raw, "average_wind_gust")
	copyRawIfSet(&activity.PrevailingWindDeg, raw, "prevailing_wind_deg")
	copyRawIfSet(&activity.HeadwindPercent, raw, "headwind_percent")
	copyRawIfSet(&activity.TailwindPercent, raw, "tailwind_percent")
	copyRawIfSet(&activity.AverageAltitude, raw, "average_altitude")
	copyRawIfSet(&activity.MinAltitude, raw, "min_altitude")
	copyRawIfSet(&activity.MaxAltitude, raw, "max_altitude")
	copyRawIfSet(&activity.AverageGradient, raw, "average_gradient")
	copyRawIfSet(&activity.AverageLactate, raw, "average_lactate")
	copyRawIfSet(&activity.MinLactate, raw, "min_lactate")
	copyRawIfSet(&activity.MaxLactate, raw, "max_lactate")
	copyRawIfSet(&activity.AverageYaw, raw, "average_yaw")
	copyRawIfSet(&activity.RouteID, raw, "route_id")
	copyRawIfSet(&activity.StrainScore, raw, "strain_score")
}

func normalizeActivityIntensity(value float64) float64 {
	if value > intensityPercentThreshold {
		return round3(value / percentScale)
	}

	return round3(value)
}

func copyRawAnyIfSet(target *any, raw map[string]json.RawMessage, key string) {
	var value any
	if decodeRawIfPresent(raw, key, &value) && value != nil {
		*target = value
	}
}

func copyRawIfSet[T comparable](target *T, raw map[string]json.RawMessage, key string) {
	var value T
	// If the snake_case key is present (and not null), always assign — including
	// explicit zeros/false/"" — so dual-key payloads cannot keep a stale camelCase value.
	if decodeRawIfPresent(raw, key, &value) {
		*target = value
	}
}

func copyRawValueIfSet[T any](target *T, raw map[string]json.RawMessage, key string) {
	var value T
	if decodeRawIfPresent(raw, key, &value) {
		*target = value
	}
}

func decodeRawIfPresent[T any](raw map[string]json.RawMessage, key string, target *T) bool {
	value, ok := raw[key]
	if !ok || string(value) == "null" {
		return false
	}

	if err := json.Unmarshal(value, target); err != nil {
		return false
	}

	return true
}

func unmarshalWithSnakeFields[T, A any](
	data []byte,
	target *T,
	alias *A,
	convert func(A) T,
	apply func(*T, map[string]json.RawMessage),
	normalize func(*T),
) error {
	if err := json.Unmarshal(data, alias); err != nil {
		return fmt.Errorf("unmarshal camel fields: %w", err)
	}

	*target = convert(*alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal raw fields: %w", err)
	}

	apply(target, raw)

	if normalize != nil {
		normalize(target)
	}

	return nil
}
