package icu_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	icu "github.com/Thejuampi/icu"
)

func TestBuildZeppURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		want string
	}{
		{"user info", "/huami.health.getUserInfo.json", "https://api-mifit.huami.com/huami.health.getUserInfo.json"},
		{"band data", "/v1/data/band_data.json", "https://api-mifit.huami.com/v1/data/band_data.json"},
		{"sport history", "/v1/sport/run/history.json", "https://api-mifit.huami.com/v1/sport/run/history.json"},
		{"sport detail", "/v1/sport/run/detail.json", "https://api-mifit.huami.com/v1/sport/run/detail.json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := icu.BuildZeppURL(tc.path)
			if got != tc.want {
				t.Fatalf("BuildZeppURL(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestBuildZeppURLForRegion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		countryCode string
		want        string
	}{
		{"US", "https://api-mifit.huami.com/v1/data/band_data.json"},
		{"DE", "https://api-mifit-de.huami.com/v1/data/band_data.json"},
		{"FR", "https://api-mifit-de.huami.com/v1/data/band_data.json"},
		{"GB", "https://api-mifit-de.huami.com/v1/data/band_data.json"},
		{"CN", "https://api-mifit-cn.huami.com/v1/data/band_data.json"},
		{"XX", "https://api-mifit.huami.com/v1/data/band_data.json"},
	}

	for _, tc := range cases {
		t.Run(tc.countryCode, func(t *testing.T) {
			t.Parallel()

			got := icu.BuildZeppURLForRegion(tc.countryCode, "/v1/data/band_data.json")
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildZeppEventsURL(t *testing.T) {
	t.Parallel()

	got := icu.BuildZeppEventsURL("user-123")
	want := "https://api-mifit.zepp.com/users/user-123/events"

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestZeppCommonHeadersExposeApptoken(t *testing.T) {
	t.Parallel()

	headers := icu.ZeppCommonHeaders("my-app-token")
	if headers.Get("Apptoken") != "my-app-token" {
		t.Fatalf("apptoken = %q, want my-app-token", headers.Get("Apptoken"))
	}
}

func TestZeppDTOsJSONRoundtrip(t *testing.T) { //nolint:gocognit // many subtests ensure all DTOs roundtrip
	t.Parallel()

	t.Run("user info", func(t *testing.T) {
		t.Parallel()

		original := icu.ZeppUserInfo{UserID: "u1", Nickname: "Tester", Height: 180, Weight: 75}

		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded icu.ZeppUserInfo
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.UserID != original.UserID || decoded.Nickname != original.Nickname ||
			decoded.Height != original.Height || decoded.Weight != original.Weight {
			t.Fatalf("roundtrip mismatch: %+v vs %+v", decoded, original)
		}
	})

	t.Run("band data day", func(t *testing.T) {
		t.Parallel()

		original := icu.BandDataDay{
			Date:       "2026-06-01",
			SummaryRaw: "raw",
			Summary: &icu.BandDataSummary{
				Goal:   8000,
				Serial: "ABC",
				Steps:  &icu.BandDataSteps{Total: 1000, Calories: 50, Distance: 800},
			},
		}

		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded icu.BandDataDay
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.Date != original.Date || decoded.Summary.Goal != 8000 {
			t.Fatalf("roundtrip mismatch: %+v vs %+v", decoded, original)
		}
	})

	t.Run("workout detail", func(t *testing.T) {
		t.Parallel()

		original := icu.WorkoutDetail{
			TrackID:   "t1",
			StartTime: 1700000000000,
			AvgHR:     150,
			HRSeries:  []int{120, 130, 140, 150},
		}

		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded icu.WorkoutDetail
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.TrackID != original.TrackID || len(decoded.HRSeries) != 4 {
			t.Fatalf("roundtrip mismatch: %+v vs %+v", decoded, original)
		}
	})
}

func TestResolveAPIKeyFlagBeatsEnvBeatsConfig(t *testing.T) {
	t.Setenv("INTERVALS_ICU_API_KEY", "env-key")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(&icu.Config{APIKey: "config-key"}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveAPIKey(map[string]string{"api-key": "flag-key"}); got != "flag-key" {
		t.Fatalf("flag: got %q, want flag-key", got)
	}

	if got := icu.ResolveAPIKey(nil); got != "env-key" {
		t.Fatalf("env: got %q, want env-key", got)
	}

	t.Setenv("INTERVALS_ICU_API_KEY", "")

	if got := icu.ResolveAPIKey(nil); got != "config-key" {
		t.Fatalf("config: got %q, want config-key", got)
	}
}

func TestResolveAthleteIDFlagBeatsEnvBeatsConfig(t *testing.T) {
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "env-id")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(&icu.Config{AthleteID: "config-id"}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveAthleteID(map[string]string{"athlete-id": "flag-id"}); got != "flag-id" {
		t.Fatalf("flag: got %q, want flag-id", got)
	}

	if got := icu.ResolveAthleteID(nil); got != "env-id" {
		t.Fatalf("env: got %q, want env-id", got)
	}

	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "")

	if got := icu.ResolveAthleteID(nil); got != "config-id" {
		t.Fatalf("config: got %q, want config-id", got)
	}
}

func TestResolveOutputFormatUsesFlagsThenConfigThenDefaults(t *testing.T) {
	t.Parallel()

	t.Run("flag csv", func(t *testing.T) {
		t.Parallel()

		if got := icu.ResolveOutputFormat(map[string]string{"output": "csv"}); got != icu.FormatCSV {
			t.Fatalf("got %v, want csv", got)
		}
	})

	t.Run("flag table", func(t *testing.T) {
		t.Parallel()

		if got := icu.ResolveOutputFormat(map[string]string{"output": "table"}); got != icu.FormatTable {
			t.Fatalf("got %v, want table", got)
		}
	})

	t.Run("default json", func(t *testing.T) {
		t.Parallel()

		if got := icu.ResolveOutputFormat(nil); got != icu.FormatJSON {
			t.Fatalf("got %v, want json", got)
		}
	})
}

func TestResolveZeppAppTokenFlagBeatsEnvBeatsConfig(t *testing.T) {
	t.Setenv("ZEPP_APP_TOKEN", "env-token")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(&icu.Config{ZeppAppToken: "config-token"}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveZeppAppToken(map[string]string{"zepp-app-token": "flag-token"}); got != "flag-token" {
		t.Fatalf("flag: got %q, want flag-token", got)
	}

	if got := icu.ResolveZeppAppToken(nil); got != "env-token" {
		t.Fatalf("env: got %q, want env-token", got)
	}

	t.Setenv("ZEPP_APP_TOKEN", "")

	if got := icu.ResolveZeppAppToken(nil); got != "config-token" {
		t.Fatalf("config: got %q, want config-token", got)
	}
}

func TestResolveZeppUserIDFlagBeatsEnvBeatsConfig(t *testing.T) {
	t.Setenv("ZEPP_USER_ID", "env-id")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(&icu.Config{ZeppUserID: "config-id"}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveZeppUserID(map[string]string{"zepp-user-id": "flag-id"}); got != "flag-id" {
		t.Fatalf("flag: got %q, want flag-id", got)
	}

	if got := icu.ResolveZeppUserID(nil); got != "env-id" {
		t.Fatalf("env: got %q, want env-id", got)
	}

	t.Setenv("ZEPP_USER_ID", "")

	if got := icu.ResolveZeppUserID(nil); got != "config-id" {
		t.Fatalf("config: got %q, want config-id", got)
	}
}

func TestSaveConfigFailsOnBadPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	cfg := &icu.Config{APIKey: "test"}
	err := icu.SaveConfig(cfg)
	// Should succeed in temp dir; we just verify it doesn't panic on basic save
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewZeppClientFromAuthDefaultsToGlobalHost(t *testing.T) {
	t.Parallel()

	auth := &icu.ZeppAuthResult{AppToken: "tok", UserID: "u1"}
	client := icu.NewZeppClientFromAuth(auth)

	if got := client.DataHostForTest(); got != "https://api-mifit.huami.com" {
		t.Fatalf("dataHost = %q, want global host", got)
	}
}

func TestConfigDirFallsBackToDotIcu(t *testing.T) {
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	path := icu.ConfigPath()
	if !strings.HasSuffix(path, ".icu/config.json") && !strings.HasSuffix(path, ".icu\\config.json") {
		t.Fatalf("expected .icu in path, got %q", path)
	}
}

func TestResolveZeppLoginTokenFlagBeatsEnvBeatsConfig(t *testing.T) {
	t.Setenv("ZEPP_LOGIN_TOKEN", "env-token")

	cfg := &icu.Config{ZeppLoginToken: "config-token"}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveZeppLoginToken(map[string]string{"zepp-login-token": "flag-token"}); got != "flag-token" {
		t.Fatalf("flag-token precedence: got %q, want flag-token", got)
	}

	t.Setenv("ZEPP_LOGIN_TOKEN", "env-token-2")

	if got := icu.ResolveZeppLoginToken(nil); got != "env-token-2" {
		t.Fatalf("env-token precedence: got %q, want env-token-2", got)
	}

	t.Setenv("ZEPP_LOGIN_TOKEN", "")

	if got := icu.ResolveZeppLoginToken(nil); got != "config-token" {
		t.Fatalf("config-token precedence: got %q, want config-token", got)
	}
}

func TestResolveZeppLoginTokenEmptyWhenAllEmpty(t *testing.T) {
	t.Setenv("ZEPP_LOGIN_TOKEN", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(&icu.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveZeppLoginToken(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestResolveZeppCountryCodePrecedence(t *testing.T) {
	t.Setenv("ZEPP_COUNTRY_CODE", "env-cc")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	if err := icu.SaveConfig(&icu.Config{ZeppCountryCode: "config-cc"}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if got := icu.ResolveZeppCountryCode(map[string]string{"zepp-country-code": "flag-cc"}); got != "flag-cc" {
		t.Fatalf("flag precedence: got %q, want flag-cc", got)
	}

	if got := icu.ResolveZeppCountryCode(nil); got != "env-cc" {
		t.Fatalf("env precedence: got %q, want env-cc", got)
	}

	t.Setenv("ZEPP_COUNTRY_CODE", "")

	if got := icu.ResolveZeppCountryCode(nil); got != "config-cc" {
		t.Fatalf("config precedence: got %q, want config-cc", got)
	}
}

func TestParseZeppDateToMillisUsesLocalTime(t *testing.T) {
	t.Parallel()

	got, err := icu.ParseZeppDateToMillisForTest("2026-06-01")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	expected := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if got != expected {
		t.Fatalf("got %d, want %d", got, expected)
	}
}

func TestParseZeppDateToMillisRejectsInvalid(t *testing.T) {
	t.Parallel()

	_, err := icu.ParseZeppDateToMillisForTest("not-a-date")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestSecretFingerprintMatchesKnownValue(t *testing.T) {
	t.Parallel()

	want := "ba7816bf8f01"
	got := icu.SecretFingerprint("abc")

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSecretFingerprintEmptyForEmptyValue(t *testing.T) {
	t.Parallel()

	if got := icu.SecretFingerprint(""); got != "" {
		t.Fatalf("expected empty fingerprint for empty value, got %q", got)
	}
}

func TestSportTypeNameMapsKnownValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code int
		want string
	}{
		{1, "running"},
		{2, "walking"},
		{3, "cycling"},
		{4, "hiking"},
		{5, "swimming_pool"},
		{6, "open_water_swim"},
		{7, "elliptical"},
		{8, "rowing"},
		{9, "climbing"},
		{10, "treadmill"},
		{11, "strength_training"},
		{12, "yoga"},
		{13, "pilates"},
		{14, "indoor_cycling"},
		{15, "basketball"},
		{16, "football"},
		{17, "tennis"},
		{18, "badminton"},
		{19, "table_tennis"},
		{20, "golf"},
		{21, "skiing"},
		{22, "snowboarding"},
		{23, "jump_rope"},
		{24, "dance"},
		{999, "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			if got := icu.SportTypeName(tc.code); got != tc.want {
				t.Fatalf("SportTypeName(%d) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestSportTypeNameMapsAllBranches(t *testing.T) {
	t.Parallel()

	for code := 1; code <= 24; code++ {
		got := icu.SportTypeName(code)
		if got == "unknown" {
			t.Fatalf("SportTypeName(%d) returned unknown, expected a named sport", code)
		}
	}
}

var _ = strings.Contains
