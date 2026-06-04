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
			WantOut: 0,
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
