package main

import (
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestConfigDiagnoseDoesNotRequireAuth(t *testing.T) {
	t.Parallel()

	if commandRequiresAuth("config", "diagnose") {
		t.Fatalf("config diagnose requires auth")
	}
}

func TestConfigCommandsFunctionalRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("INTERVALS_ICU_API_KEY", "")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "")

	registry := NewCommandRegistry()
	registerConfigCommands(registry)

	if err := runConfigCommand(t, registry, "set", map[string]string{
		"api-key":    "abcd1234wxyz",
		"athlete-id": "i123",
		"output":     "json",
	}); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	if err := runConfigCommand(t, registry, "show", nil); err != nil {
		t.Fatalf("config show failed: %v", err)
	}

	if err := runConfigCommand(t, registry, "path", nil); err != nil {
		t.Fatalf("config path failed: %v", err)
	}

	if err := runConfigCommand(t, registry, "diagnose", nil); err != nil {
		t.Fatalf("config diagnose failed: %v", err)
	}

	data, err := os.ReadFile(icu.ConfigPath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	contents := string(data)

	if !strings.Contains(contents, "abcd1234wxyz") || !strings.Contains(contents, "i123") {
		t.Fatalf("config file = %q", contents)
	}
}

func TestConfigDiagnoseVerboseFlag(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerConfigCommands(registry)

	cmd, ok := registry.Lookup("config", "diagnose")
	if !ok {
		t.Fatal("missing config diagnose command")
	}

	if _, err := captureStdout(t, func() error { return cmd.Run(nil, nil, nil) }); err != nil {
		t.Fatalf("config diagnose without verbose: %v", err)
	}

	if _, err := captureStdout(t, func() error { return cmd.Run(nil, map[string]string{"verbose": "true"}, nil) }); err != nil {
		t.Fatalf("config diagnose with verbose: %v", err)
	}
}

func runConfigCommand(t *testing.T, registry *CommandRegistry, action string, flags map[string]string) error {
	t.Helper()

	cmd, ok := registry.Lookup("config", action)
	if !ok {
		t.Fatalf("missing config %s command", action)
	}

	return cmd.Run(nil, flags, nil)
}
