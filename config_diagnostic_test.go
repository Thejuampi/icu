package icu_test

import (
	"fmt"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestBuildConfigDiagnosticPrefersAPIKeyFlag(t *testing.T) {
	t.Parallel()

	cfg := &icu.Config{APIKey: "config-key", AthleteID: "cfg-athlete", Output: "csv"}
	got := icu.BuildConfigDiagnostic(
		map[string]string{"api-key": "flag-key"},
		"env-key",
		"env-athlete",
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
	got := icu.BuildConfigDiagnostic(nil, "", "", cfg, "test-config.json", "")

	if strings.Contains(fmt.Sprintf("%+v", got), secret) {
		t.Fatalf("diagnostic exposed secret")
	}
}
