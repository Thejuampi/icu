package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestAnalyzeWarmup(t *testing.T) {
	t.Parallel()

	watts := make([]float64, 60)
	hr := make([]float64, 60)
	for i := range watts {
		watts[i] = 100 + float64(i)
		hr[i] = 100 + float64(i)*0.5
	}

	warmup := icu.ActivitySection{
		Name:       "warmup",
		StartIndex: 0,
		EndIndex:   60,
		Metadata:   map[string]any{},
	}

	got := icu.AnalyzeWarmup(watts, hr, warmup, 178)

	if got.DurationSeconds != 60 {
		t.Fatalf("DurationSeconds = %d, want 60", got.DurationSeconds)
	}
	if got.AvgPower <= 0 {
		t.Fatal("AvgPower should be positive")
	}
	if got.AvgHR <= 0 {
		t.Fatal("AvgHr should be positive")
	}
	if got.HRSlope <= 0 {
		t.Fatalf("HRSlope = %v, want positive (rising HR)", got.HRSlope)
	}
}

func TestAnalyzeWarmupNilSection(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 200, 300}
	hr := []float64{120, 130, 140}

	got := icu.AnalyzeWarmup(watts, hr, icu.ActivitySection{}, 178)

	if got.DurationSeconds != 0 {
		t.Fatalf("DurationSeconds = %d, want 0", got.DurationSeconds)
	}
}

func TestAnalyzeCooldown(t *testing.T) {
	t.Parallel()

	watts := make([]float64, 40)
	hr := make([]float64, 40)
	for i := range watts {
		watts[i] = 200 - float64(i)*3
		hr[i] = 150 - float64(i)
	}

	cooldown := icu.ActivitySection{
		Name:       "cooldown",
		StartIndex: 0,
		EndIndex:   40,
		Metadata:   map[string]any{},
	}

	got := icu.AnalyzeCooldown(watts, hr, cooldown, 178)

	if got.DurationSeconds != 40 {
		t.Fatalf("DurationSeconds = %d, want 40", got.DurationSeconds)
	}
	if got.HRRecovery1Min <= 0 {
		t.Fatalf("HRRecovery1Min = %v, want positive", got.HRRecovery1Min)
	}
}

func TestAnalyzeIntervals(t *testing.T) {
	t.Parallel()

	watts := []float64{100, 300, 300, 100, 300, 300, 100, 300, 300, 100}
	hr := []float64{120, 160, 165, 130, 162, 168, 125, 158, 163, 120}

	sections := []icu.ActivitySection{
		{Name: "WORK", StartIndex: 1, EndIndex: 3, Metadata: map[string]any{"type": "WORK"}},
		{Name: "RECOVERY", StartIndex: 3, EndIndex: 4, Metadata: map[string]any{"type": "RECOVERY"}},
		{Name: "WORK", StartIndex: 4, EndIndex: 6, Metadata: map[string]any{"type": "WORK"}},
		{Name: "RECOVERY", StartIndex: 6, EndIndex: 7, Metadata: map[string]any{"type": "RECOVERY"}},
		{Name: "WORK", StartIndex: 7, EndIndex: 9, Metadata: map[string]any{"type": "WORK"}},
		{Name: "RECOVERY", StartIndex: 9, EndIndex: 10, Metadata: map[string]any{"type": "RECOVERY"}},
	}

	got := icu.AnalyzeIntervals(sections, watts, hr, 285)

	if got.RepCount != 3 {
		t.Fatalf("RepCount = %d, want 3", got.RepCount)
	}
	if got.Repeatability.WorkPowerMean <= 0 {
		t.Fatal("WorkPowerMean should be positive")
	}
}

func TestAnalyzeZoneAlignment(t *testing.T) {
	t.Parallel()

	zoneTimes := []icu.ZoneTime{
		{ID: "Z1", Secs: 2000},
		{ID: "Z2", Secs: 1500},
		{ID: "Z3", Secs: 800},
		{ID: "Z4", Secs: 300},
	}
	hrZoneTimes := []int{2500, 1200, 600, 300, 0, 0, 0}

	got := icu.AnalyzeZoneAlignment(zoneTimes, hrZoneTimes)

	if got.PowerZ1Z2Pct <= 0 {
		t.Fatal("PowerZ1Z2Pct should be positive")
	}
}

func TestAnalyzeZoneAlignmentEmpty(t *testing.T) {
	t.Parallel()

	var zoneTimes []icu.ZoneTime
	var hrZoneTimes []int

	got := icu.AnalyzeZoneAlignment(zoneTimes, hrZoneTimes)

	if got.PowerZ1Z2Pct != 0 {
		t.Fatalf("PowerZ1Z2Pct = %v, want 0", got.PowerZ1Z2Pct)
	}
}

func TestAnalyzeActivityMicroNoStreams(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{
		ID:   "i123",
		Name: "Test Ride",
	}
	activity.MovingTime = 3600
	activity.TrainingLoad = 60
	activity.Intensity = 0.7

	var streams icu.StreamData
	var dto icu.IntervalsDTO

	got := icu.AnalyzeActivityMicro(&activity, streams, &dto, 285, 178)

	if got.SessionSummary.ID != "i123" {
		t.Fatalf("SessionSummary.ID = %s, want i123", got.SessionSummary.ID)
	}
	if got.Warmup != nil {
		t.Fatal("Warmup should be nil without streams")
	}
	if got.Cooldown != nil {
		t.Fatal("Cooldown should be nil without streams")
	}
}

func TestAnalyzeActivityMicroNilActivity(t *testing.T) {
	t.Parallel()

	got := icu.AnalyzeActivityMicro(nil, nil, nil, 285, 178)

	if got.ActivityID != "" {
		t.Fatalf("ActivityID = %s, want empty", got.ActivityID)
	}
}

func TestAnalyzeActivityMicroIntervalsWithoutStreams(t *testing.T) {
	t.Parallel()

	activity := icu.Activity{
		ID:         "i789",
		Name:       "Intervals Only",
		MovingTime: 3600,
	}
	activity.TrainingLoad = 70
	activity.Intensity = 0.8

	var streams icu.StreamData
	dto := icu.IntervalsDTO{
		Intervals: []icu.Interval{
			{Type: "WORK", StartIndex: 60, EndIndex: 120},
			{Type: "RECOVERY", StartIndex: 120, EndIndex: 150},
		},
	}

	got := icu.AnalyzeActivityMicro(&activity, streams, &dto, 285, 178)

	if got.Intervals != nil {
		t.Fatal("Intervals should be nil without stream data")
	}
}

func TestAnalyzeActivityMicroWithStreams(t *testing.T) {
	t.Parallel()

	var activity icu.Activity
	activity.ID = "i456"
	activity.Name = "Full Ride"
	activity.MovingTime = 3600
	activity.TrainingLoad = 80
	activity.Intensity = 0.75
	activity.ZoneTimes = []icu.ZoneTime{
		{ID: "Z1", Secs: 1000},
		{ID: "Z2", Secs: 1500},
		{ID: "Z3", Secs: 800},
		{ID: "Z4", Secs: 300},
	}
	activity.HRZoneTimes = []int{800, 1200, 900, 700, 0, 0, 0}

	streams := make(icu.StreamData)
	var watts, hr []float64
	for i := range 300 {
		switch {
		case i < 150:
			watts = append(watts, float64(150+i))
			if watts[i] > 250 {
				watts[i] = 250
			}
			hr = append(hr, 120+float64(i)*0.2)
		case i > 220:
			watts = append(watts, float64(250-(i-220)))
			if watts[i] < 50 {
				watts[i] = 50
			}
			hr = append(hr, float64(150-(i-220)))
		default:
			watts = append(watts, 250)
			hr = append(hr, 150)
		}
	}
	streams["watts"] = watts
	streams["heartrate"] = hr

	var dto icu.IntervalsDTO
	dto.Intervals = []icu.Interval{
		{Type: "WORK", StartIndex: 60, EndIndex: 120},
		{Type: "RECOVERY", StartIndex: 120, EndIndex: 150},
		{Type: "WORK", StartIndex: 150, EndIndex: 210},
	}

	got := icu.AnalyzeActivityMicro(&activity, streams, &dto, 285, 178)

	if got.Warmup == nil {
		t.Fatal("Warmup should not be nil with streams")
	}
	if got.Cooldown == nil {
		t.Fatal("Cooldown should not be nil with streams")
	}
	if got.Intervals == nil {
		t.Fatal("Intervals should not be nil")
	}
	if got.Intervals.RepCount != 2 {
		t.Fatalf("RepCount = %d, want 2", got.Intervals.RepCount)
	}
}

func TestAnalyzeCooldownShort(t *testing.T) {
	t.Parallel()

	watts := []float64{150, 120, 90, 60}
	hr := []float64{160, 150, 140, 130}
	section := icu.ActivitySection{
		Name:       "cooldown",
		StartIndex: 0,
		EndIndex:   4,
	}

	got := icu.AnalyzeCooldown(watts, hr, section, 178)

	if got.HRDecayRate <= 0 {
		t.Fatalf("HRDecayRate = %v, want positive", got.HRDecayRate)
	}
}
