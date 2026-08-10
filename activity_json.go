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
	activity.applyValidityAndLoadSnakeFields(raw)
	activity.applyEmitOnlySnakeFields(raw)
	activity.StubShape = isStravaStubShape(raw, activity.Source)
}

// applyEmitOnlySnakeFields carries wave-4's nine emit-only fields (D-1/A2)
// across the same snake/camel casing gap every other API field already
// crosses here. This wiring is plumbing only — it transports a raw value
// into its typed field and computes nothing from it; T4.3's containment
// check treats this file as part of Activity's own type definition, not as
// a consumer.
func (activity *Activity) applyEmitOnlySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&activity.PolarizationIndex, raw, "polarization_index")
	copyRawIfSet(&activity.SSCP, raw, "ss_cp")
	copyRawIfSet(&activity.SSWPrime, raw, "ss_w_prime")
	copyRawIfSet(&activity.SSPMax, raw, "ss_p_max")
	copyRawIfSet(&activity.CarbsUsed, raw, "carbs_used")
	copyRawIfSet(&activity.CarbsIngested, raw, "carbs_ingested")
	copyRawIfSet(&activity.RollingFTPDelta, raw, "icu_rolling_ftp_delta")
	copyRawIfSet(&activity.PowerField, raw, "power_field")
	activity.HRR = decodeHRRMeasurement(raw, "icu_hrr")
}

// decodeHRRMeasurement parses icu_hrr's nested object with the same
// snake-to-camel overlay Activity itself uses at the top level (F-24):
// HRRMeasurement's own JSON tags are camelCase to match this package's
// outward contract, so a direct json.Unmarshal of the raw (snake_case)
// object would only ever populate the one member whose tag happens to be
// spelled identically in both casings ("hrr"). Returns nil when the key is
// absent or explicitly null, matching every other nullable object field.
func decodeHRRMeasurement(raw map[string]json.RawMessage, key string) *HRRMeasurement {
	value, ok := raw[key]
	if !ok || string(value) == "null" {
		return nil
	}

	var nested map[string]json.RawMessage
	if err := json.Unmarshal(value, &nested); err != nil {
		return nil
	}

	measurement := &HRRMeasurement{}
	copyRawIfSet(&measurement.StartIndex, nested, "start_index")
	copyRawIfSet(&measurement.EndIndex, nested, "end_index")
	copyRawIfSet(&measurement.StartTime, nested, "start_time")
	copyRawIfSet(&measurement.EndTime, nested, "end_time")
	copyRawIfSet(&measurement.StartBPM, nested, "start_bpm")
	copyRawIfSet(&measurement.EndBPM, nested, "end_bpm")
	copyRawIfSet(&measurement.AverageWatts, nested, "average_watts")
	copyRawIfSet(&measurement.HRR, nested, "hrr")

	return measurement
}

// applyValidityAndLoadSnakeFields carries wave-2's newly-fetched fields
// (D-2 ignore flags, D-21 pairing, D-23 tri-load channels) across the same
// snake/camel casing gap every other API field already crosses here.
func (activity *Activity) applyValidityAndLoadSnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&activity.PairedEventID, raw, "paired_event_id")
	copyRawIfSet(&activity.IgnorePower, raw, "icu_ignore_power")
	copyRawIfSet(&activity.IgnoreHR, raw, "icu_ignore_hr")
	copyRawIfSet(&activity.IgnoreTime, raw, "icu_ignore_time")
	copyRawIfSet(&activity.PowerLoad, raw, "power_load")
	copyRawIfSet(&activity.PaceLoad, raw, "pace_load")
	copyRawIfSet(&activity.PaceLoadType, raw, "pace_load_type")
}

// stravaStubKeyCount is D-24's STRAVA-stub shape signature: a raw activity
// record reduced to exactly this many keys when Strava withheld the full
// record from the API.
const stravaStubKeyCount = 5

// isStravaStubShape discriminates on shape (D-24), never on absence of
// type, so a genuinely typeless-but-real record does not fall in here. The
// raw key count is not recoverable from the typed Activity after the fact,
// so this must run at unmarshal time while raw is still in hand.
func isStravaStubShape(raw map[string]json.RawMessage, source string) bool {
	if len(raw) != stravaStubKeyCount {
		return false
	}

	_, hasNote := raw["_note"]

	return hasNote && source == "STRAVA"
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
