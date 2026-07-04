package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	zeppDefaultDateRangeDays = 7
	zeppLoginTimeout         = 5 * time.Minute
)

func registerZeppCommands(registry *CommandRegistry) {
	registry.Register("zepp", "token", zeppTokenCommand())
	registry.Register("zepp", "login", zeppLoginCommand())
	registry.Register("zepp", "logout", zeppLogoutCommand())
	registry.Register("zepp", "status", zeppStatusCommand())
	registry.Register("zepp", "profile", zeppProfileCommand())
	registry.Register("zepp", "summary", zeppSimpleCommand("summary", "Daily band summary (steps, sleep, HR, SpO2, weight, stress).", (*icu.ZeppClient).BandData))
	registry.Register("zepp", "sleep", zeppSimpleCommand("sleep", "Sleep records with stages, score, and average HR.", (*icu.ZeppClient).SleepDays))
	registry.Register("zepp", "heart-rate", zeppHeartRateCommand())
	registry.Register("zepp", "spo2", zeppSimpleCommand("spo2", "Blood oxygen measurements for the date range.", (*icu.ZeppClient).SpO2Readings))
	registry.Register("zepp", "stress", zeppSimpleCommand("stress", "Stress scores for the date range.", (*icu.ZeppClient).StressDays))
	registry.Register("zepp", "pai", zeppSimpleCommand("pai", "Personal Activity Intelligence (PAI) daily scores and zone breakdown.", (*icu.ZeppClient).PAIDays))
	registry.Register("zepp", "events", zeppEventsCommand())
	registry.Register("zepp", "hrv", zeppHRVCommand())
	registry.Register("zepp", "weight", zeppSimpleCommand("weight", "Weight measurements from /users/{id}/members/-1/weightRecords.", (*icu.ZeppClient).WeightRecords))
	registry.Register("zepp", "manual-data", zeppSimpleCommand("manual-data", "Manually entered wellness records from /v1/user/manualData.json.", (*icu.ZeppClient).ManualData))
	registry.Register(
		"zepp", "second-heart-rate",
		zeppSimpleCommand("second-heart-rate", "Per-second heart-rate COS file index from /users/me/fileInfo/events. Blobs are not downloaded.", (*icu.ZeppClient).SecondHeartRateFiles),
	)
	registry.Register("zepp", "spo2-windows", zeppSingleDateCommand("spo2-windows", "SpO2 ODI windows for a single day from /users/{id}/events/dateString.", (*icu.ZeppClient).SpO2Windows))

	registerV2EventCommand(registry, "readiness", mustV2EventPreset("readiness"), icu.DecodeReadiness, "Daily readiness scores from /v2/users/me/events.")
	registerV2EventCommand(registry, "body-battery", mustV2EventPreset("body-battery"), icu.DecodeBodyBattery, "Body-battery / Charge levels from /v2/users/me/events.")
	registerV2EventCommand(registry, "hybridcharge", mustV2EventPreset("hybridcharge"), icu.DecodeHybridCharge, "HybridCharge energy scores from /v2/users/me/events.")
	registerV2EventCommand(registry, "biocharge", mustV2EventPreset("biocharge"), icu.DecodeHybridCharge, "Alias for hybridcharge. Reads legacy BioCharge-compatible energy scores from /v2/users/me/events.")
	registerV2EventCommand(registry, "health-summary", mustV2EventPreset("daily-health"), icu.DecodeHealthSummary, "Daily health summaries from /v2/users/me/events.")
	registerV2EventCommand(registry, "mood", mustV2EventPreset("emotion"), icu.DecodeMood, "Mood / emotion readings from /v2/users/me/events.")
	registerV2EventCommand(registry, "skin-temp", mustV2EventPreset("skin-temp"), icu.DecodeSkinTemp, "Skin temperature delta readings from /v2/users/me/events.")
	registerV2EventCommand(registry, "stress-minute", mustV2EventPreset("stress-minute"), icu.DecodeStressMinute, "Per-minute stress readings from /v2/users/me/events.")
	registerV2EventCommand(registry, "respiratory-rate", mustV2EventPreset("respiratory"), icu.DecodeRespiratoryRate, "Overnight respiratory rate readings from /v2/users/me/events.")
	registry.Register("zepp", "blood-pressure", zeppBloodPressureCommand())

	registerWatchStatCommand(registry, "sport-load", (*icu.ZeppClient).SportLoad, "Daily training load from the watch sport statistics.")
	registerWatchStatCommand(registry, "vo2", (*icu.ZeppClient).VO2Max, "VO2 max estimates from the watch sport statistics.")
	registry.Register("zepp", "workouts", zeppWorkoutsCommand())
	registry.Register("zepp", "workout", zeppWorkoutCommand())
}

func mustV2EventPreset(name string) icu.V2EventPreset {
	preset, ok := icu.V2EventPresetByName(name)
	if !ok {
		panic("unknown preset: " + name)
	}

	return preset
}

func zeppAuthCommand(usage, description string, run func(email, password string) error) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			email := icu.StringFlag(flags, "email", "")
			if email == "" {
				return errors.New("email is required: use --email flag")
			}

			password, err := promptPasswordFlagOrInteractive(flags["password"])
			if err != nil {
				return err
			}

			return run(email, password)
		},
	}
}

func zeppTokenCommand() *Command {
	return zeppAuthCommand(
		"zepp token --email EMAIL [--password PASSWORD]",
		"[debug] Get Zepp API tokens. WARNING: outputs tokens in plaintext. Use 'zepp login' for normal auth. Use this only when you need to inspect or manually manage tokens.", //nolint:lll // security warning
		runZeppToken,
	)
}

func zeppLoginCommand() *Command {
	return zeppAuthCommand(
		"zepp login --email EMAIL [--password PASSWORD]",
		"Authenticate with Zepp using email and password to obtain login tokens.",
		runZeppLogin,
	)
}

func zeppLogoutCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp logout",
		Description: "Remove the stored Zepp login token from the local config.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			return runZeppLogout()
		},
	}
}

func zeppStatusCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp status",
		Description: "Show whether a Zepp login token is configured (no raw value is printed).",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			return runZeppStatus(flags)
		},
	}
}

func zeppProfileCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp profile",
		Description: "Fetch the Zepp user profile for the authenticated account.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client) error {
			return runZeppProfile()
		},
	}
}

func zeppSimpleCommand[T any](name, description string, fetch func(*icu.ZeppClient, context.Context, string, string) (T, error)) *Command {
	return &Command{
		Name:        "",
		Usage:       fmt.Sprintf("zepp %s --oldest YYYY-MM-DD --newest YYYY-MM-DD", name),
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := fetch(client, ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppHeartRateCommand() *Command {
	return zeppSwitchCommand(
		"zepp heart-rate --source band|app --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		"Heart rate time series for the date range from band summary or the user heart-rate endpoint.",
		"source", "band", "band", "app",
		(*icu.ZeppClient).HeartRateSeries,
		(*icu.ZeppClient).HeartRateEndpoint,
	)
}

func zeppHRVCommand() *Command {
	return zeppSwitchCommand(
		"zepp hrv --metric sdnn|rmssd --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		"Nightly HRV values from /v2/users/me/events.",
		"metric", "sdnn", "sdnn", "rmssd",
		(*icu.ZeppClient).HRVSDNNDays,
		(*icu.ZeppClient).HRVRMSSDDays,
	)
}

func zeppEventsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp events --preset NAME --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Fetch V2 wellness events by preset.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			presetName := icu.StringFlag(flags, "preset", "")
			if presetName == "" {
				return errors.New("preset is required: use --preset flag")
			}

			preset, ok := icu.V2EventPresetByName(presetName)
			if !ok {
				return fmt.Errorf("unknown preset %q", presetName)
			}

			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				raw, err := client.FetchV2Events(ctx, preset, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch events: %w", err)
				}

				events, err := icu.DecodeV2Events(raw)
				if err != nil {
					return fmt.Errorf("decode events: %w", err)
				}

				return writeJSON(events)
			})
		},
	}
}

func zeppSwitchCommand[A, B any](
	usage, description, flagName, defaultValue, optionA, optionB string,
	fetchA func(*icu.ZeppClient, context.Context, string, string) (A, error),
	fetchB func(*icu.ZeppClient, context.Context, string, string) (B, error),
) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			value := icu.StringFlag(flags, flagName, defaultValue)
			if value != optionA && value != optionB {
				return fmt.Errorf("invalid %s %q: must be %s or %s", flagName, value, optionA, optionB)
			}

			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				var records any

				var err error

				if value == optionA {
					records, err = fetchA(client, ctx, oldest, newest)
				} else {
					records, err = fetchB(client, ctx, oldest, newest)
				}

				if err != nil {
					return fmt.Errorf("fetch %s: %w", flagName, err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppBloodPressureCommand() *Command {
	return zeppSwitchCommand(
		"zepp blood-pressure --source watch|user --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		"Blood-pressure readings from watch events or the user BP endpoint.",
		"source", "watch", "watch", "user",
		(*icu.ZeppClient).BloodPressureDays,
		(*icu.ZeppClient).BloodPressureUser,
	)
}

func registerV2EventCommand[T any](
	registry *CommandRegistry,
	name string,
	preset icu.V2EventPreset,
	decoder func([]byte) ([]T, error),
	description string,
) {
	registry.Register("zepp", name, &Command{
		Name:        "",
		Usage:       fmt.Sprintf("zepp %s --oldest YYYY-MM-DD --newest YYYY-MM-DD", name),
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				raw, err := client.FetchV2Events(ctx, preset, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				decoded, err := decoder(raw)
				if err != nil {
					return fmt.Errorf("decode %s: %w", name, err)
				}

				return writeJSON(decoded)
			})
		},
	})
}

func registerWatchStatCommand[T any](
	registry *CommandRegistry,
	name string,
	fetch func(*icu.ZeppClient, context.Context, string, string) (T, error),
	description string,
) {
	registry.Register("zepp", name, &Command{
		Name:        "",
		Usage:       fmt.Sprintf("zepp %s --oldest YYYY-MM-DD --newest YYYY-MM-DD", name),
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := fetch(client, ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				return writeJSON(records)
			})
		},
	})
}

func zeppSingleDateCommand[T any](name, description string, fetch func(*icu.ZeppClient, context.Context, string, string) (T, error)) *Command {
	return &Command{
		Name:        "",
		Usage:       fmt.Sprintf("zepp %s --date YYYY-MM-DD [--timezone TZ]", name),
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			date := icu.StringFlag(flags, "date", "")
			if date == "" {
				return errors.New("date is required: use --date flag")
			}

			tz := icu.StringFlag(flags, "timezone", "UTC")

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				windows, err := fetch(client, ctx, date, tz)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				return writeJSON(windows)
			})
		},
	}
}

func zeppWorkoutsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp workouts [--sport NAME] [--oldest YYYY-MM-DD --newest YYYY-MM-DD]",
		Description: "List workouts recorded in Zepp. Defaults: sport=run, last 7 days.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			sport := icu.StringFlag(flags, "sport", "run")
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.Workouts(ctx, sport, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch workouts: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppWorkoutCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp workout [--sport NAME] <id>",
		Description: "Fetch a single workout detail (HR, pace, altitude series). Default sport=run.",
		Run: func(args []string, flags map[string]string, _ *icu.Client) error {
			sport := icu.StringFlag(flags, "sport", "run")

			if len(args) == 0 {
				return errMissing("zepp workout id")
			}

			trackID := args[0]

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				detail, err := client.Workout(ctx, sport, trackID)
				if err != nil {
					return fmt.Errorf("fetch workout detail: %w", err)
				}

				return writeJSON(detail)
			})
		},
	}
}

func zeppDateRange(flags map[string]string) (string, string) { //nolint:gocritic // unnamed results are clearer here
	oldest := icu.StringFlag(flags, "oldest", "")
	newest := icu.StringFlag(flags, "newest", "")

	if oldest == "" || newest == "" {
		defaultOldest, defaultNewest := zeppDefaultDateRange()
		if oldest == "" {
			oldest = defaultOldest
		}

		if newest == "" {
			newest = defaultNewest
		}
	}

	return oldest, newest
}

func zeppDefaultDateRange() (string, string) { //nolint:gocritic // unnamed results are clearer here
	now := time.Now()
	end := now.Format("2006-01-02")
	start := now.AddDate(0, 0, -zeppDefaultDateRangeDays+1).Format("2006-01-02")

	return start, end
}

func runZeppWithClient(fn func(ctx context.Context, client *icu.ZeppClient) error) error {
	loginToken := icu.ResolveZeppLoginToken(nil)
	appToken := icu.ResolveZeppAppToken(nil)
	userID := icu.ResolveZeppUserID(nil)
	countryCode := icu.ResolveZeppCountryCode(nil)

	if loginToken == "" || appToken == "" || userID == "" {
		return fmt.Errorf("%w", icu.ErrZeppNotAuthenticated)
	}

	auth := &icu.ZeppAuthResult{
		LoginToken:  loginToken,
		AppToken:    appToken,
		UserID:      userID,
		CountryCode: countryCode,
	}

	client := icu.NewZeppClientFromAuth(auth, zeppClientOptionsForRun()...)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) //nolint:mnd // reasonable timeout for Zepp API
	defer cancel()

	return fn(ctx, client)
}

func zeppClientOptionsForRun() []icu.ZeppClientOption {
	var options []icu.ZeppClientOption

	if baseURL := osGetenv("ZEPP_BASE_URL"); baseURL != "" {
		options = append(options, icu.WithZeppBaseURL(baseURL))
	}

	if eventsURL := osGetenv("ZEPP_EVENTS_URL"); eventsURL != "" {
		options = append(options, icu.WithZeppEventsURL(eventsURL))
	}

	return options
}

func zeppLogin(email, password string) (*icu.ZeppAuthResult, error) {
	tokensURL := osGetenv("ZEPP_TOKENS_URL")
	loginURL := osGetenv("ZEPP_LOGIN_URL")

	if tokensURL != "" && loginURL != "" {
		auth, err := icu.ZeppLoginWithURLs(tokensURL, loginURL, email, password)
		if err != nil {
			return nil, fmt.Errorf("zepp login with URLs: %w", err)
		}

		return auth, nil
	}

	auth, err := icu.ZeppLogin(email, password)
	if err != nil {
		return nil, fmt.Errorf("zepp login: %w", err)
	}

	return auth, nil
}

func runZeppToken(email, password string) error {
	auth, err := zeppLogin(email, password)
	if err != nil {
		return wrapCommandError(fmt.Errorf("zepp token failed: %w", err))
	}

	return writeJSON(map[string]any{
		"loginToken":  auth.LoginToken,
		"appToken":    auth.AppToken,
		"userId":      auth.UserID,
		"countryCode": auth.CountryCode,
	})
}

func runZeppLogin(email, password string) error {
	fmt.Fprintln(osStdout(), "Authenticating with Zepp...")

	auth, err := zeppLogin(email, password)
	if err != nil {
		return wrapCommandError(fmt.Errorf("zepp login failed: %w", err))
	}

	cfg, err := icu.LoadConfig()
	if err != nil {
		return wrapCommandError(fmt.Errorf("loading config: %w", err))
	}

	cfg.ZeppLoginToken = auth.LoginToken
	cfg.ZeppAppToken = auth.AppToken
	cfg.ZeppUserID = auth.UserID
	cfg.ZeppCountryCode = auth.CountryCode

	if err := icu.SaveConfig(cfg); err != nil {
		return wrapCommandError(fmt.Errorf("saving config: %w", err))
	}

	fmt.Fprintf(osStdout(), "Successfully logged in as user %s\n", auth.UserID)
	fmt.Fprintf(osStdout(), "Tokens saved to %s\n", icu.ConfigPath())

	return nil
}

func runZeppLogout() error {
	cfg, err := icu.LoadConfig()
	if err != nil {
		return wrapCommandError(fmt.Errorf("loading config: %w", err))
	}

	if cfg.ZeppLoginToken == "" {
		return writeJSON(map[string]any{"ok": true, "cleared": false})
	}

	cfg.ZeppLoginToken = ""

	if err := icu.SaveConfig(cfg); err != nil {
		return wrapCommandError(fmt.Errorf("saving config: %w", err))
	}

	return writeJSON(map[string]any{"ok": true, "cleared": true, "savedTo": icu.ConfigPath()})
}

func runZeppStatus(flags map[string]string) error {
	loginToken := icu.ResolveZeppLoginToken(flags)
	appToken := icu.ResolveZeppAppToken(flags)
	userID := icu.ResolveZeppUserID(flags)

	out := map[string]any{
		"set":         loginToken != "",
		"fingerprint": fingerprintForDisplay(loginToken),
		"length":      len(loginToken),
	}

	if loginToken == "" || appToken == "" || userID == "" {
		out["valid"] = false
		out["error"] = "no Zepp login token configured. Run 'icu zepp login'."

		return writeJSON(out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) //nolint:mnd // reasonable timeout for status check
	defer cancel()

	auth := &icu.ZeppAuthResult{
		LoginToken: loginToken,
		AppToken:   appToken,
		UserID:     userID,
	}

	client := icu.NewZeppClientFromAuth(auth, zeppClientOptionsForRun()...)

	info, err := client.UserInfo(ctx)
	if err != nil {
		out["valid"] = false
		out["error"] = err.Error()

		return writeJSON(out)
	}

	out["valid"] = true
	out["nickname"] = info.Nickname
	out["userId"] = info.UserID

	return writeJSON(out)
}

func runZeppProfile() error {
	return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
		info, err := client.UserInfo(ctx)
		if err != nil {
			return fmt.Errorf("fetch user info: %w", err)
		}

		return writeJSON(info)
	})
}

func fingerprintForDisplay(value string) string {
	return icu.SecretFingerprint(value)
}
