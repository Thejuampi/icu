package main

import (
	"bytes"
	"strings"
	"testing"
)

type runCase struct {
	Name    string
	Args    []string
	WantOut int
	Check   func(t *testing.T, stdout, stderr string)
}

func TestRunDispatchHelpFlags(t *testing.T) {
	t.Parallel()

	cases := []runCase{
		{
			Name:    "no args",
			Args:    []string{"icu"},
			WantOut: 1,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "icu - Intervals.icu CLI", "global help header")
				checkContains(t, stdout, "Resources:", "resources section")
			},
		},
		{
			Name:    "icu help",
			Args:    []string{"icu", "help"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "icu - Intervals.icu CLI", "global help header")
			},
		},
		{
			Name:    "icu --help",
			Args:    []string{"icu", "--help"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "icu - Intervals.icu CLI", "global help header")
			},
		},
		{
			Name:    "icu -h",
			Args:    []string{"icu", "-h"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "icu - Intervals.icu CLI", "global help header")
			},
		},
		{
			Name:    "unknown resource --help shows global",
			Args:    []string{"icu", "bogus", "--help"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "icu - Intervals.icu CLI", "global help for unknown resource")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tc.Args, &stdout, &stderr)

			if code != tc.WantOut {
				t.Fatalf("run exit code = %d, want %d\nstdout: %s\nstderr: %s", code, tc.WantOut, stdout.String(), stderr.String())
			}

			tc.Check(t, stdout.String(), stderr.String())
		})
	}
}

func TestRunResourceHelp(t *testing.T) {
	t.Parallel()

	cases := []runCase{
		{
			Name:    "resource --help",
			Args:    []string{"icu", "ftp", "--help"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "Commands for ftp:", "resource help header")
				checkContains(t, stdout, "ftp show", "ftp show usage")
				checkContains(t, stdout, "ftp update", "ftp update usage")
				checkNotContains(t, stdout, "icu - Intervals.icu CLI", "should not be global help")
			},
		},
		{
			Name:    "resource -h",
			Args:    []string{"icu", "ftp", "-h"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "Commands for ftp:", "resource help via -h")
			},
		},
		{
			Name:    "resource with no show action defaults to help",
			Args:    []string{"icu", "activities"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "Commands for activities:", "resource help header")
				checkContains(t, stdout, "activities list", "activities list usage")
			},
		},
		{
			Name:    "resource with no show action - wellness",
			Args:    []string{"icu", "wellness"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "Commands for wellness:", "resource help header")
				checkContains(t, stdout, "wellness list", "wellness list usage")
				checkContains(t, stdout, "wellness get", "wellness get usage")
			},
		},
		{
			Name:    "idFirst resource with --help",
			Args:    []string{"icu", "activity", "i123", "--help"},
			WantOut: 0,
			Check: func(t *testing.T, stdout, _ string) {
				t.Helper()
				checkContains(t, stdout, "Commands for activity:", "idFirst resource help")
				checkContains(t, stdout, "activity <id> show", "idFirst show usage")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tc.Args, &stdout, &stderr)

			if code != tc.WantOut {
				t.Fatalf("run exit code = %d, want %d\nstdout: %s\nstderr: %s", code, tc.WantOut, stdout.String(), stderr.String())
			}

			tc.Check(t, stdout.String(), stderr.String())
		})
	}
}

func TestRunErrorMessages(t *testing.T) {
	t.Parallel()

	cases := []runCase{
		{
			Name:    "unknown resource falls back to show action",
			Args:    []string{"icu", "bogus"},
			WantOut: 1,
			Check: func(t *testing.T, _, stderr string) {
				t.Helper()
				checkContains(t, stderr, "Unknown command: bogus show", "unknown resource error")
			},
		},
		{
			Name:    "unknown action",
			Args:    []string{"icu", "activities", "show"},
			WantOut: 1,
			Check: func(t *testing.T, _, stderr string) {
				t.Helper()
				checkContains(t, stderr, "Unknown command: activities show", "unknown action error")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tc.Args, &stdout, &stderr)

			if code != tc.WantOut {
				t.Fatalf("run exit code = %d, want %d\nstdout: %s\nstderr: %s", code, tc.WantOut, stdout.String(), stderr.String())
			}

			tc.Check(t, stdout.String(), stderr.String())
		})
	}
}

func TestResourceHelpShowsDescriptions(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAllCommands(registry)

	var buf bytes.Buffer

	printHelp(registry, &buf, "ftp")

	output := buf.String()
	if !strings.Contains(output, "Commands for ftp:") {
		t.Fatalf("ftp help missing header: %s", output)
	}
}

func TestRunIDFirstWithPositionalArgsGivesErrorWithoutServer(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"icu", "activity", "i123", "show", "--fields", "id"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (no server)\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
}

func TestParseShortFlagWithValue(t *testing.T) {
	t.Parallel()

	flags := parseFlags([]string{"-o", "value", "pos"})

	if flags["o"] != "value" {
		t.Fatalf("short flag value = %q, want value", flags["o"])
	}

	if flags["_posargs_"] != "pos" {
		t.Fatalf("positional = %q, want pos", flags["_posargs_"])
	}
}

func TestRunMissingAPIKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("INTERVALS_ICU_API_KEY", "")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "")

	var stdout, stderr bytes.Buffer
	code := run([]string{"icu", "athlete"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run exit code = %d, want 1\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	if !strings.Contains(stderr.String(), "API key required") {
		t.Fatalf("missing API key error not found in stderr: %s", stderr.String())
	}
}

//nolint:paralleltest // Environment-based auth isolation prevents parallel subtests.
func TestAnalysisCoachingRejectsInvalidInputBeforeAuth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("INTERVALS_ICU_API_KEY", "")
	t.Setenv("INTERVALS_ICU_ATHLETE_ID", "")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown flag", args: []string{"--bogus"}, want: "unknown flag --bogus"},
		{name: "positional argument", args: []string{"junk"}, want: "unexpected positional argument"},
		{name: "missing string value", args: []string{"--sport-type"}, want: "--sport-type requires a value"},
		{name: "missing integer value", args: []string{"--history-days"}, want: "--history-days requires a value"},
		{name: "invalid integer", args: []string{"--history-days", "abc"}, want: "--history-days must be an integer"},
		{name: "non-positive integer", args: []string{"--history-days", "0"}, want: "--history-days must be greater than 0"},
		{name: "invalid boolean", args: []string{"--resolve", "sometimes"}, want: "--resolve must be a boolean"},
		{name: "malformed date", args: []string{"--history-oldest", "2026-02-30", "--history-newest", "2026-03-01"}, want: "--history-oldest must use YYYY-MM-DD"},
		{name: "inverted range", args: []string{"--plan-oldest", "2026-07-05", "--plan-newest", "2026-06-29"}, want: "--plan-oldest must be on or before --plan-newest"},
		{name: "conflicting range", args: []string{"--plan-oldest", "2026-06-29", "--plan-newest", "2026-07-26", "--plan-days", "28"}, want: "cannot be combined with --plan-days"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"icu", "analysis", "coaching"}, test.args...), &stdout, &stderr)

			if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("run = code %d stdout %q stderr %q, want code 1 empty stdout and %q", code, stdout.String(), stderr.String(), test.want)
			}
			if strings.Contains(stderr.String(), "API key required") {
				t.Fatalf("stderr = %q, validation must run before auth", stderr.String())
			}
		})
	}
}

func TestAnalysisCoachingBooleanValuesAreValid(t *testing.T) {
	t.Parallel()

	cmd := analysisCoachingCommand()
	for _, value := range []string{"", "true", "false", "1", "0"} {
		if err := validateCommandInput(cmd, nil, map[string]string{"resolve": value}); err != nil {
			t.Fatalf("validateCommandInput(resolve=%q) error = %v, want nil", value, err)
		}
	}
}

func TestLegacyCommandWithoutSchemaKeepsLegacyValidation(t *testing.T) {
	t.Parallel()

	if err := validateCommandInput(&Command{}, []string{"legacy"}, map[string]string{"unknown": "value"}); err != nil {
		t.Fatalf("validateCommandInput legacy error = %v, want nil", err)
	}
}

func TestSchemaWithoutCustomValidationAcceptsValidFlags(t *testing.T) {
	t.Parallel()

	command := &Command{Schema: &CommandSchema{Flags: []CommandFlag{{Name: "name", ValueName: "VALUE"}}}}
	if err := validateCommandInput(command, nil, map[string]string{"name": "valid"}); err != nil {
		t.Fatalf("validateCommandInput error = %v, want nil", err)
	}
}

func TestCommandSchemaRejectsUnsupportedFlagKind(t *testing.T) {
	t.Parallel()

	command := &Command{Schema: &CommandSchema{Flags: []CommandFlag{{Name: "invalid", Kind: CommandFlagKind(99)}}}}
	if err := validateCommandInput(command, nil, map[string]string{"invalid": "value"}); err == nil {
		t.Fatal("validateCommandInput error = nil, want unsupported kind error")
	}
}

func TestAnalysisCoachingHelpUsesCommandSchema(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"icu", "analysis", "coaching", "--help"}, &stdout, &stderr)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("run = code %d stderr %q, want code 0 and empty stderr", code, stderr.String())
	}
	for _, expected := range []string{
		"Usage: icu analysis coaching",
		"--history-days N",
		"default: 84",
		"--plan-days N",
		"default: 28",
		"--limit N",
		"--include-adaptation BOOL",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("help = %q, want %q", stdout.String(), expected)
		}
	}
}

func checkContains(t *testing.T, s, substr, desc string) {
	t.Helper()

	if !strings.Contains(s, substr) {
		t.Fatalf("%s: missing '%s' in:\n%s", desc, substr, s)
	}
}

func checkNotContains(t *testing.T, s, substr, desc string) {
	t.Helper()

	if strings.Contains(s, substr) {
		t.Fatalf("%s: unexpected '%s' in:\n%s", desc, substr, s)
	}
}
