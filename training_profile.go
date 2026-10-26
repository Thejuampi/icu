package icu

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// TrainingProfile is the athlete's locally configured goals and caps. Every
// field is optional: a zero value means "not configured" and the matching
// check is skipped rather than replaced by a built-in number.
type TrainingProfile struct {
	TargetFTP  int `json:"targetFtp,omitempty"`
	OutdoorFTP int `json:"outdoorFtp,omitempty"`
	IndoorFTP  int `json:"indoorFtp,omitempty"`
	// MaxRampPerWeek caps the projected CTL rise per 7 days.
	MaxRampPerWeek float64 `json:"maxRampPerWeek,omitempty"`
	// MinZ2MinutesPerWeek is the weekly endurance-zone volume floor.
	MinZ2MinutesPerWeek int `json:"minZ2MinutesPerWeek,omitempty"`
	// Z2LowPct/Z2HighPct override the Z2 band (%FTP); when unset the band
	// comes from the sport settings' power zones.
	Z2LowPct  float64 `json:"z2LowPct,omitempty"`
	Z2HighPct float64 `json:"z2HighPct,omitempty"`
	// UsualDays are lower-case three-letter weekdays ("mon".."sun").
	UsualDays []string `json:"usualDays,omitempty"`
	// UsualStartTimes are "HH:MM" local start times.
	UsualStartTimes []string `json:"usualStartTimes,omitempty"`
	// MaxSimilarSessionsPerWeek is how many sessions with the same structure
	// a week may hold before the monotony check warns.
	MaxSimilarSessionsPerWeek int `json:"maxSimilarSessionsPerWeek,omitempty"`
	// CTLDays/ATLDays are the fitness/fatigue time constants of the load
	// projection; unset means the intervals.icu defaults.
	CTLDays int `json:"ctlDays,omitempty"`
	ATLDays int `json:"atlDays,omitempty"`
}

// Intervals.icu's fitness and fatigue time constants (days). These are the
// model's definition, not a physiological cut-off, and are overridable via
// TrainingProfile.CTLDays/ATLDays.
const (
	DefaultCTLDays = 42
	DefaultATLDays = 7
)

func profileWeekdays() []string {
	return []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
}

// ValidateTrainingProfile rejects values that cannot be meaningful.
func ValidateTrainingProfile(profile *TrainingProfile) error {
	if profile == nil {
		return nil
	}
	for _, value := range []int{
		profile.TargetFTP, profile.OutdoorFTP, profile.IndoorFTP, profile.MinZ2MinutesPerWeek,
		profile.MaxSimilarSessionsPerWeek, profile.CTLDays, profile.ATLDays,
	} {
		if value < 0 {
			return fmt.Errorf("profile values must not be negative (got %d)", value)
		}
	}
	if profile.MaxRampPerWeek < 0 {
		return fmt.Errorf("profile maxRampPerWeek must not be negative (got %v)", profile.MaxRampPerWeek)
	}
	if (profile.Z2LowPct > 0) != (profile.Z2HighPct > 0) || profile.Z2LowPct > profile.Z2HighPct {
		return errors.New("profile z2LowPct/z2HighPct must both be set with low <= high")
	}
	for _, day := range profile.UsualDays {
		if !slices.Contains(profileWeekdays(), day) {
			return fmt.Errorf("profile usual day %q must be one of %s", day, strings.Join(profileWeekdays(), ","))
		}
	}
	for _, clock := range profile.UsualStartTimes {
		if _, err := time.Parse("15:04", clock); err != nil {
			return fmt.Errorf("profile usual start time %q must be HH:MM", clock)
		}
	}

	return nil
}

// ProfileZ2Band returns the Z2 band in %FTP: the profile override first,
// otherwise intervals.icu power zones (upper bounds, Z2 = (zone1, zone2]).
func ProfileZ2Band(profile *TrainingProfile, settings *SportSettings) (float64, float64, bool) {
	if profile != nil && profile.Z2HighPct > 0 {
		return profile.Z2LowPct, profile.Z2HighPct, true
	}
	if settings != nil && len(settings.PowerZones) >= 2 {
		return float64(settings.PowerZones[0]), float64(settings.PowerZones[1]), true
	}

	return 0, 0, false
}

// ValidatePlanAgainstProfile returns warnings for planned (non-cancelled)
// sessions that fall outside the athlete's usual days/times, an intent FTP
// above the target FTP, and weeks short of the Z2 floor.
func ValidatePlanAgainstProfile(proposal *PlanProposalFile, profile *TrainingProfile, settings *SportSettings) []string {
	if proposal == nil || profile == nil {
		return nil
	}
	var warnings []string
	if profile.TargetFTP > 0 && proposal.Intent.FTP > profile.TargetFTP {
		warnings = append(warnings, fmt.Sprintf(
			"profile: intent FTP %d W is above target FTP %d W; sessions would be priced at a goal not yet reached",
			proposal.Intent.FTP, profile.TargetFTP,
		))
	}
	z2ByWeek := map[string]int{}
	var weeks []string
	for index := range proposal.Sessions {
		session := &proposal.Sessions[index]
		if session.Action == CalendarActionCancel {
			continue
		}
		warnings = append(warnings, profileScheduleWarnings(session, profile)...)
		week := planWeekStart(session.Date)
		if _, seen := z2ByWeek[week]; !seen {
			weeks = append(weeks, week)
		}
		z2ByWeek[week] += sessionZ2Seconds(session.Desc, profile, settings)
	}

	return append(warnings, profileZ2Warnings(weeks, z2ByWeek, profile, settings)...)
}

func profileScheduleWarnings(session *PlanResolvedSession, profile *TrainingProfile) []string {
	var warnings []string
	if len(profile.UsualDays) > 0 {
		if parsed, err := time.Parse(analysisDateLayout, session.Date); err == nil {
			day := profileWeekdays()[parsed.Weekday()]
			if !slices.Contains(profile.UsualDays, day) {
				warnings = append(warnings, fmt.Sprintf(
					"profile: %s %q is on %s, not a usual training day (%s)",
					session.Date, session.Name, day, strings.Join(profile.UsualDays, ","),
				))
			}
		}
	}
	if len(profile.UsualStartTimes) > 0 && len(session.StartTime) >= len("15:04") {
		clock := session.StartTime[:len("15:04")]
		if !slices.Contains(profile.UsualStartTimes, clock) {
			warnings = append(warnings, fmt.Sprintf(
				"profile: %s %q starts at %s, not a usual start time (%s)",
				session.Date, session.Name, clock, strings.Join(profile.UsualStartTimes, ","),
			))
		}
	}

	return warnings
}

func profileZ2Warnings(weeks []string, z2ByWeek map[string]int, profile *TrainingProfile, settings *SportSettings) []string {
	if profile.MinZ2MinutesPerWeek <= 0 || len(weeks) == 0 {
		return nil
	}
	low, high, ok := ProfileZ2Band(profile, settings)
	if !ok {
		return []string{"profile: minZ2MinutesPerWeek is set but no Z2 band is known " +
			"(set z2LowPct/z2HighPct or power zones in intervals.icu)"}
	}
	var warnings []string
	for _, week := range weeks {
		minutes := z2ByWeek[week] / secondsPerMinute
		if minutes < profile.MinZ2MinutesPerWeek {
			warnings = append(warnings, fmt.Sprintf(
				"profile: week %s has %d planned Z2 minutes (%.0f-%.0f%% FTP), below the %d-minute floor",
				week, minutes, low, high, profile.MinZ2MinutesPerWeek,
			))
		}
	}

	return warnings
}

// sessionZ2Seconds sums steady steps whose target sits inside the Z2 band
// (low exclusive, high inclusive, as intervals.icu zone bounds are upper
// limits). Ramps count when both ends are inside the band.
func sessionZ2Seconds(desc string, profile *TrainingProfile, settings *SportSettings) int {
	low, high, ok := ProfileZ2Band(profile, settings)
	if !ok || desc == "" {
		return 0
	}
	doc, err := ParseWorkoutDescription(desc)
	if err != nil {
		return 0
	}

	return z2StepSeconds(doc.Steps, low, high)
}

func z2StepSeconds(steps []WorkoutStep, low, high float64) int {
	inBand := func(value float64) bool { return value > low && value <= high }
	total := 0
	for index := range steps {
		step := &steps[index]
		if len(step.Steps) > 0 {
			total += max(step.Reps, 1) * z2StepSeconds(step.Steps, low, high)

			continue
		}
		if step.Power == nil {
			continue
		}
		if step.Ramp {
			if inBand(step.Power.Start) && inBand(step.Power.End) {
				total += step.Duration
			}

			continue
		}
		if inBand(step.Power.Value) {
			total += step.Duration
		}
	}

	return total
}
