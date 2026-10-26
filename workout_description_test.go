package icu_test

import (
	"errors"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestParseWorkoutDescriptionParsesSteadyStep(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 60m 70%")

	if err != nil || len(doc.Steps) != 1 || doc.Steps[0].Duration != 3600 || doc.Steps[0].Power.Value != 70 {
		t.Fatalf("ParseWorkoutDescription steady = %+v err=%v, want one 3600s 70%% step", doc, err)
	}
}

func TestParseWorkoutDescriptionParsesRampStep(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 10m 55-75%")

	if err != nil || len(doc.Steps) != 1 || !doc.Steps[0].Ramp || doc.Steps[0].Power.Start != 55 || doc.Steps[0].Power.End != 75 {
		t.Fatalf("ParseWorkoutDescription ramp = %+v err=%v, want one ramp 55-75%% step", doc, err)
	}
}

func TestParseWorkoutDescriptionParsesRepeatBlock(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("3x\n  - 5m 110%\n  - 5m 55%")

	if err != nil || len(icu.ExpandWorkoutSteps(doc)) != 6 {
		t.Fatalf("ParseWorkoutDescription repeat expanded steps = %d err=%v, want 6", len(icu.ExpandWorkoutSteps(doc)), err)
	}
}

func TestParseWorkoutDescriptionRejectsAmbiguousText(t *testing.T) {
	t.Parallel()

	_, err := icu.ParseWorkoutDescription("ride easy for a while")

	if !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
		t.Fatalf("ParseWorkoutDescription unsupported error = %v, want ErrWorkoutDescriptionUnsupported", err)
	}
}

func TestParseWorkoutDescriptionSkipsLeadingProse(t *testing.T) {
	t.Parallel()

	description := "Long Z2 durable 170-180W. HR ~130-140. Skin gate.\n\n- 15m 50-58%\n\n2x\n  - 6m 60-63%\n  - 2m 52-56%"

	doc, err := icu.ParseWorkoutDescription(description)

	if err != nil || len(icu.ExpandWorkoutSteps(doc)) != 5 {
		t.Fatalf("ParseWorkoutDescription prose = %+v err=%v, want 5 expanded steps", doc, err)
	}
}

func TestParseWorkoutDescriptionKeepsProseInDescription(t *testing.T) {
	t.Parallel()

	description := "Skin gate on long rides.\n- 60m 70%"

	doc, err := icu.ParseWorkoutDescription(description)

	if err != nil || doc.Description != description {
		t.Fatalf("ParseWorkoutDescription description = %+v err=%v, want the original text", doc, err)
	}
}

func TestParseWorkoutDescriptionParsesLeadingKeywordAndUnitSuffix(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 12m Ramp 50-58% FTP")

	if err != nil || len(doc.Steps) != 1 || !doc.Steps[0].Ramp || doc.Steps[0].Text != "Ramp" {
		t.Fatalf("ParseWorkoutDescription keyword = %+v err=%v, want one ramp step with text Ramp", doc, err)
	}
}

func TestParseWorkoutDescriptionClassifiesLeadingWarmupKeyword(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 15m Warmup 55-75% FTP")

	if err != nil || icu.ExpandWorkoutSteps(doc)[0].Kind != "warmup" {
		t.Fatalf("ParseWorkoutDescription leading warmup = %+v err=%v, want warmup kind", doc, err)
	}
}

func TestParseWorkoutDescriptionRoundTripsCalendarDescription(t *testing.T) {
	t.Parallel()

	description := "Long Z2 durable 170-180W. Skin gate.\n\n" +
		"- 15m Ramp 50-58% FTP\n\n" +
		"8x\n  - 3m 60-63% FTP\n  - 45s 50-55% FTP\n\n" +
		"- 12m Ramp 58-50% FTP"

	doc, err := icu.ParseWorkoutDescription(description)

	if err != nil || len(icu.ExpandWorkoutSteps(doc)) != 18 {
		t.Fatalf("ParseWorkoutDescription calendar text = %+v err=%v, want 18 expanded steps", doc, err)
	}
}

func TestParseWorkoutDescriptionStillRejectsMalformedStepLine(t *testing.T) {
	t.Parallel()

	_, err := icu.ParseWorkoutDescription("- 15q 88-92%\n- 60m 70%")

	if !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
		t.Fatalf("ParseWorkoutDescription malformed step error = %v, want ErrWorkoutDescriptionUnsupported", err)
	}
}

func TestParseWorkoutDescriptionPreservesStepText(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 10m 55-75% warmup\n- 5m 50% cooldown")

	if err != nil || icu.ExpandWorkoutSteps(doc)[0].Kind != "warmup" || icu.ExpandWorkoutSteps(doc)[1].Kind != "cooldown" {
		t.Fatalf("ParseWorkoutDescription kinds = %+v err=%v, want warmup/cooldown", icu.ExpandWorkoutSteps(doc), err)
	}
}

func TestParseWorkoutDescriptionParsesHourAndSecondDurations(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 1h 70%\n- 30s 50%")

	if err != nil || doc.Duration != 3630 {
		t.Fatalf("ParseWorkoutDescription duration = %+v err=%v, want 3630s", doc, err)
	}
}

func TestParseWorkoutDescriptionRejectsNestedRepeats(t *testing.T) {
	t.Parallel()

	_, err := icu.ParseWorkoutDescription("2x\n  3x\n    - 1m 100%")

	if !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
		t.Fatalf("ParseWorkoutDescription nested repeat error = %v, want ErrWorkoutDescriptionUnsupported", err)
	}
}

func TestParseWorkoutDescriptionRejectsEmptyRepeat(t *testing.T) {
	t.Parallel()

	_, err := icu.ParseWorkoutDescription("2x")

	if !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
		t.Fatalf("ParseWorkoutDescription empty repeat error = %v, want ErrWorkoutDescriptionUnsupported", err)
	}
}

func TestParseWorkoutDescriptionNativeRepeatSyntax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		desc         string
		wantDuration int
		wantTopSteps int
	}{
		{"native un-indented children closed by blank line", "- 15m 60%\n\n3x\n- 12m 80%\n- 6m 65%\n\n- 12m 60%", 15*60 + 3*18*60 + 12*60, 3},
		{"native repeat at end of description", "- 10m 55%\n\n2x\n- 5m 90%\n- 5m 50%", 10*60 + 2*10*60, 2},
		{"indented children still parse", "- 15m 60%\n3x\n  - 12m 80%\n  - 6m 65%\n- 12m 60%", 15*60 + 3*18*60 + 12*60, 3},
		{"indented children across blank lines", "3x\n  - 1m 100%\n\n  - 1m 50%\n\n- 5m 50%", 3*2*60 + 5*60, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := icu.ParseWorkoutDescription(tt.desc)
			if err != nil {
				t.Fatalf("ParseWorkoutDescription() error = %v", err)
			}
			if doc.Duration != tt.wantDuration || len(doc.Steps) != tt.wantTopSteps {
				t.Fatalf("duration=%d steps=%d, want duration=%d steps=%d", doc.Duration, len(doc.Steps), tt.wantDuration, tt.wantTopSteps)
			}
		})
	}
}

func TestParseWorkoutDescriptionAcceptsCompoundDurations(t *testing.T) {
	t.Parallel()

	doc, err := icu.ParseWorkoutDescription("- 2m30s 80%\n- 1h30m 60%\n- 1h0m5s 50%")
	if err != nil {
		t.Fatalf("ParseWorkoutDescription error = %v", err)
	}
	if len(doc.Steps) != 3 || doc.Steps[0].Duration != 150 || doc.Steps[1].Duration != 5400 ||
		doc.Steps[2].Duration != 3605 {
		t.Fatalf("steps = %+v, want 150s, 5400s, 3605s", doc.Steps)
	}

	for _, bad := range []string{"- 30s2m 80%", "- 2m2m 80%", "- 0m0s 80%"} {
		if _, err := icu.ParseWorkoutDescription(bad); !errors.Is(err, icu.ErrWorkoutDescriptionUnsupported) {
			t.Fatalf("ParseWorkoutDescription(%q) error = %v, want unsupported", bad, err)
		}
	}
}
