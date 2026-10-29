package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	icu "github.com/Thejuampi/icu"
)

// Wire (snake_case) field names accepted by PUT /athlete/{id}/sport-settings/{id}.
// The icu.SportSettings struct tags are camelCase and are ignored by the API on
// write, so update bodies are built as maps keyed by these names.
const (
	wireFTP        = "ftp"
	wireIndoorFTP  = "indoor_ftp"
	wireLTHR       = "lthr"
	wireMaxHR      = "max_hr"
	wireHRZones    = "hr_zones"
	wirePowerZones = "power_zones"

	// powerZoneOpenTop is the intervals.icu sentinel for the open-ended top
	// power zone (% of FTP).
	powerZoneOpenTop = 999
)

func registerSportsCommands(registry *CommandRegistry) {
	registry.Register("sports", "list", listAllCommand[[]icu.SportSettings]("sport-settings", "sports list", "List all sport settings."))
	registry.Register("sports", "get", getByIDCommand[icu.SportSettings]("sport-settings", "sports get <type|id>", "Get sport settings by type or ID.", "sport type or id", nil))
	registry.Register("sports", "update", sportsUpdateCommand())
	registry.Register("sports", "delete", deleteByIDCommand("sport-settings", "sports delete <id>", "Delete sport settings.", "sport settings id"))
}

type sportFieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

type sportsUpdateResult struct {
	ID       string             `json:"id"`
	Changes  []sportFieldChange `json:"changes"`
	Body     map[string]any     `json:"body"`
	DryRun   bool               `json:"dryRun"`
	Applied  bool               `json:"applied"`
	Message  string             `json:"message,omitempty"`
	ReadBack *icu.SportSettings `json:"readBack,omitempty"`
}

func sportsUpdateCommand() *Command {
	return &Command{
		Name: "",
		Usage: "sports update <type|id> [--ftp WATTS] [--indoor-ftp WATTS] [--lthr BPM] [--max-hr BPM] " +
			"[--hr-zones B1,B2,...] [--power-zones P1,P2,...] [--dry-run]",
		Description: "Update sport settings. Any subset of fields; only changed fields are sent. " +
			"--hr-zones are zone upper bounds in bpm (bounded by the effective max HR), --power-zones are zone " +
			"upper bounds in % of FTP (999 = open top zone); both must be strictly ascending positive ints with the " +
			"expected zone count. Output shows a before/after diff and the read-back; --dry-run only previews.",
		Run: runSportsUpdate,
	}
}

func runSportsUpdate(args []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errMissing("sport type or id")
	}
	id := args[0]
	if !anySportFlag(flags) {
		return errors.New("sports update: no fields given (use --ftp, --indoor-ftp, --lthr, --max-hr, --hr-zones or --power-zones)")
	}

	var current icu.SportSettings
	if err := client.Get("sport-settings", []string{id}, nil, &current); err != nil {
		return wrapCommandError(err)
	}

	body, changes, err := buildSportsUpdate(flags, &current)
	if err != nil {
		return err
	}

	result := sportsUpdateResult{ID: id, Changes: changes, Body: body}
	if len(body) == 0 {
		result.Message = "no changes: values already match"

		return writeJSON(stdout, result)
	}
	if BoolFlag(flags, "dry-run") {
		result.DryRun = true
		result.Message = dryRunNothingWritten

		return writeJSON(stdout, result)
	}

	q := map[string]string{"recalcHrZones": "false"}
	if err := client.Put("sport-settings", []string{id}, q, body, nil); err != nil {
		return wrapCommandError(err)
	}
	result.Applied = true

	var after icu.SportSettings
	if err := client.Get("sport-settings", []string{id}, nil, &after); err != nil {
		return wrapCommandError(fmt.Errorf("write succeeded but read-back failed: %w", err))
	}
	result.ReadBack = &after
	if mismatch := readBackMismatch(body, &after); mismatch != "" {
		result.Message = "read-back mismatch: " + mismatch
		if err := writeJSON(stdout, result); err != nil {
			return err
		}

		return fmt.Errorf("sports update: read-back mismatch on %s", mismatch)
	}

	return writeJSON(stdout, result)
}

type sportIntFlag struct{ flag, field string }

func sportIntFlags() []sportIntFlag {
	return []sportIntFlag{
		{"ftp", wireFTP}, {"indoor-ftp", wireIndoorFTP}, {"lthr", wireLTHR}, {"max-hr", wireMaxHR},
	}
}

func anySportFlag(flags map[string]string) bool {
	for _, name := range []string{"ftp", "indoor-ftp", "lthr", "max-hr", "hr-zones", "power-zones"} {
		if _, ok := flags[name]; ok {
			return true
		}
	}

	return false
}

func sportIntValue(s *icu.SportSettings, field string) int {
	switch field {
	case wireFTP:
		return s.FTP
	case wireIndoorFTP:
		return s.IndoorFTP
	case wireLTHR:
		return s.LTHR
	default:
		return s.MaxHR
	}
}

func buildSportsUpdate(flags map[string]string, current *icu.SportSettings) (map[string]any, []sportFieldChange, error) {
	body := map[string]any{}
	changes := []sportFieldChange{}
	effectiveMaxHR := current.MaxHR

	for _, f := range sportIntFlags() {
		raw, ok := flags[f.flag]
		if !ok {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || v <= 0 {
			return nil, nil, fmt.Errorf("--%s must be a positive integer, got %q", f.flag, raw)
		}
		if f.field == wireMaxHR {
			effectiveMaxHR = v
		}
		if before := sportIntValue(current, f.field); before != v {
			body[f.field] = v
			changes = append(changes, sportFieldChange{Field: f.field, Before: before, After: v})
		}
	}

	type zoneSpec struct {
		flag, field, unit string
		maxValue          int
		current           []int
		names             []string
	}
	specs := []zoneSpec{
		{"hr-zones", wireHRZones, "bpm (max_hr)", effectiveMaxHR, current.HRZones, current.HRZoneNames},
		{"power-zones", wirePowerZones, "% of FTP", powerZoneOpenTop, current.PowerZones, current.PowerZoneNames},
	}
	for _, z := range specs {
		raw, ok := flags[z.flag]
		if !ok {
			continue
		}
		zones, err := parseZones(z.flag, raw, z.unit, z.maxValue)
		if err != nil {
			return nil, nil, err
		}
		if err := checkZoneCount(z.flag, len(zones), len(z.current), len(z.names)); err != nil {
			return nil, nil, err
		}
		if !intsEqual(zones, z.current) {
			body[z.field] = zones
			changes = append(changes, sportFieldChange{Field: z.field, Before: z.current, After: zones})
		}
	}

	return body, changes, nil
}

// checkZoneCount requires the new zone list to match the current zone count,
// or the configured zone-name count when no zones are set yet.
func checkZoneCount(flag string, got, currentCount, nameCount int) error {
	switch {
	case currentCount > 0:
		if got != currentCount {
			return fmt.Errorf("--%s: got %d zones, current settings have %d", flag, got, currentCount)
		}
	case nameCount > 0:
		if got != nameCount {
			return fmt.Errorf("--%s: got %d zones, settings define %d zone names", flag, got, nameCount)
		}
	default:
		return fmt.Errorf("--%s: current settings have no zones or zone names; cannot determine the expected zone count", flag)
	}

	return nil
}

// parseZones parses strictly ascending positive ints; maxValue <= 0 means no
// upper bound is known.
func parseZones(flag, raw, unit string, maxValue int) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("--%s: want comma-separated integers (%s)", flag, unit)
	}
	parts := strings.Split(raw, ",")
	zones := make([]int, 0, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("--%s: %q is not an integer", flag, p)
		}
		if v <= 0 {
			return nil, fmt.Errorf("--%s: %d must be positive", flag, v)
		}
		if maxValue > 0 && v > maxValue {
			return nil, fmt.Errorf("--%s: %d exceeds %d %s", flag, v, maxValue, unit)
		}
		if i > 0 && v <= zones[i-1] {
			return nil, fmt.Errorf("--%s: values must be strictly ascending (%d after %d)", flag, v, zones[i-1])
		}
		zones = append(zones, v)
	}

	return zones, nil
}

func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// readBackMismatch lists body fields whose read-back value differs, sorted.
func readBackMismatch(body map[string]any, after *icu.SportSettings) string {
	var bad []string
	for field, want := range body {
		switch w := want.(type) {
		case int:
			if sportIntValue(after, field) != w {
				bad = append(bad, field)
			}
		case []int:
			got := after.HRZones
			if field == wirePowerZones {
				got = after.PowerZones
			}
			if !intsEqual(got, w) {
				bad = append(bad, field)
			}
		default:
			bad = append(bad, field)
		}
	}
	sort.Strings(bad)

	return strings.Join(bad, ",")
}
