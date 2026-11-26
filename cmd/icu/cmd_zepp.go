package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	zeppDefaultDateRangeDays = 7
	zeppLoginTimeout         = 5 * time.Minute

	// zeppBaseURLFlag points zepp login/token at another Zepp auth host
	// (e.g. a local mock). It is distinct from the global --base-url.
	zeppBaseURLFlag = "zepp-base-url"
	// zeppAllowInsecureHostFlag lets --zepp-base-url name a host that is
	// neither loopback nor a Zepp domain.
	zeppAllowInsecureHostFlag = "allow-insecure-zepp-host"
	zeppReauthenticateFlag    = "reauthenticate"
	zeppWebLoginFlag          = "web"
	zeppAndroidUnsafeFlag     = "android-unsafe"
	zeppTokensPath            = "/v2/registrations/tokens"
	zeppLoginPathPart         = "/v2/client/login"
)

const (
	zeppAndroidUnsafeWarning = "Warning: --android-unsafe uses the Android Zepp login and can sign out the phone app."
	zeppWebAliasMessage      = "Note: --web is the default and can be omitted." //nolint:gosec // CLI deprecation message, not a credential
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
	registry.Register("zepp", "second-heart-rate", zeppSecondHeartRateCommand())
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

func zeppAuthCommand(usage, description string, run func(stdout, stderr io.Writer, flags map[string]string) error) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			return run(stdout, stderr, flags)
		},
	}
}

type zeppLoginInput struct {
	email    string
	password string
	baseURL  string
}

func readZeppLoginInput(stderr io.Writer, flags map[string]string) (zeppLoginInput, error) {
	email := icu.StringFlag(flags, "email", "")
	if email == "" {
		return zeppLoginInput{}, errors.New("email is required: use --email flag")
	}

	password, err := promptPasswordFlagOrInteractive(stderr, flags["password"])
	if err != nil {
		return zeppLoginInput{}, err
	}

	// The global --base-url (intervals.icu) is stripped in executeCommand.
	baseURL := icu.StringFlag(flags, zeppBaseURLFlag, "")
	if err := checkZeppBaseURL(baseURL, zeppSecretPassword, BoolFlag(flags, zeppAllowInsecureHostFlag), stderr); err != nil {
		return zeppLoginInput{}, err
	}

	return zeppLoginInput{email: email, password: password, baseURL: baseURL}, nil
}

func zeppTokenCommand() *Command {
	return zeppAuthCommand(
		"zepp token --email EMAIL [--password PASSWORD] [--web] [--android-unsafe] [--zepp-base-url URL] [--allow-insecure-zepp-host]",
		"[debug] Get Zepp API tokens via the default web login. WARNING: outputs tokens in plaintext. Use 'zepp login' for normal auth. --android-unsafe uses the Android login and can sign out the phone app.", //nolint:lll // security warning
		func(stdout, stderr io.Writer, flags map[string]string) error {
			android, err := zeppUsesAndroidLogin(stderr, flags)
			if err != nil {
				return err
			}
			if err := rejectAndroidAuthOverrides(flags, android); err != nil {
				return err
			}

			input, err := readZeppLoginInput(stderr, flags)
			if err != nil {
				return err
			}

			return runZeppToken(stdout, input, android)
		},
	)
}

func zeppLoginCommand() *Command {
	return zeppAuthCommand(
		"zepp login --email EMAIL [--password PASSWORD] [--web] [--android-unsafe] [--zepp-base-url URL] [--allow-insecure-zepp-host] [--reauthenticate]",
		"Save a Zepp login. The default is the web login. --web is a deprecated alias for the default. --android-unsafe uses the Android login and can sign out the phone app. Use --reauthenticate when credentials are already saved.", //nolint:lll // command summary
		runZeppLogin,
	)
}

func zeppLogoutCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp logout",
		Description: "Clear Zepp credentials from the local config.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			return runZeppLogout(stdout)
		},
	}
}

func zeppStatusCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp status [--zepp-login-token TOKEN] [--zepp-app-token TOKEN] [--zepp-user-id ID]",
		Description: "Show whether a Zepp login token is configured (no raw value is printed).",
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			return runZeppStatus(stdout, flags)
		},
	}
}

func zeppProfileCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp profile",
		Description: "Fetch the Zepp user profile for the authenticated account.",
		Run: func(_ []string, _ map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			return runZeppProfile(stdout)
		},
	}
}

func zeppSimpleCommand[T any](name, description string, fetch func(*icu.ZeppClient, context.Context, string, string) (T, error)) *Command {
	return &Command{
		Name:        "",
		Usage:       fmt.Sprintf("zepp %s --oldest YYYY-MM-DD --newest YYYY-MM-DD", name),
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := fetch(client, ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				return writeJSON(stdout, records)
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
	return &Command{
		Name:  "",
		Usage: "zepp hrv --metric sdnn|rmssd|sleephrv --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Nightly HRV from /v2/users/me/events. " +
			"sleephrv is the nightly value the Zepp app shows (readiness sleepHRV, the source wellness sync writes); " +
			"rmssd is the mean of the night's samples (an approximation); sdnn may be empty.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			metric := icu.StringFlag(flags, "metric", "sleephrv")
			if metric != "sdnn" && metric != "rmssd" && metric != "sleephrv" {
				return fmt.Errorf("invalid metric %q: must be sdnn, rmssd or sleephrv", metric)
			}

			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				var records any

				var err error

				switch metric {
				case "sdnn":
					records, err = client.HRVSDNNDays(ctx, oldest, newest)
				case "rmssd":
					records, err = client.HRVRMSSDDays(ctx, oldest, newest)
				case "sleephrv":
					records, err = client.SleepHRVDays(ctx, oldest, newest)
				}

				if err != nil {
					return fmt.Errorf("fetch hrv %s: %w", metric, err)
				}

				return writeJSON(stdout, records)
			})
		},
	}
}

func zeppSecondHeartRateCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "zepp second-heart-rate --oldest YYYY-MM-DD --newest YYYY-MM-DD",
		Description: "Per-second heart-rate COS file index from /users/me/fileInfo/events. " +
			"Blobs are not downloaded or parsed (the blob format is not documented). " +
			"URLs are redacted (userinfo, query and fragment stripped) because they may carry temporary signatures.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.SecondHeartRateFiles(ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch second-heart-rate: %w", err)
				}

				for i := range records {
					records[i].URL, _ = redactURL(records[i].URL)
				}

				return writeJSON(stdout, records)
			})
		},
	}
}

func zeppEventsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp events --preset NAME --oldest YYYY-MM-DD --newest YYYY-MM-DD [--raw]",
		Description: "Fetch V2 wellness events by preset." + rawHelp,
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
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

				if BoolFlag(flags, zeppRawFlag) {
					return writeZeppRaw(stdout, raw)
				}

				events, err := icu.DecodeV2Events(raw)
				if err != nil {
					return fmt.Errorf("decode events: %w", err)
				}

				return writeJSON(stdout, events)
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
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
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

				return writeJSON(stdout, records)
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
		Usage:       fmt.Sprintf("zepp %s --oldest YYYY-MM-DD --newest YYYY-MM-DD [--raw]", name),
		Description: description + rawHelp,
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				raw, err := client.FetchV2Events(ctx, preset, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				if BoolFlag(flags, zeppRawFlag) {
					return writeZeppRaw(stdout, raw)
				}

				decoded, err := decoder(raw)
				if err != nil {
					return fmt.Errorf("decode %s: %w", name, err)
				}

				return writeJSON(stdout, decoded)
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
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := fetch(client, ctx, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch %s: %w", name, err)
				}

				return writeJSON(stdout, records)
			})
		},
	})
}

func zeppSingleDateCommand[T any](name, description string, fetch func(*icu.ZeppClient, context.Context, string, string) (T, error)) *Command {
	return &Command{
		Name:        "",
		Usage:       fmt.Sprintf("zepp %s --date YYYY-MM-DD [--timezone TZ]", name),
		Description: description,
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
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

				return writeJSON(stdout, windows)
			})
		},
	}
}

func zeppWorkoutsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp workouts [--sport NAME] [--oldest YYYY-MM-DD --newest YYYY-MM-DD]",
		Description: "List workouts recorded in Zepp. Defaults: sport=run, last 7 days.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
			sport := icu.StringFlag(flags, "sport", "run")
			oldest, newest := zeppDateRange(flags)

			return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
				records, err := client.Workouts(ctx, sport, oldest, newest)
				if err != nil {
					return fmt.Errorf("fetch workouts: %w", err)
				}

				return writeJSON(stdout, records)
			})
		},
	}
}

func zeppWorkoutCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "zepp workout [--sport NAME] <id>",
		Description: "Fetch a single workout detail (HR, pace, altitude series). Default sport=run.",
		Run: func(args []string, flags map[string]string, _ *icu.Client, stdout, stderr io.Writer) error {
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

				return writeJSON(stdout, detail)
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
	return runZeppWithClientOptions(zeppClientOptionsForRun(), fn)
}

func runZeppWithClientOptions(options []icu.ZeppClientOption, fn func(ctx context.Context, client *icu.ZeppClient) error) error {
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

	client := icu.NewZeppClientFromAuth(auth, options...)

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

func zeppLogin(email, password, zeppBaseURL, deviceID string) (*icu.ZeppSession, error) {
	options := icu.ZeppLoginOptions{
		DeviceID:  deviceID,
		TokensURL: osGetenv("ZEPP_TOKENS_URL"),
		LoginURL:  osGetenv("ZEPP_LOGIN_URL"),
	}

	if base := strings.TrimRight(zeppBaseURL, "/"); base != "" {
		options.TokensURL, options.LoginURL = base+zeppTokensPath, base+zeppLoginPathPart
	}

	auth, err := icu.ZeppLoginSession(email, password, options)
	if err != nil {
		return nil, fmt.Errorf("zepp login: %w", err)
	}

	return auth, nil
}

func runZeppToken(stdout io.Writer, input zeppLoginInput, android bool) error {
	auth, err := authenticateZeppLogin(input, "", android)
	if err != nil {
		return wrapCommandError(fmt.Errorf("zepp token failed: %w", err))
	}

	return writeJSON(stdout, map[string]any{
		"loginToken":  auth.LoginToken,
		"appToken":    auth.AppToken,
		"userId":      auth.UserID,
		"countryCode": auth.CountryCode,
	})
}

func runZeppLogin(stdout, stderr io.Writer, flags map[string]string) error {
	previous, err := icu.LoadConfig()
	if err != nil {
		return wrapCommandError(fmt.Errorf("loading config: %w", err))
	}
	android, err := zeppUsesAndroidLogin(stderr, flags)
	if err != nil {
		return err
	}

	if (previous.ZeppLoginToken != "" || previous.ZeppAppToken != "") && !BoolFlag(flags, zeppReauthenticateFlag) {
		return errors.New("credentials for Zepp are already saved. Use them for reads, or pass --reauthenticate to sign in again")
	}

	if err := rejectAndroidAuthOverrides(flags, android); err != nil {
		return err
	}

	input, err := readZeppLoginInput(stderr, flags)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "Authenticating with Zepp...")

	auth, err := authenticateZeppLogin(input, previous.ZeppDeviceID, android)
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
	if auth.DeviceID != "" {
		cfg.ZeppDeviceID = auth.DeviceID
	}

	if err := icu.SaveConfigWithAction(cfg, "zepp_login"); err != nil {
		return wrapCommandError(fmt.Errorf("saving config: %w", err))
	}

	fmt.Fprintf(stdout, "Successfully logged in as user %s\n", auth.UserID)
	fmt.Fprintf(stdout, "Tokens saved to %s\n", icu.ConfigPath())

	return nil
}

func authenticateZeppLogin(input zeppLoginInput, deviceID string, android bool) (*icu.ZeppSession, error) {
	if android {
		return zeppLogin(input.email, input.password, input.baseURL, deviceID)
	}

	auth, err := icu.ZeppWebLogin(input.email, input.password, input.baseURL)
	if err != nil {
		return nil, fmt.Errorf("web login: %w", err)
	}

	return &icu.ZeppSession{ZeppAuthResult: auth}, nil
}

// zeppUsesAndroidLogin reports whether the caller opted into the Android
// login. The Android path is reached only through --android-unsafe.
// --web or --web=true is a deprecated alias for the default web path and
// cannot be combined with --android-unsafe. --web=false does not select
// Android; with --android-unsafe it is allowed and uses the Android path.
func zeppUsesAndroidLogin(stderr io.Writer, flags map[string]string) (bool, error) {
	if err := validateCommandFlag(CommandFlag{Name: zeppWebLoginFlag, Kind: commandFlagBoolean}, flags[zeppWebLoginFlag]); err != nil {
		return false, err
	}
	if err := validateCommandFlag(CommandFlag{Name: zeppAndroidUnsafeFlag, Kind: commandFlagBoolean}, flags[zeppAndroidUnsafeFlag]); err != nil {
		return false, err
	}
	android := BoolFlag(flags, zeppAndroidUnsafeFlag)
	web := BoolFlag(flags, zeppWebLoginFlag)
	if android && web {
		return false, errors.New("cannot combine --web and --android-unsafe")
	}
	if android {
		fmt.Fprintln(stderr, zeppAndroidUnsafeWarning)

		return true, nil
	}
	if web {
		fmt.Fprintln(stderr, zeppWebAliasMessage)
	}

	return false, nil
}

func rejectAndroidAuthOverrides(flags map[string]string, android bool) error {
	if android || icu.StringFlag(flags, zeppBaseURLFlag, "") != "" {
		return nil
	}
	if osGetenv("ZEPP_TOKENS_URL") != "" || osGetenv("ZEPP_LOGIN_URL") != "" {
		return errors.New("use --zepp-base-url for custom auth endpoints")
	}

	return nil
}

func runZeppLogout(stdout io.Writer) error {
	cfg, err := icu.LoadConfig()
	if err != nil {
		return wrapCommandError(fmt.Errorf("loading config: %w", err))
	}

	if cfg.ZeppLoginToken == "" && cfg.ZeppAppToken == "" && cfg.ZeppUserID == "" && cfg.ZeppCountryCode == "" {
		return writeJSON(stdout, map[string]any{"ok": true, "cleared": false})
	}

	cfg.ZeppLoginToken = ""
	cfg.ZeppAppToken = ""
	cfg.ZeppUserID = ""
	cfg.ZeppCountryCode = ""

	if err := icu.SaveConfigWithAction(cfg, "zepp_logout"); err != nil {
		return wrapCommandError(fmt.Errorf("saving config: %w", err))
	}

	return writeJSON(stdout, map[string]any{"ok": true, "cleared": true, "savedTo": icu.ConfigPath()})
}

func runZeppStatus(stdout io.Writer, flags map[string]string) error {
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

		return writeJSON(stdout, out)
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

		return writeJSON(stdout, out)
	}

	out["valid"] = true
	out["nickname"] = info.Nickname
	out["userId"] = info.UserID

	return writeJSON(stdout, out)
}

func runZeppProfile(stdout io.Writer) error {
	return runZeppWithClient(func(ctx context.Context, client *icu.ZeppClient) error {
		info, err := client.UserInfo(ctx)
		if err != nil {
			return fmt.Errorf("fetch user info: %w", err)
		}

		return writeJSON(stdout, info)
	})
}

func fingerprintForDisplay(value string) string {
	return icu.SecretFingerprint(value)
}

// Secrets --zepp-base-url receives, named in checkZeppBaseURL messages.
const (
	zeppSecretPassword = "Zepp password"
	zeppSecretAppToken = "Zepp app token" //nolint:gosec // G101: a label for messages, not a credential
)

// checkZeppBaseURL guards --zepp-base-url, which receives a Zepp secret (the
// password for zepp login/token, the app token for wellness sync): plain http
// is refused unless the host is loopback, and any host that is neither
// loopback nor under zepp.com/huami.com is refused unless allowOther
// (--allow-insecure-zepp-host), in which case it only warns on stderr.
func checkZeppBaseURL(raw, secret string, allowOther bool, stderr io.Writer) error {
	if raw == "" {
		return nil
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%w: --%s must be an http(s) URL with a host, got %q", errInvalidValue, zeppBaseURLFlag, raw)
	}

	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	loopback := isLoopbackHost(host)

	if parsed.Scheme == "http" && !loopback {
		return fmt.Errorf("%w: --%s refuses plain http for non-loopback host %q; use https", errInvalidValue, zeppBaseURLFlag, host)
	}

	if loopback {
		return nil
	}

	// Hosts (and subdomains) that legitimately serve Zepp authentication.
	for _, domain := range []string{"zepp.com", "huami.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return nil
		}
	}

	if !allowOther {
		return fmt.Errorf("%w: --%s host %q is not a Zepp host (zepp.com, huami.com) or loopback; your %s would be sent there. Pass --%s to allow it",
			errInvalidValue, zeppBaseURLFlag, host, secret, zeppAllowInsecureHostFlag)
	}

	_, _ = fmt.Fprintf(stderr, "warning: --%s host %q is not a Zepp host (zepp.com, huami.com); your %s will be sent there\n", zeppBaseURLFlag, host, secret)

	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

const (
	zeppRawFlag = "raw"
	rawHelp     = " --raw prints the Zepp JSON as received with identifiers and secrets redacted" +
		" (identifier keys, any key ending in id/ids; strings unless their key is a known-safe field;" +
		" emails, MACs, JWTs, UUIDs, URLs and blobs everywhere; JSON inside strings too). See docs/cli-reference.md."
)

// writeZeppRaw prints the Zepp response as received, with every identifier
// and secret redacted. It never prints the unredacted body, even on error.
func writeZeppRaw(stdout io.Writer, raw []byte) error {
	redacted, err := icu.RedactZeppJSON(raw)
	if err != nil {
		return fmt.Errorf("raw output: response is not JSON, not printed: %w", err)
	}

	if _, err := stdout.Write(append(redacted, '\n')); err != nil {
		return fmt.Errorf("write raw: %w", err)
	}

	return nil
}
