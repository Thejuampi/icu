package icu_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestDiagnoseConfigReturnsErrorOnCorruptedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	icuDir := home + "/.icu"
	if err := os.MkdirAll(icuDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cfgPath := icuDir + "/config.json"
	if err := os.WriteFile(cfgPath, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	diag := icu.DiagnoseConfig(nil)
	if diag.ConfigError == "" {
		t.Fatalf("expected config error for corrupted file, got empty")
	}
}

func TestSaveConfigMkdirFailsOnFileAtPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	filePath := home + "/.icu"
	if err := os.WriteFile(filePath, []byte("blocker"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := &icu.Config{APIKey: "", AthleteID: "", Output: "", ZeppLoginToken: "test", ZeppAppToken: "", ZeppUserID: "", ZeppCountryCode: ""}

	if err := icu.SaveConfig(cfg); err == nil {
		t.Fatal("expected error: a file exists at the path where config dir should be")
	}
}

func TestBuildConfigDiagnosticHandlesNilConfig(t *testing.T) {
	t.Parallel()

	diag := icu.BuildConfigDiagnostic(nil, "", "", "", nil, "test.json", "")
	if diag.ConfigPath != "test.json" {
		t.Fatalf("ConfigPath = %q, want test.json", diag.ConfigPath)
	}
}

func TestDiagnoseConfigFallsBackToEmptyConfigOnLoadError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	diag := icu.DiagnoseConfig(nil)
	if diag.ConfigPath == "" {
		t.Fatalf("expected a config path, got empty")
	}
}

func TestBuildConfigDiagnosticPrefersAPIKeyFlag(t *testing.T) {
	t.Parallel()

	cfg := &icu.Config{APIKey: "config-key", AthleteID: "cfg-athlete", Output: "csv"}
	got := icu.BuildConfigDiagnostic(
		map[string]string{"api-key": "flag-key"},
		"env-key",
		"env-athlete",
		"env-zepp",
		cfg,
		"test-config.json",
		"",
	)

	if got.APIKey.ResolvedSource != "flag" {
		t.Fatalf("ResolvedSource = %q, want flag", got.APIKey.ResolvedSource)
	}
}

func TestBuildConfigDiagnosticDoesNotExposeAPIKey(t *testing.T) {
	t.Parallel()

	secret := "super-secret-api-key"
	cfg := &icu.Config{APIKey: secret, AthleteID: "", Output: ""}
	got := icu.BuildConfigDiagnostic(nil, "", "", "", cfg, "test-config.json", "")

	if strings.Contains(fmt.Sprintf("%+v", got), secret) {
		t.Fatalf("diagnostic exposed secret")
	}
}

func TestBuildConfigDiagnosticDoesNotExposeZeppLoginToken(t *testing.T) {
	t.Parallel()

	secret := "super-secret-zepp-token"
	cfg := &icu.Config{ZeppLoginToken: secret}
	got := icu.BuildConfigDiagnostic(nil, "", "", "", cfg, "test-config.json", "")

	if strings.Contains(fmt.Sprintf("%+v", got), secret) {
		t.Fatalf("diagnostic exposed zepp secret")
	}
}
