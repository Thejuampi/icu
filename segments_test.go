package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestDetectSectionsByHRStabilization(t *testing.T) {
	t.Parallel()

	hr := make([]float64, 100)
	watts := make([]float64, 100)
	for i := range watts {
		watts[i] = 200
		switch {
		case i < 30:
			hr[i] = 100 + float64(i)*(40.0/30.0)
		case i > 80:
			hr[i] = 140 - float64(i-80)*(30.0/20.0)
		default:
			hr[i] = 140
		}
	}

	warmup, cooldown, _ := icu.DetectSectionsByHRStabilization(hr, watts)

	if warmup.EndIndex <= warmup.StartIndex {
		t.Error("warmup section not detected")
	}
	if cooldown.EndIndex <= cooldown.StartIndex {
		t.Error("cooldown section not detected")
	}
}

func TestDetectSectionsByHRStabilizationShortStreams(t *testing.T) {
	t.Parallel()

	hr := []float64{100, 101, 102}
	watts := []float64{200, 200, 200}

	main, _, _ := icu.DetectSectionsByHRStabilization(hr, watts)

	if main.EndIndex != len(hr) {
		t.Fatalf("main end = %d, want %d", main.EndIndex, len(hr))
	}
}

func TestDetectIntervalsFromDTO(t *testing.T) {
	t.Parallel()

	var dto icu.IntervalsDTO
	dto.Intervals = []icu.Interval{
		{Type: "WORK", StartIndex: 0, EndIndex: 10},
		{Type: "RECOVERY", StartIndex: 10, EndIndex: 20},
		{Type: "WORK", StartIndex: 20, EndIndex: 30},
		{Type: "RECOVERY", StartIndex: 30, EndIndex: 40},
	}

	sections := icu.DetectIntervalsFromDTO(&dto)

	if len(sections) != 4 {
		t.Fatalf("sections count = %d, want 4", len(sections))
	}
	if sections[0].Metadata["type"] != "WORK" {
		t.Fatalf("first section type = %v, want WORK", sections[0].Metadata["type"])
	}
}

func TestDetectIntervalsFromDTOEmpty(t *testing.T) {
	t.Parallel()

	var dto icu.IntervalsDTO
	sections := icu.DetectIntervalsFromDTO(&dto)

	if len(sections) != 0 {
		t.Fatalf("sections count = %d, want 0", len(sections))
	}
}

func TestDetectIntervalsFromDTONil(t *testing.T) {
	t.Parallel()

	sections := icu.DetectIntervalsFromDTO(nil)

	if len(sections) != 0 {
		t.Fatalf("sections count = %d, want 0", len(sections))
	}
}

func TestDetectIntervalsFromDTOInvalidBounds(t *testing.T) {
	t.Parallel()

	var dto icu.IntervalsDTO
	dto.Intervals = []icu.Interval{
		{Type: "WORK", StartIndex: -1, EndIndex: 10},
		{Type: "WORK", StartIndex: 10, EndIndex: 5},
		{Type: "WORK", StartIndex: 0, EndIndex: 20},
	}

	sections := icu.DetectIntervalsFromDTO(&dto)

	if len(sections) != 1 {
		t.Fatalf("sections count = %d, want 1", len(sections))
	}
}
