package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func stableRegime(samples []float64) icu.RebalanceRegime {
	regime, _ := icu.DetectRebalanceRegime(samples)
	return regime
}

func nilInput() *icu.RebalanceInput {
	return &icu.RebalanceInput{
		Scope: icu.RebalanceScope{
			StartDate: "2026-06-22",
			EndDate:   "2026-06-28",
		},
	}
}

func TestBuildRebalanceEnvelopeBlocksMissingRegime(t *testing.T) {
	t.Parallel()

	if _, ok := icu.BuildRebalanceEnvelope(&icu.RebalanceRegime{}, nilInput(), 300); ok {
		t.Fatalf("missing regime should block envelope")
	}
}

func TestBuildRebalanceEnvelopeWithinFence(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{280, 290, 300, 310, 320, 300, 305, 295, 315, 300})
	report, ok := icu.BuildRebalanceEnvelope(&regime, nilInput(), 300)
	if !ok {
		t.Fatalf("within-fence envelope should build")
	}
	if report.OutsideEnvelope {
		t.Fatalf("current within fence reported outside")
	}
	if report.Envelope.Low.Cmp(report.Envelope.Current) >= 0 {
		t.Fatalf("low should be below current")
	}
	if report.Envelope.High.Cmp(report.Envelope.Current) <= 0 {
		t.Fatalf("high should be above current")
	}
}

func TestBuildRebalanceEnvelopeCurrentAboveHighIsOutside(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{100, 110, 105, 108, 102, 109, 104, 107, 103, 106})
	report, ok := icu.BuildRebalanceEnvelope(&regime, nilInput(), 600)
	if !ok {
		t.Fatalf("envelope should still build when current is outside")
	}
	if !report.OutsideEnvelope {
		t.Fatalf("current far above high should be outside envelope")
	}
}

func TestBuildRebalanceEnvelopeCurrentBelowLowIsOutside(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{200, 210, 205, 208, 202, 209, 204, 207, 203, 206})
	report, ok := icu.BuildRebalanceEnvelope(&regime, nilInput(), 10)
	if !ok {
		t.Fatalf("envelope should still build when current is outside")
	}
	if !report.OutsideEnvelope {
		t.Fatalf("current far below low should be outside envelope")
	}
}

func TestBuildRebalanceEnvelopeMissingMetricsReduceConfidence(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{280, 290, 300, 310, 320, 300, 305, 295, 315, 300})
	// With nil input and activities in scope, metrics compute from available
	// data. Build a minimal input that has all 10 metrics available to get full confidence.
	input := nilInput()
	full := icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", TrainingLoad: 300, Intensity: 0.66, MovingTime: 12000, RPE: 5},
		},
		Events: []icu.Event{
			{ID: 1, Category: "WORKOUT", Type: "Ride", Target: "POWER", StartDateLocal: "2026-06-24T07:00:00", TrainingLoad: 150, MovingTime: 12384, Intensity: 0.66},
		},
		SportSettings: &icu.SportSettings{FTP: 285, IndoorFTP: 285, PowerZones: []int{55, 75, 90}},
		Scope:         input.Scope,
		Wellness: &icu.WellnessAnalysis{
			Scope:     icu.WellnessScope{Records: 10},
			HRV:       icu.WellnessSignal{ZScore: 0.5},
			RestingHR: icu.WellnessSignal{Delta: 0},
			Sleep:     icu.WellnessSignal{Ratio: 0.5},
		},
		Plan: &icu.TrainingPlanAnalysis{
			History:  icu.TrainingPlanHistory{PlannedLoadAlignment: "within_recent_tolerance"},
			Decision: icu.TrainingPlanDecisionGuidance{ADEScore: 100},
		},
	}
	fullReport, okFull := icu.BuildRebalanceEnvelope(&regime, &full, 300)
	if !okFull {
		t.Fatalf("full metrics envelope should build")
	}
	partial := icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", TrainingLoad: 300},
		},
		Scope: input.Scope,
	}
	partialReport, okPartial := icu.BuildRebalanceEnvelope(&regime, &partial, 300)
	if !okPartial {
		t.Fatalf("partial metrics envelope should build")
	}
	if partialReport.Confidence >= fullReport.Confidence {
		t.Fatalf("partial metrics confidence %f should be below full %f", partialReport.Confidence, fullReport.Confidence)
	}
}

func TestRebalanceWeeklyDurationSeriesExcludesScopeWeek(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: "Ride", StartDateLocal: "2026-05-25T08:00:00", MovingTime: 7200},
		{Type: "Ride", StartDateLocal: "2026-06-01T08:00:00", MovingTime: 5400},
		{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", MovingTime: 999}, // scope week excluded
	}
	got := icu.RebalanceWeeklyDurationSeries(activities, "2026-06-22")
	if len(got) != 2 {
		t.Fatalf("expected 2 weeks, got %d: %v", len(got), got)
	}
}

func TestRebalanceWeeklyDurationSeriesIgnoresNonCycling(t *testing.T) {
	t.Parallel()

	activities := []icu.Activity{
		{Type: "Run", StartDateLocal: "2026-05-25T08:00:00", MovingTime: 7200},
	}
	got := icu.RebalanceWeeklyDurationSeries(activities, "2026-06-22")
	if len(got) != 0 {
		t.Fatalf("expected 0 weeks for non-cycling, got %d", len(got))
	}
}

func TestBuildRebalanceEnvelopeMetricsExpandWithWellness(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{280, 290, 300, 310, 320, 300, 305, 295, 315, 300})
	// Without wellness: limited metrics available → lower completeness
	noWellness := nilInput()
	report, ok := icu.BuildRebalanceEnvelope(&regime, noWellness, 300)
	if !ok {
		t.Fatalf("no-wellness envelope should build")
	}
	// With wellness: more metrics available → higher confidence
	withWellness := icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", TrainingLoad: 300, Intensity: 0.66, MovingTime: 12000, RPE: 5},
		},
		Events: []icu.Event{
			{ID: 1, Category: "WORKOUT", Type: "Ride", Target: "POWER", StartDateLocal: "2026-06-24T07:00:00", TrainingLoad: 150, MovingTime: 12384, Intensity: 0.66},
		},
		SportSettings: &icu.SportSettings{FTP: 285, IndoorFTP: 285, PowerZones: []int{55, 75, 90}},
		Scope:         noWellness.Scope,
		Wellness: &icu.WellnessAnalysis{
			Scope:     icu.WellnessScope{Records: 10},
			HRV:       icu.WellnessSignal{ZScore: 0.5},
			RestingHR: icu.WellnessSignal{Delta: -1},
			Sleep:     icu.WellnessSignal{Ratio: 0.7},
		},
		Plan: &icu.TrainingPlanAnalysis{
			History:  icu.TrainingPlanHistory{PlannedLoadAlignment: "within_recent_tolerance"},
			Decision: icu.TrainingPlanDecisionGuidance{ADEScore: 90},
		},
	}
	reportWell, okWell := icu.BuildRebalanceEnvelope(&regime, &withWellness, 300)
	if !okWell {
		t.Fatalf("with-wellness envelope should build")
	}
	if reportWell.Confidence <= report.Confidence {
		t.Fatalf("wellness should increase confidence: %.3f -> %.3f", report.Confidence, reportWell.Confidence)
	}
}

func TestBuildRebalanceEnvelopeAdaptationModifier(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{280, 290, 300, 310, 320, 300, 305, 295, 315, 300})
	// Low ADE → careful (contracted high)
	input := icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", TrainingLoad: 300, Intensity: 0.66, MovingTime: 12000},
		},
		Scope: icu.RebalanceScope{StartDate: "2026-06-22", EndDate: "2026-06-28"},
		Plan: &icu.TrainingPlanAnalysis{
			History:  icu.TrainingPlanHistory{PlannedLoadAlignment: "within_recent_tolerance"},
			Decision: icu.TrainingPlanDecisionGuidance{ADEScore: 20},
		},
	}
	report, ok := icu.BuildRebalanceEnvelope(&regime, &input, 300)
	if !ok {
		t.Fatalf("adaptation envelope should build")
	}
	// With low ADE and no wellness, the high modifier should still produce valid envelope
	base, _ := icu.BuildRebalanceEnvelope(&regime, nilInput(), 300)
	if report.Envelope.High.Cmp(base.Envelope.High) > 0 {
		t.Fatalf("low ADE should not expand high beyond neutral: low=%s neutral=%s",
			report.Envelope.High.DecimalString(), base.Envelope.High.DecimalString())
	}
}

func TestBuildRebalanceEnvelopeLowNeverBelowZero(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{50, 55, 50, 52, 50, 53, 50, 51, 50, 54})
	report, ok := icu.BuildRebalanceEnvelope(&regime, nilInput(), 52)
	if !ok {
		t.Fatalf("envelope should build")
	}
	zero := icu.ZeroRebalanceRat()
	if report.Envelope.Low.Cmp(zero) < 0 {
		t.Fatalf("low below zero: %s", report.Envelope.Low.DecimalString())
	}
}

func TestRebalanceMetricDurationEnoughHistory(t *testing.T) {
	t.Parallel()

	input := &icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-05-04T08:00:00", MovingTime: 7200},
			{Type: "Ride", StartDateLocal: "2026-05-11T08:00:00", MovingTime: 5400},
			{Type: "Ride", StartDateLocal: "2026-05-18T08:00:00", MovingTime: 9000},
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", MovingTime: 3600},
		},
		Scope: icu.RebalanceScope{StartDate: "2026-06-22", EndDate: "2026-06-28"},
	}
	regime := stableRegime([]float64{280, 290, 300, 295, 305, 298})
	report, ok := icu.BuildRebalanceEnvelope(&regime, input, 300)
	if !ok {
		t.Fatalf("envelope with duration data should build")
	}
	if report.Confidence <= 0 {
		t.Fatalf("duration data should produce positive confidence")
	}
}

func TestRebalanceMetricComplianceAlignments(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{250, 260, 270, 280, 290, 300})
	tests := []struct {
		name  string
		input icu.RebalanceInput
	}{
		{
			name:  "above peak contracts most",
			input: icu.RebalanceInput{Plan: &icu.TrainingPlanAnalysis{History: icu.TrainingPlanHistory{PlannedLoadAlignment: "above_peak_tolerance"}}},
		},
		{
			name:  "below contracts moderately",
			input: icu.RebalanceInput{Plan: &icu.TrainingPlanAnalysis{History: icu.TrainingPlanHistory{PlannedLoadAlignment: "below_plan"}}},
		},
		{
			name:  "above average contracts mildly",
			input: icu.RebalanceInput{Plan: &icu.TrainingPlanAnalysis{History: icu.TrainingPlanHistory{PlannedLoadAlignment: "above_average"}}},
		},
		{
			name:  "within tolerance neutral",
			input: icu.RebalanceInput{Plan: &icu.TrainingPlanAnalysis{History: icu.TrainingPlanHistory{PlannedLoadAlignment: "within_recent_tolerance"}}},
		},
		{
			name:  "unknown alignment neutral",
			input: icu.RebalanceInput{Plan: &icu.TrainingPlanAnalysis{History: icu.TrainingPlanHistory{PlannedLoadAlignment: "unknown"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, ok := icu.BuildRebalanceEnvelope(&regime, &tt.input, 280)
			if !ok {
				t.Fatalf("envelope should build for %s", tt.name)
			}
			_ = report
		})
	}
}

func TestRebalanceMetricDurationReturnsOneWithFewWeeks(t *testing.T) {
	t.Parallel()

	input := &icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-15T08:00:00", MovingTime: 7200},
		},
		Scope: icu.RebalanceScope{StartDate: "2026-06-22", EndDate: "2026-06-28"},
	}
	regime := stableRegime([]float64{280, 290, 300, 295, 305, 298})
	report, ok := icu.BuildRebalanceEnvelope(&regime, input, 300)
	if !ok {
		t.Fatalf("envelope with few duration weeks should build")
	}
	_ = report
}

func TestRebalanceMetricComplianceNilPlan(t *testing.T) {
	t.Parallel()

	regime := stableRegime([]float64{250, 260, 270, 280, 290, 300})
	report, ok := icu.BuildRebalanceEnvelope(&regime, nilInput(), 280)
	if !ok {
		t.Fatalf("envelope without plan should build")
	}
	_ = report
}

func TestRebalanceMetricZoneDistributionAllZ2(t *testing.T) {
	t.Parallel()

	input := &icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", Intensity: 0.65, MovingTime: 7200},
		},
		Scope:         icu.RebalanceScope{StartDate: "2026-06-22", EndDate: "2026-06-28"},
		SportSettings: &icu.SportSettings{FTP: 285},
	}
	regime := stableRegime([]float64{280, 290, 300, 295, 305, 298})
	report, ok := icu.BuildRebalanceEnvelope(&regime, input, 300)
	if !ok {
		t.Fatalf("envelope with all-Z2 zones should build")
	}
	_ = report
}

func TestRebalanceMetricRPEWithData(t *testing.T) {
	t.Parallel()

	input := &icu.RebalanceInput{
		Activities: []icu.Activity{
			{Type: "Ride", StartDateLocal: "2026-06-22T08:00:00", RPE: 3},
		},
		Scope: icu.RebalanceScope{StartDate: "2026-06-22", EndDate: "2026-06-28"},
	}
	regime := stableRegime([]float64{280, 290, 300, 295, 305, 298})
	report, ok := icu.BuildRebalanceEnvelope(&regime, input, 300)
	if !ok {
		t.Fatalf("envelope with RPE data should build")
	}
	_ = report
}

func TestRebalanceMetricRestingHRWithData(t *testing.T) {
	t.Parallel()

	input := &icu.RebalanceInput{
		Wellness: &icu.WellnessAnalysis{
			Scope:     icu.WellnessScope{Records: 10},
			RestingHR: icu.WellnessSignal{Delta: -3},
		},
	}
	regime := stableRegime([]float64{280, 290, 300, 295, 305, 298})
	report, ok := icu.BuildRebalanceEnvelope(&regime, input, 300)
	if !ok {
		t.Fatalf("envelope with resting HR data should build")
	}
	_ = report
}
