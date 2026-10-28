package main

import (
	"fmt"
	"sort"
	"strings"

	icu "github.com/Thejuampi/icu"
)

func registerAthleteCommands(registry *CommandRegistry) {
	registry.Register("athlete", "show", listAllCommand[icu.Athlete]("athlete", "athlete show", "Get athlete profile with sport settings."))
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
		Usage: "athlete update [--weight KG] [--icu-weight KG] [--height M] [--name NAME] [--bio TEXT] [--dry-run]",
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

func runAthleteUpdate(_ []string, flags map[string]string, client *icu.Client) error {
	u := buildAthleteUpdate(flags)

	var current icu.Athlete
	if err := client.Get("athlete", nil, nil, &current); err != nil {
		return wrapCommandError(err)
	}
	_, before := athleteUpdateFields(&u, &current)
	result := athleteUpdateResult{Body: u, Before: before}
	if BoolFlag(flags, "dry-run") {
		result.DryRun = true
		result.Message = "dry-run: nothing written"

		return writeJSON(result)
	}

	if err := client.Put("athlete", nil, nil, u, nil); err != nil {
		return wrapCommandError(err)
	}
	result.Applied = true

	var after icu.Athlete
	if err := client.Get("athlete", nil, nil, &after); err != nil {
		return wrapCommandError(fmt.Errorf("write succeeded but read-back failed: %w", err))
	}
	result.ReadBack = &after
	want, got := athleteUpdateFields(&u, &after)
	var bad []string
	for k, v := range want {
		if got[k] != v {
			bad = append(bad, k)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		result.Message = "read-back mismatch: " + strings.Join(bad, ",")
		if err := writeJSON(result); err != nil {
			return err
		}

		return fmt.Errorf("athlete update: read-back mismatch on %s", strings.Join(bad, ","))
	}

	return writeJSON(result)
}

func athletePlanCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "athlete plan [update --plan-id ID --start-date DATE]",
		Description: "Get or update training plan.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			if _, ok := flags["plan-id"]; ok {
				var u icu.AthleteTrainingPlanUpdate
				u.ID = icu.ResolveAthleteID(flags)
				u.PlanID = IntFlag(flags, "plan-id", 0)
				u.StartDate = icu.StringFlag(flags, "start-date", "")

				var p icu.AthleteTrainingPlan
				if err := client.Put("training-plan", nil, nil, u, &p); err != nil {
					return wrapCommandError(err)
				}

				return writeJSON(p)
			}

			var p icu.AthleteTrainingPlan
			if err := client.Get("training-plan", nil, nil, &p); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(p)
		},
	}
}

func athleteSettingsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "athlete settings --device desktop|phone|tablet",
		Description: "Get athlete settings for device class.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			device := icu.StringFlag(flags, "device", "desktop")

			var s any
			if err := client.Get("settings", []string{device}, nil, &s); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(s)
		},
	}
}
