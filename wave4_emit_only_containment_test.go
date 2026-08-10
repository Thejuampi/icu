package icu_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// wave4EmitOnlyFieldNames is T4.3's containment list: the Go identifiers for
// wave-4's nine emit-only fields (D-1/A2) declared on Activity.
var wave4EmitOnlyFieldNames = []string{ //nolint:gochecknoglobals // fixed enumerated list (T4.3), see doc comment above.
	"PolarizationIndex",
	"SSCP",
	"SSWPrime",
	"SSPMax",
	"CarbsUsed",
	"CarbsIngested",
	"RollingFTPDelta",
	"PowerField",
	"HRR",
}

// wave4EmitOnlyDefinitionFiles are the type's own definition and wire
// wiring: declaring the fields (types.go) and transporting raw snake_case
// values into them (activity_json.go) is plumbing, not consumption.
var wave4EmitOnlyDefinitionFiles = map[string]bool{ //nolint:gochecknoglobals // fixed enumerated list (T4.3), see doc comment above.
	"types.go":         true,
	"activity_json.go": true,
}

// TestWaveFourEmitOnlyFieldsStayContained is T4.3: none of wave-4's nine
// emit-only fields may be referenced anywhere in this package's production
// (non-test) source outside their own type definition/wire wiring, or
// inside cyclingSessionFromActivity (analysis.go) -- the two locations the
// plan permits. As of wave-4, cyclingSessionFromActivity has zero
// references to any of the nine (extending it would add the fields to
// every analysis output's per-session rows and move the wave-2 golden,
// which this wave's own done-when criterion forbids); this test only
// forbids that changing silently, it does not forbid it outright -- a
// deliberate future change there would still need AGENTS.md's T4.4
// classification exercise.
func TestWaveFourEmitOnlyFieldsStayContained(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	fieldMatchers := make(map[string]*regexp.Regexp, len(wave4EmitOnlyFieldNames))
	for _, field := range wave4EmitOnlyFieldNames {
		fieldMatchers[field] = regexp.MustCompile(`\b` + field + `\b`)
	}

	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if wave4EmitOnlyDefinitionFiles[name] {
			continue
		}

		contents, readErr := os.ReadFile(filepath.Join(".", name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}

		lines := strings.Split(string(contents), "\n")
		permittedStart, permittedEnd := -1, -1
		if name == "analysis.go" {
			permittedStart, permittedEnd = cyclingSessionFromActivityLineRange(lines)
		}

		for lineIndex, line := range lines {
			if permittedStart != -1 && lineIndex >= permittedStart && lineIndex <= permittedEnd {
				continue
			}

			code := stripLineComment(line)
			for _, field := range wave4EmitOnlyFieldNames {
				if fieldMatchers[field].MatchString(code) {
					violations = append(violations, fmt.Sprintf(
						"%s:%d references emit-only field %q outside its permitted footprint: %q",
						name, lineIndex+1, field, strings.TrimSpace(line),
					))
				}
			}
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("T4.3 emit-only containment broken (%d violation(s)):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

// stripLineComment truncates a line at its first "//", so an English word
// coincidentally matching a field name in prose (e.g. "HRR" the heart-rate-
// recovery abbreviation, used in metrics.go's unrelated pure computation)
// does not trip the containment check. A line-oriented heuristic, not a
// tokenizer -- consistent with T4.3's own "grep-shaped" acceptance
// language.
func stripLineComment(line string) string {
	if index := strings.Index(line, "//"); index != -1 {
		return line[:index]
	}

	return line
}

// cyclingSessionFromActivityLineRange returns the zero-based [start, end]
// line indices (inclusive) of cyclingSessionFromActivity's function body in
// analysis.go, found by brace-depth scanning from its declaration line -- a
// grep-shaped boundary, matching T4.3's own acceptance language, not a full
// parse.
func cyclingSessionFromActivityLineRange(lines []string) (int, int) {
	start := -1
	depth := 0

	for index, line := range lines {
		if start == -1 {
			if !strings.Contains(line, "func cyclingSessionFromActivity(") {
				continue
			}
			start = index
		}

		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth == 0 {
			return start, index
		}
	}

	return -1, -1
}
