package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	icu "github.com/Thejuampi/icu"
)

const (
	// maxHRZoneBPM bounds HR zone upper limits (bpm).
	maxHRZoneBPM = 250
	// maxPowerZonePct bounds power zone upper limits (% of FTP); intervals.icu
	// uses 999 as the open-ended top-zone sentinel.
	maxPowerZonePct = 999
	maxZoneCount    = 12
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
			"[--hr-zones B1,B2,...] [--power-zones P1,P2,...] [--dry-run] [--yes]",
		Description: "Update sport settings. Any subset of fields; only changed fields are sent. " +
			"--hr-zones are zone upper bounds in bpm, --power-zones are zone upper bounds in % of FTP; both must be " +
			"strictly ascending positive ints with the same count as the current zones. Prints a preview diff; " +
			"writes only with --yes, then reads the settings back.",
		Run: runSportsUpdate,
	}
}

func runSportsUpdate(args []string, flags map[string]string, client *icu.Client) error {
	if len(args) == 0 {
		return errMissing("sport type or id")
	}
	id := args[0]

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
		if len(changes) == 0 && !anySportFlag(flags) {
			return errors.New("sports update: no fields given (use --ftp, --indoor-ftp, --lthr, --max-hr, --hr-zones or --power-zones)")
		}
		result.Message = "no changes: values already match"

		return writeJSON(result)
	}

	if BoolFlag(flags, "dry-run") || !BoolFlag(flags, "yes") {
		result.DryRun = true
		result.Message = "preview only; re-run with --yes to write"

		return writeJSON(result)
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
	}

	return writeJSON(result)
}

func sportIntFlags() []struct{ flag, field string } {
	return []struct{ flag, field string }{
		{"ftp", "ftp"}, {"indoor-ftp", "indoorFtp"}, {"lthr", "lthr"}, {"max-hr", "maxHr"},
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
	case "ftp":
		return s.FTP
	case "indoorFtp":
		return s.IndoorFTP
	case "lthr":
		return s.LTHR
	default:
		return s.MaxHR
	}
}

func buildSportsUpdate(flags map[string]string, current *icu.SportSettings) (map[string]any, []sportFieldChange, error) {
	body := map[string]any{}
	changes := []sportFieldChange{}

	for _, f := range sportIntFlags() {
		raw, ok := flags[f.flag]
		if !ok {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || v <= 0 {
			return nil, nil, fmt.Errorf("--%s must be a positive integer, got %q", f.flag, raw)
		}
		if before := sportIntValue(current, f.field); before != v {
			body[f.field] = v
			changes = append(changes, sportFieldChange{Field: f.field, Before: before, After: v})
		}
	}

	zoneFlags := []struct {
		flag, field, unit string
		max               int
		current           []int
	}{
		{"hr-zones", "hrZones", "bpm", maxHRZoneBPM, current.HRZones},
		{"power-zones", "powerZones", "% of FTP", maxPowerZonePct, current.PowerZones},
	}
	for _, z := range zoneFlags {
		raw, ok := flags[z.flag]
		if !ok {
			continue
		}
		zones, err := parseZones(z.flag, raw, z.unit, z.max)
		if err != nil {
			return nil, nil, err
		}
		if len(z.current) > 0 && len(zones) != len(z.current) {
			return nil, nil, fmt.Errorf("--%s: got %d zones, current settings have %d", z.flag, len(zones), len(z.current))
		}
		if !intsEqual(zones, z.current) {
			body[z.field] = zones
			changes = append(changes, sportFieldChange{Field: z.field, Before: z.current, After: zones})
		}
	}

	return body, changes, nil
}

func parseZones(flag, raw, unit string, maxValue int) ([]int, error) {
	parts := strings.Split(raw, ",")
	if strings.TrimSpace(raw) == "" || len(parts) > maxZoneCount {
		return nil, fmt.Errorf("--%s: want 1-%d comma-separated integers (%s)", flag, maxZoneCount, unit)
	}
	zones := make([]int, 0, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("--%s: %q is not an integer", flag, p)
		}
		if v <= 0 || v > maxValue {
			return nil, fmt.Errorf("--%s: %d out of range 1-%d %s", flag, v, maxValue, unit)
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
			if field == "powerZones" {
				got = after.PowerZones
			}
			if !intsEqual(got, w) {
				bad = append(bad, field)
			}
		}
	}

	return strings.Join(bad, ",")
}
