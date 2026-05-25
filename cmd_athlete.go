package main

func init() {
	RegisterCommand("athlete", "show", &Command{
		Usage: "athlete show",
		Description: "Get athlete profile with sportSettings.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var a Athlete
			if err := client.Get("athlete", nil, nil, &a); err != nil {
				return err
			}
			return WriteJSON(osStdout(), a)
		},
	})

	RegisterCommand("athlete", "update", &Command{
		Usage: "athlete update --weight 81 --icu-weight 81 --height 1.81",
		Description: "Update athlete profile fields.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			u := AthleteUpdate{}
			setIf := func(key string, set func()) {
				if _, ok := flags[key]; ok { set() }
			}
			setIf("weight", func() { v := floatFlagVal(flags, "weight", 0); u.Weight = &v })
			setIf("icu-weight", func() { v := floatFlagVal(flags, "icu-weight", 0); u.ICUWeight = &v })
			setIf("height", func() { v := floatFlagVal(flags, "height", 0); u.Height = &v })
			setIf("name", func() { v := StringFlag(flags, "name", ""); u.Name = &v })
			setIf("bio", func() { v := StringFlag(flags, "bio", ""); u.Bio = &v })
			var a Athlete
			if err := client.Put("athlete", nil, nil, u, &a); err != nil {
				return err
			}
			return WriteJSON(osStdout(), a)
		},
	})

	RegisterCommand("athlete", "profile", &Command{
		Usage: "athlete profile",
		Description: "Get athlete profile info.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var p AthleteProfile
			if err := client.Get("profile", nil, nil, &p); err != nil {
				return err
			}
			return WriteJSON(osStdout(), p)
		},
	})

	RegisterCommand("athlete", "summary", &Command{
		Usage: "athlete summary [--start DATE] [--end DATE]",
		Description: "Summary info for followed athletes.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			q := queryFromFlags(flags, "start", "end")
			var s []SummaryWithCats
			if err := client.Get("athlete-summary", nil, q, &s); err != nil {
				return err
			}
			return WriteJSON(osStdout(), s)
		},
	})

	RegisterCommand("athlete", "plan", &Command{
		Usage: "athlete plan [update --plan-id ID --start-date DATE]",
		Description: "Get or update training plan.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if _, ok := flags["plan-id"]; ok {
				u := AthleteTrainingPlanUpdate{
					ID:        ResolveAthleteID(flags),
					PlanID:    IntFlag(flags, "plan-id", 0),
					StartDate: StringFlag(flags, "start-date", ""),
				}
				var p AthleteTrainingPlan
				if err := client.Put("training-plan", nil, nil, u, &p); err != nil {
					return err
				}
				return WriteJSON(osStdout(), p)
			}
			var p AthleteTrainingPlan
			if err := client.Get("training-plan", nil, nil, &p); err != nil {
				return err
			}
			return WriteJSON(osStdout(), p)
		},
	})

	RegisterCommand("athlete", "settings", &Command{
		Usage: "athlete settings --device desktop|phone|tablet",
		Description: "Get athlete settings for device class.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			device := StringFlag(flags, "device", "desktop")
			var s any
			if err := client.Get("settings", []string{device}, nil, &s); err != nil {
				return err
			}
			return WriteJSON(osStdout(), s)
		},
	})
}
