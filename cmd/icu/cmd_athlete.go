package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	icu "github.com/Thejuampi/icu"
)

func registerAthleteCommands(registry *CommandRegistry) {
	registry.Register("athlete", "show", athleteShowCommand())
	registry.Register("athlete", "update", athleteUpdateCommand())
	registry.Register("athlete", "profile", listAllCommand[icu.AthleteProfile]("profile", "athlete profile", "Get athlete profile info."))
	registry.Register(
		"athlete", "summary",
		listQueryCommand[[]icu.SummaryWithCats](
			"athlete-summary",
			"athlete summary [--start DATE] [--end DATE]",
			"Summary info for followed athletes.",
			queryBuilder("start", "end"),
		),
	)
	registry.Register("athlete", "plan", athletePlanCommand())
	registry.Register("athlete", "settings", athleteSettingsCommand())
}

type athleteUpdateResult struct {
	Body     icu.AthleteUpdate `json:"body"`
	Before   map[string]any    `json:"before"`
	DryRun   bool              `json:"dryRun"`
	Applied  bool              `json:"applied"`
	Message  string            `json:"message,omitempty"`
	ReadBack *icu.Athlete      `json:"readBack,omitempty"`
}

func athleteUpdateCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "athlete update [--weight KG] [--icu-weight KG] [--height M] [--name NAME] [--bio TEXT] [--dry-run] [--include-private]",
		Description: "Update athlete profile fields. Shows current values and a read-back; --dry-run only previews. " +
			"A read-back mismatch exits non-zero.",
		Run: runAthleteUpdate,
	}
}

func buildAthleteUpdate(flags map[string]string) icu.AthleteUpdate {
	var u icu.AthleteUpdate
	setIf := func(key string, set func()) {
		if _, ok := flags[key]; ok {
			set()
		}
	}

	setIf("weight", func() { v := floatFlagVal(flags, "weight", 0); u.Weight = &v })
	setIf("icu-weight", func() { v := floatFlagVal(flags, "icu-weight", 0); u.ICUWeight = &v })
	setIf("height", func() { v := floatFlagVal(flags, "height", 0); u.Height = &v })
	setIf("name", func() { v := icu.StringFlag(flags, "name", ""); u.Name = &v })
	setIf("bio", func() { v := icu.StringFlag(flags, "bio", ""); u.Bio = &v })

	return u
}

// athleteUpdateFields returns, for each field set in u, the wire name with the
// wanted value and the value currently on a.
func athleteUpdateFields(u *icu.AthleteUpdate, a *icu.Athlete) (map[string]any, map[string]any) {
	want, have := map[string]any{}, map[string]any{}
	if u.Weight != nil {
		want["weight"], have["weight"] = *u.Weight, a.Weight
	}
	if u.ICUWeight != nil {
		want["icu_weight"], have["icu_weight"] = *u.ICUWeight, a.ICUWeight
	}
	if u.Height != nil {
		want["height"], have["height"] = *u.Height, a.Height
	}
	if u.Name != nil {
		want["name"], have["name"] = *u.Name, a.Name
	}
	if u.Bio != nil {
		want["bio"], have["bio"] = *u.Bio, a.Bio
	}

	return want, have
}

func runAthleteUpdate(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
	u := buildAthleteUpdate(flags)
	if want, _ := athleteUpdateFields(&u, &icu.Athlete{}); len(want) == 0 {
		return fmt.Errorf("%w: athlete update needs at least one of --weight, --icu-weight, --height, --name, --bio", errInvalidValue)
	}

	var current icu.Athlete
	if err := client.Get("athlete", nil, nil, &current); err != nil {
		return wrapCommandError(err)
	}
	_, before := athleteUpdateFields(&u, &current)
	result := athleteUpdateResult{Body: u, Before: before}
	if BoolFlag(flags, "dry-run") {
		result.DryRun = true
		result.Message = dryRunNothingWritten

		return writeJSON(stdout, result)
	}

	if err := client.Put("athlete", nil, nil, u, nil); err != nil {
		return wrapCommandError(err)
	}
	result.Applied = true

	var after icu.Athlete
	if err := client.Get("athlete", nil, nil, &after); err != nil {
		return wrapCommandError(fmt.Errorf("write succeeded but read-back failed: %w", err))
	}
	want, got := athleteUpdateFields(&u, &after)
	redactAthletePrivate(&after, flags)
	result.ReadBack = &after
	var bad []string
	for k, v := range want {
		if got[k] != v {
			bad = append(bad, k)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		result.Message = "read-back mismatch: " + strings.Join(bad, ",")
		if err := writeJSON(stdout, result); err != nil {
			return err
		}

		return fmt.Errorf("athlete update: read-back mismatch on %s", strings.Join(bad, ","))
	}

	return writeJSON(stdout, result)
}

func athletePlanCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "athlete plan [update --plan-id ID --start-date DATE]",
		Description: "Get or update training plan.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			if _, ok := flags["plan-id"]; ok {
				var u icu.AthleteTrainingPlanUpdate
				u.ID = icu.ResolveAthleteID(flags)
				u.PlanID = IntFlag(flags, "plan-id", 0)
				u.StartDate = icu.StringFlag(flags, "start-date", "")

				var p icu.AthleteTrainingPlan
				if err := client.Put("training-plan", nil, nil, u, &p); err != nil {
					return wrapCommandError(err)
				}

				return writeJSON(stdout, p)
			}

			var p icu.AthleteTrainingPlan
			if err := client.Get("training-plan", nil, nil, &p); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(stdout, p)
		},
	}
}

func athleteSettingsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "athlete settings --device desktop|phone|tablet",
		Description: "Get athlete settings for device class.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			device := icu.StringFlag(flags, "device", "desktop")

			var s any
			if err := client.Get("settings", []string{device}, nil, &s); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(stdout, s)
		},
	}
}

func athleteShowCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "athlete show [--include-private]",
		Description: "Get athlete profile with sport settings. Date of birth and Strava id are " +
			"omitted unless --include-private is set.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			var athlete icu.Athlete
			if err := client.Get("athlete", nil, nil, &athlete); err != nil {
				return wrapCommandError(err)
			}
			redactAthletePrivate(&athlete, flags)

			return writeJSON(stdout, athlete)
		},
	}
}

// redactAthletePrivate clears the API key always, and date of birth and
// Strava id unless --include-private is set. athlete show, the athlete update
// read-back and analysis coaching (via redactAthleteForCoaching) use it.
func redactAthletePrivate(athlete *icu.Athlete, flags map[string]string) {
	if athlete == nil {
		return
	}
	athlete.APIKey = ""
	if BoolFlag(flags, "include-private") {
		return
	}
	athlete.DateOfBirth = ""
	athlete.StravaID = 0
}

// redactAthleteForCoaching also drops identity and contact fields (name,
// email, bio, city/state/country) that a coaching context has no use for,
// unless --include-private is set. Timezone, sex and physiology stay.
func redactAthleteForCoaching(athlete *icu.Athlete, flags map[string]string) {
	redactAthletePrivate(athlete, flags)
	if athlete == nil || BoolFlag(flags, "include-private") {
		return
	}
	athlete.Name, athlete.FirstName, athlete.LastName = "", "", ""
	athlete.Email, athlete.Bio = "", ""
	athlete.City, athlete.State, athlete.Country = "", "", ""
}
