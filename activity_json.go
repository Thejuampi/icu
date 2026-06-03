package icu

import (
	"encoding/json"
	"fmt"
)

const intensityPercentThreshold = 2

type activityAlias Activity

func (activity *Activity) UnmarshalJSON(data []byte) error {
	var alias activityAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshal activity camel fields: %w", err)
	}

	*activity = Activity(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal activity raw fields: %w", err)
	}

	activity.applySnakeFields(raw)
	activity.Intensity = normalizeActivityIntensity(activity.Intensity)

	return nil
}

func (activity *Activity) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawStringIfSet(&activity.StartDateLocal, raw, "start_date_local")
	copyRawStringIfSet(&activity.StartDate, raw, "start_date")
	copyRawIntIfSet(&activity.MovingTime, raw, "moving_time")
	copyRawIntIfSet(&activity.ElapsedTime, raw, "elapsed_time")
	copyRawFloatIfSet(&activity.TotalElevationGain, raw, "total_elevation_gain")
	copyRawIntIfSet(&activity.AverageHeartRate, raw, "average_heartrate")
	copyRawIntIfSet(&activity.MaxHeartRate, raw, "max_heartrate")
	copyRawIntIfSet(&activity.AveragePower, raw, "icu_average_watts")
	copyRawIntIfSet(&activity.WeightedAvgPower, raw, "icu_weighted_avg_watts")
	copyRawIntIfSet(&activity.TrainingLoad, raw, "icu_training_load")
	copyRawFloatIfSet(&activity.Intensity, raw, "icu_intensity")
	copyRawIntIfSet(&activity.FTP, raw, "icu_ftp")
	copyRawIntIfSet(&activity.CriticalPower, raw, "icu_pm_cp")
	copyRawIntIfSet(&activity.WPrime, raw, "icu_pm_w_prime")
	copyRawIntIfSet(&activity.PMax, raw, "icu_pm_p_max")
	copyRawIntIfSet(&activity.FTPWatts, raw, "icu_pm_ftp")
	copyRawIntIfSet(&activity.RollingFTP, raw, "icu_rolling_ftp")
	copyRawIntIfSet(&activity.JoulesAboveFTP, raw, "icu_joules_above_ftp")
	copyRawIntIfSet(&activity.MaxWbalDepletion, raw, "icu_max_wbal_depletion")
	copyRawFloatIfSet(&activity.EfficiencyFactor, raw, "icu_efficiency_factor")
	copyRawFloatIfSet(&activity.VariabilityIndex, raw, "icu_variability_index")
	copyRawFloatIfSet(&activity.PowerHR, raw, "icu_power_hr")
	copyRawFloatIfSet(&activity.PowerHRZ2, raw, "icu_power_hr_z2")
	copyRawIntIfSet(&activity.PowerHRZ2Mins, raw, "icu_power_hr_z2_mins")
	copyRawIntIfSet(&activity.CadenceZ2, raw, "icu_cadence_z2")
	activity.applyEnvironmentSnakeFields(raw)
	copyRawFloatIfSet(&activity.ATL, raw, "icu_atl")
	copyRawFloatIfSet(&activity.CTL, raw, "icu_ctl")
	copyRawValueIfSet(&activity.ZoneTimes, raw, "icu_zone_times")
	copyRawValueIfSet(&activity.HRZoneTimes, raw, "icu_hr_zone_times")
}

func (activity *Activity) applyEnvironmentSnakeFields(raw map[string]json.RawMessage) {
	copyRawFloatIfSet(&activity.AverageTemp, raw, "average_temp")
	copyRawFloatIfSet(&activity.MinTemp, raw, "min_temp")
	copyRawFloatIfSet(&activity.MaxTemp, raw, "max_temp")
	copyRawFloatIfSet(&activity.AverageWeatherTemp, raw, "average_weather_temp")
	copyRawFloatIfSet(&activity.MinWeatherTemp, raw, "min_weather_temp")
	copyRawFloatIfSet(&activity.MaxWeatherTemp, raw, "max_weather_temp")
	copyRawFloatIfSet(&activity.AverageFeelsLike, raw, "average_feels_like")
	copyRawFloatIfSet(&activity.AverageWindSpeed, raw, "average_wind_speed")
	copyRawFloatIfSet(&activity.AverageWindGust, raw, "average_wind_gust")
	copyRawIntIfSet(&activity.PrevailingWindDeg, raw, "prevailing_wind_deg")
	copyRawFloatIfSet(&activity.HeadwindPercent, raw, "headwind_percent")
	copyRawFloatIfSet(&activity.TailwindPercent, raw, "tailwind_percent")
	copyRawFloatIfSet(&activity.AverageAltitude, raw, "average_altitude")
	copyRawFloatIfSet(&activity.MinAltitude, raw, "min_altitude")
	copyRawFloatIfSet(&activity.MaxAltitude, raw, "max_altitude")
	copyRawFloatIfSet(&activity.AverageGradient, raw, "average_gradient")
	copyRawFloatIfSet(&activity.AverageLactate, raw, "average_lactate")
	copyRawFloatIfSet(&activity.MinLactate, raw, "min_lactate")
	copyRawFloatIfSet(&activity.MaxLactate, raw, "max_lactate")
	copyRawFloatIfSet(&activity.AverageYaw, raw, "average_yaw")
	copyRawInt64IfSet(&activity.RouteID, raw, "route_id")
	copyRawFloatIfSet(&activity.StrainScore, raw, "strain_score")
}

func copyRawInt64IfSet(target *int64, raw map[string]json.RawMessage, key string) {
	var value int64
	if decodeRawIfPresent(raw, key, &value) && value != 0 {
		*target = value
	}
}

func normalizeActivityIntensity(value float64) float64 {
	if value > intensityPercentThreshold {
		return round3(value / percentScale)
	}

	return round3(value)
}

func copyRawStringIfSet(target *string, raw map[string]json.RawMessage, key string) {
	var value string
	if decodeRawIfPresent(raw, key, &value) && value != "" {
		*target = value
	}
}

func copyRawIntIfSet(target *int, raw map[string]json.RawMessage, key string) {
	var value int
	if decodeRawIfPresent(raw, key, &value) && value != 0 {
		*target = value
	}
}

func copyRawFloatIfSet(target *float64, raw map[string]json.RawMessage, key string) {
	var value float64
	if decodeRawIfPresent(raw, key, &value) && value != 0 {
		*target = value
	}
}

func copyRawBoolIfSet(target *bool, raw map[string]json.RawMessage, key string) {
	var value bool
	if decodeRawIfPresent(raw, key, &value) && value {
		*target = value
	}
}

func copyRawAnyIfSet(target *any, raw map[string]json.RawMessage, key string) {
	var value any
	if decodeRawIfPresent(raw, key, &value) && value != nil {
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
