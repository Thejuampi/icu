package icu_test

import (
	"encoding/json"
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestActivityUnmarshalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"start_date_local":"2026-05-30T09:53:54","moving_time":8355,` +
		`"icu_training_load":124,"icu_intensity":72.98245,"hr_load":139,"hr_load_type":"HRSS","trimp":215.5,` +
		`"icu_weighted_avg_watts":208,"icu_ctl":53.70188,"icu_atl":67.18892}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if activity.StartDateLocal != "2026-05-30T09:53:54" || activity.MovingTime != 8355 ||
		activity.TrainingLoad != 124 || activity.Intensity != 0.73 || activity.HRLoad != 139 ||
		activity.HRLoadType != "HRSS" || activity.TRIMP != 215.5 {
		t.Fatalf("Activity = %+v, want snake-case fields and normalized intensity", activity)
	}
}

func TestActivityUnmarshalCamelCaseHRLoadFields(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"hrLoad":145,"hrLoadType":"HRSS","trimp":233.91678}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if activity.HRLoad != 145 || activity.HRLoadType != "HRSS" || activity.TRIMP != 233.91678 {
		t.Fatalf("Activity = %+v, want camel-case HR load fields", activity)
	}
}

func TestActivityUnmarshalEnvironmentalSnakeCaseFields(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"average_temp":31.4,"min_temp":22.1,"max_temp":35.2,` +
		`"average_weather_temp":30.3,"min_weather_temp":21.5,"max_weather_temp":34.6,` +
		`"average_feels_like":33.8,"average_wind_speed":18.4,"average_wind_gust":32.7,` +
		`"prevailing_wind_deg":185,"headwind_percent":42.5,"tailwind_percent":21.2,` +
		`"average_altitude":514.8,"min_altitude":480.2,"max_altitude":731.9,` +
		`"average_gradient":3.7,"average_lactate":2.4,"min_lactate":1.1,"max_lactate":5.8,` +
		`"average_yaw":7.6,"route_id":12345,"strain_score":68.9}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	got := struct {
		AverageTemp        float64
		MinTemp            float64
		MaxTemp            float64
		AverageWeatherTemp float64
		MinWeatherTemp     float64
		MaxWeatherTemp     float64
		AverageFeelsLike   float64
		AverageWindSpeed   float64
		AverageWindGust    float64
		PrevailingWindDeg  int
		HeadwindPercent    float64
		TailwindPercent    float64
		AverageAltitude    float64
		MinAltitude        float64
		MaxAltitude        float64
		AverageGradient    float64
		AverageLactate     float64
		MinLactate         float64
		MaxLactate         float64
		AverageYaw         float64
		RouteID            int64
		StrainScore        float64
	}{
		AverageTemp:        activity.AverageTemp,
		MinTemp:            activity.MinTemp,
		MaxTemp:            activity.MaxTemp,
		AverageWeatherTemp: activity.AverageWeatherTemp,
		MinWeatherTemp:     activity.MinWeatherTemp,
		MaxWeatherTemp:     activity.MaxWeatherTemp,
		AverageFeelsLike:   activity.AverageFeelsLike,
		AverageWindSpeed:   activity.AverageWindSpeed,
		AverageWindGust:    activity.AverageWindGust,
		PrevailingWindDeg:  activity.PrevailingWindDeg,
		HeadwindPercent:    activity.HeadwindPercent,
		TailwindPercent:    activity.TailwindPercent,
		AverageAltitude:    activity.AverageAltitude,
		MinAltitude:        activity.MinAltitude,
		MaxAltitude:        activity.MaxAltitude,
		AverageGradient:    activity.AverageGradient,
		AverageLactate:     activity.AverageLactate,
		MinLactate:         activity.MinLactate,
		MaxLactate:         activity.MaxLactate,
		AverageYaw:         activity.AverageYaw,
		RouteID:            activity.RouteID,
		StrainScore:        activity.StrainScore,
	}
	want := struct {
		AverageTemp        float64
		MinTemp            float64
		MaxTemp            float64
		AverageWeatherTemp float64
		MinWeatherTemp     float64
		MaxWeatherTemp     float64
		AverageFeelsLike   float64
		AverageWindSpeed   float64
		AverageWindGust    float64
		PrevailingWindDeg  int
		HeadwindPercent    float64
		TailwindPercent    float64
		AverageAltitude    float64
		MinAltitude        float64
		MaxAltitude        float64
		AverageGradient    float64
		AverageLactate     float64
		MinLactate         float64
		MaxLactate         float64
		AverageYaw         float64
		RouteID            int64
		StrainScore        float64
	}{31.4, 22.1, 35.2, 30.3, 21.5, 34.6, 33.8, 18.4, 32.7, 185, 42.5, 21.2, 514.8, 480.2, 731.9, 3.7, 2.4, 1.1, 5.8, 7.6, 12345, 68.9}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Activity environmental fields = %+v, want %+v", got, want)
	}
}
