package main

import (
	"io"

	icu "github.com/Thejuampi/icu"
)

type Command struct {
	Name        string
	Usage       string
	Description string
	Schema      *CommandSchema
	Validate    func(flags map[string]string) error
	Run         func(args []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error
}

type CommandRegistry struct {
	commands map[string]map[string]*Command
}

func NewCommandRegistry() *CommandRegistry {
	registry := CommandRegistry{commands: map[string]map[string]*Command{}}

	return &registry
}

func (registry *CommandRegistry) Register(resource, action string, cmd *Command) {
	if registry.commands[resource] == nil {
		registry.commands[resource] = map[string]*Command{}
	}

	if extra := undocumentedUsageFlags(resource + " " + action); extra != "" {
		cmd.Usage += " " + extra
	}
	registry.commands[resource][action] = cmd
}

// undocumentedUsageFlags lists flags these commands read (and document in
// docs/cli-reference.md) that their Usage line was missing. Usage is the
// allow-list for unknown-flag rejection, so they are appended here.
// decouplingUsageFlags are the decoupling flags the analysis commands pass
// to enrichActivitiesDecoupling.
const decouplingUsageFlags = "[--decoupling-source api|streams] [--decoupling-warmup-min N] " +
	"[--decoupling-cooldown-min N] [--decoupling-hr-lag S] [--decoupling-iqr-k K]"

func undocumentedUsageFlags(key string) string {
	return map[string]string{
		"activities interval-search":       "[--minReps N] [--maxReps N] [--limit N]",
		"activities list":                  "[--route-id ID]",
		"activities upload":                "[--device-name NAME] [--paired-event-id ID]",
		"activity best-efforts":            "[--stream NAME] [--duration SECS] [--distance M] [--count N]",
		"activity estimate-power-backtest": "[--cda N] [--air-density N]",
		"activity map":                     "[--bounds BOOL] [--weather BOOL]",
		"activity weather-summary":         "[--descr-config VALUE]",
		"analysis adaptation":              "[--filters FILTERS] [--limit N] [--activity-fields CSV] [--timezone TZ]",
		"analysis cycling":                 "[--fields CSV] [--limit N] [--timezone TZ] " + decouplingUsageFlags,
		"analysis wellness":                "[--timezone TZ]",
		"analysis micro":                   "[--limit N] [--calendar-id ID] [--resolve] " + decouplingUsageFlags,
		"analysis microcycle":              "[--limit N] [--calendar-id ID] [--resolve] " + decouplingUsageFlags,
		"analysis plan": "[--history-days N] [--plan-days N] [--sport-type TYPE] [--calendar-id ID] [--resolve] " +
			"[--activity-fields CSV] [--limit N] [--timezone TZ] " + decouplingUsageFlags,
		"curves hr":          "[--curves CURVES]",
		"curves pace":        "[--curves CURVES]",
		"curves power":       "[--newest DATE] [--filters FILTERS]",
		"events list":        "[--limit N] [--calendar-id ID]",
		"workouts calculate": "[--description DESC]",
	}[key]
}

func (registry *CommandRegistry) Lookup(resource, action string) (*Command, bool) {
	if resourceCommands, ok := registry.commands[resource]; ok {
		if command, ok := resourceCommands[action]; ok {
			return command, true
		}
	}

	return nil, false
}

func (registry *CommandRegistry) Resources() []string {
	keys := make([]string, 0, len(registry.commands))
	for key := range registry.commands {
		keys = append(keys, key)
	}

	return keys
}

func (registry *CommandRegistry) Actions(resource string) []string {
	if resourceCommands, ok := registry.commands[resource]; ok {
		var actions []string
		for action := range resourceCommands {
			actions = append(actions, action)
		}

		return actions
	}

	return nil
}
