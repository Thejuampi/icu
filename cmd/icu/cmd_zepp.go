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
	registry.Register("zepp", "summary", zeppSummaryCommand())
	registry.Register("zepp", "sleep", zeppSleepCommand())
	registry.Register("zepp", "heart-rate", zeppHeartRateCommand())
	registry.Register("zepp", "spo2", zeppSpO2Command())
	registry.Register("zepp", "stress", zeppStressCommand())
	registry.Register("zepp", "pai", zeppPAICommand())
	registry.Register("zepp", "workouts", zeppWorkoutsCommand())
	registry.Register("zepp", "workout", zeppWorkoutCommand())
}

func zeppTokenCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp token --email EMAIL [--password PASSWORD]",
		Description: "[debug] Get Zepp API tokens. WARNING: outputs tokens in plaintext. Use 'zepp login' for normal auth. Use this only when you need to inspect or manually manage tokens.", //nolint:lll // security warning
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			email := icu.StringFlag(flags, "email", "")

			if email == "" {
				return errors.New("email is required: use --email flag")
			}

			password, err := promptPasswordFlagOrInteractive(flags["password"])
			if err != nil {
				return err
			}

			return runZeppToken(email, password)
		},
	}
}

func zeppLoginCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp login --email EMAIL [--password PASSWORD]",
		Description: "Authenticate with Zepp using email and password to obtain login tokens.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			email := icu.StringFlag(flags, "email", "")

			if email == "" {
				return errors.New("email is required: use --email flag")
			}

			password, err := promptPasswordFlagOrInteractive(flags["password"])
			if err != nil {
				return err
			}

			return runZeppLogin(email, password)
		},
	}
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

func zeppSummaryCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp summary --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Daily band summary (steps, sleep, HR, SpO2, weight, stress).",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.BandData(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch band data: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppSleepCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp sleep --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Sleep records with stages, score, and average HR.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.SleepDays(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch sleep: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppHeartRateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp heart-rate --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Heart rate time series for the date range.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.HeartRateSeries(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch heart rate: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppSpO2Command() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp spo2 --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Blood oxygen measurements for the date range.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.SpO2Readings(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch SpO2: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppStressCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp stress --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Stress scores for the date range.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.StressDays(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch stress: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppPAICommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp pai --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Personal Activity Intelligence (PAI) daily scores and zone breakdown.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.PAIDays(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch PAI: %w", err)
				}

				return writeJSON(records)
			})
		},
	}
}

func zeppWorkoutsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp workouts [--oldest YYYY-MM-DD --newest YYYY-MM-DD]",
		Description: "List workouts recorded in Zepp. Defaults: last 7 days.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.Workouts(ctx, oldest, newest)
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
		Usage:       "zepp workout <id>",
		Description: "Fetch a single workout detail (HR, pace, altitude series).",
		Run: func(args []string, _ map[string]string, _ *icu.Client) error {
			if len(args) == 0 {
				return errMissing("zepp workout id")
			}

			trackID := args[0]

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				detail, err := client.Workout(ctx, trackID)
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

func runZeppToken(email, password string) error {
	tokensURL := osGetenv("ZEPP_TOKENS_URL")
	loginURL := osGetenv("ZEPP_LOGIN_URL")

	var auth *icu.ZeppAuthResult

	var err error

	if tokensURL != "" && loginURL != "" {
		auth, err = icu.ZeppLoginWithURLs(tokensURL, loginURL, email, password)
	} else {
		auth, err = icu.ZeppLogin(email, password)
	}

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

	tokensURL := osGetenv("ZEPP_TOKENS_URL")
	loginURL := osGetenv("ZEPP_LOGIN_URL")

	var auth *icu.ZeppAuthResult

	var err error

	if tokensURL != "" && loginURL != "" {
		auth, err = icu.ZeppLoginWithURLs(tokensURL, loginURL, email, password)
	} else {
		auth, err = icu.ZeppLogin(email, password)
	}

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
