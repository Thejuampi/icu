package main

import icu "github.com/Thejuampi/icu"

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

func athleteUpdateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "athlete update --weight 81 --icu-weight 81 --height 1.81",
		Description: "Update athlete profile fields.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
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

			var a icu.Athlete
			if err := client.Put("athlete", nil, nil, u, &a); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(a)
		},
	}
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
