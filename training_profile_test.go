package icu_test

import (
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestValidateTrainingProfile(t *testing.T) {
	t.Parallel()

	valid := &icu.TrainingProfile{
		TargetFTP: 280, MaxRampPerWeek: 5, UsualDays: []string{"tue", "sat"}, UsualStartTimes: []string{"07:00"},
		Z2LowPct: 55, Z2HighPct: 75,
	}
	if err := icu.ValidateTrainingProfile(valid); err != nil {
		t.Fatalf("valid profile error = %v", err)
	}
	if err := icu.ValidateTrainingProfile(nil); err != nil {
		t.Fatalf("nil profile error = %v", err)
	}

	for name, bad := range map[string]*icu.TrainingProfile{
		"negative int":  {TargetFTP: -1},
		"negative ramp": {MaxRampPerWeek: -1},
		"half z2":       {Z2LowPct: 55},
		"z2 inverted":   {Z2LowPct: 80, Z2HighPct: 70},
		"bad day":       {UsualDays: []string{"monday"}},
		"bad time":      {UsualStartTimes: []string{"7am"}},
	} {
		if err := icu.ValidateTrainingProfile(bad); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestProfileZ2BandPrefersProfileThenZones(t *testing.T) {
	t.Parallel()

	if low, high, ok := icu.ProfileZ2Band(&icu.TrainingProfile{Z2LowPct: 56, Z2HighPct: 76},
		&icu.SportSettings{PowerZones: []int{55, 75}}); !ok || low != 56 || high != 76 {
		t.Fatalf("profile band = %v %v %v", low, high, ok)
	}
	if low, high, ok := icu.ProfileZ2Band(nil, &icu.SportSettings{PowerZones: []int{55, 75, 90}}); !ok || low != 55 || high != 75 {
		t.Fatalf("zones band = %v %v %v", low, high, ok)
	}
	if _, _, ok := icu.ProfileZ2Band(nil, nil); ok {
		t.Fatal("band without data")
	}
}

func TestValidatePlanAgainstProfile(t *testing.T) {
	t.Parallel()

	proposal := &icu.PlanProposalFile{
		Intent: icu.PlanIntent{FTP: 300},
		Sessions: []icu.PlanResolvedSession{
			// 2026-11-02 is a Monday.
			{
				Date: "2026-11-02", Name: "Endurance", StartTime: "18:30:00", Action: icu.CalendarActionCreate,
				Desc: "- 30m 65%\n\n2x\n- 10m 70%\n- 5m 95%",
			},
			{Date: "2026-11-03", Name: "Skipped", Action: icu.CalendarActionCancel},
			{Date: "2026-11-10", Name: "Tempo", StartTime: "07:00:00", Action: icu.CalendarActionCreate, Desc: "- 60m 85%"},
		},
	}
	profile := &icu.TrainingProfile{
		TargetFTP: 280, UsualDays: []string{"tue", "sat"}, UsualStartTimes: []string{"07:00"}, MinZ2MinutesPerWeek: 60,
	}
	settings := &icu.SportSettings{PowerZones: []int{55, 75, 90}}

	got := strings.Join(icu.ValidatePlanAgainstProfile(proposal, profile, settings), "\n")
	for _, want := range []string{
		"intent FTP 300 W is above target FTP 280 W",
		`2026-11-02 "Endurance" is on mon`,
		`2026-11-02 "Endurance" starts at 18:30`,
		"week 2026-11-02 has 50 planned Z2 minutes",
		"week 2026-11-09 has 0 planned Z2 minutes",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("warnings missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Skipped") || strings.Contains(got, `"Tempo" starts`) {
		t.Fatalf("unexpected warnings:\n%s", got)
	}

	noBand := icu.ValidatePlanAgainstProfile(proposal, &icu.TrainingProfile{MinZ2MinutesPerWeek: 60}, nil)
	if len(noBand) != 1 || !strings.Contains(noBand[0], "no Z2 band") {
		t.Fatalf("no band warnings = %v", noBand)
	}
	if icu.ValidatePlanAgainstProfile(proposal, nil, nil) != nil {
		t.Fatal("nil profile should not warn")
	}
}

func TestMergeProfileFTPFillsGapsOnly(t *testing.T) {
	t.Parallel()

	api := &icu.SportSettings{FTP: 266}
	merged, warnings := icu.MergeProfileFTP(api, &icu.TrainingProfile{OutdoorFTP: 270, IndoorFTP: 250})
	if merged.FTP != 266 || merged.IndoorFTP != 250 || api.IndoorFTP != 0 {
		t.Fatalf("merged = %+v, api = %+v", merged, api)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "indoor FTP 250 W taken from the local profile") ||
		!strings.Contains(joined, "outdoor FTP 266 W differs from profile 270 W") {
		t.Fatalf("warnings = %v", warnings)
	}

	if got, warnings := icu.MergeProfileFTP(nil, nil); got == nil || got.FTP != 0 || warnings != nil {
		t.Fatalf("nil merge = %+v %v", got, warnings)
	}
}

func TestPlanPricesIndoorSessionsAtProfileIndoorFTP(t *testing.T) {
	t.Parallel()

	settings, _ := icu.MergeProfileFTP(&icu.SportSettings{FTP: 266, Types: []string{"Ride", "VirtualRide"}},
		&icu.TrainingProfile{IndoorFTP: 250})
	indoor := true
	proposal := icu.BuildPlanProposal(&icu.PlanInput{
		NowDate:       "2026-11-01",
		SportSettings: settings,
		Warnings:      []string{"ftp: note"},
		Intent: icu.PlanIntent{
			Oldest: "2026-11-02", Newest: "2026-11-08",
			Constraints: icu.PlanConstraints{AllowCreate: true, DefaultType: "Ride", DefaultStartTime: "07:00"},
			Sessions: []icu.PlanSessionDraft{
				{Date: "2026-11-02", Name: "Trainer", Type: "Ride", Indoor: &indoor, Desc: "- 60m 70%"},
				{Date: "2026-11-03", Name: "Zwift", Type: "VirtualRide", Desc: "- 60m 70%"},
				{Date: "2026-11-04", Name: "Road", Type: "Ride", Desc: "- 60m 70%"},
			},
		},
	})

	applied := map[string]int{}
	for _, session := range proposal.Sessions {
		applied[session.Name] = session.FTPApplied
	}
	if applied["Trainer"] != 250 || applied["Zwift"] != 250 || applied["Road"] != 266 {
		t.Fatalf("ftpApplied = %v, want indoor 250 and outdoor 266 (%+v)", applied, proposal.Validation)
	}
	if !strings.Contains(strings.Join(proposal.Validation.Warnings, "\n"), "ftp: note") {
		t.Fatalf("warnings = %v", proposal.Validation.Warnings)
	}
}
