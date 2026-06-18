package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	icu "github.com/Thejuampi/icu"
)

const defaultAction = "show"

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	const minArgs = 2

	registry := NewCommandRegistry()
	registerAllCommands(registry)

	if len(args) >= minArgs && isHelpArg(args[1]) {
		printHelp(registry, stdout, "")

		return 0
	}

	if len(args) < minArgs {
		printHelp(registry, stdout, "")

		return 1
	}

	resource := args[1]
	rest := args[2:]

	action, posArgs, flags, dispatch := runDispatch(registry, resource, rest, stdout)
	if dispatch {
		return executeCommand(registry, resource, action, posArgs, flags, stdout, stderr)
	}

	return 0
}

//nolint:gocritic // unnamed results are clearer in dispatch functions
func runDispatch(
	registry *CommandRegistry,
	resource string,
	rest []string,
	stdout io.Writer,
) (string, []string, map[string]string, bool) {
	if action, pos, fl, ok := tryIDFirst(registry, resource, rest, stdout); ok {
		return action, pos, fl, true
	}

	if indexOfHelp(rest) >= 0 {
		printHelp(registry, stdout, resource)

		return "", nil, nil, false
	}

	action := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "--") {
		action = rest[0]
		rest = rest[1:]
	}

	if action == "" {
		action = resolveDefault(registry, resource, stdout)
		if action == "" {
			return "", nil, nil, false
		}
	}

	flags := parseFlags(rest)

	var posArgs []string
	if pa, ok := flags["_posargs_"]; ok && pa != "" {
		posArgs = strings.Fields(pa)
	}

	delete(flags, "_posargs_")

	return action, posArgs, flags, true
}

//nolint:gocritic
func tryIDFirst(
	registry *CommandRegistry,
	resource string,
	rest []string,
	stdout io.Writer,
) (string, []string, map[string]string, bool) {
	const minArgs = 2

	if !isIDFirstResource(resource) || len(rest) < minArgs || strings.HasPrefix(rest[0], "--") || strings.HasPrefix(rest[1], "--") {
		return "", nil, nil, false
	}

	if indexOfHelp(rest[2:]) >= 0 {
		printHelp(registry, stdout, resource)

		return "", nil, nil, true
	}

	flags := parseFlags(rest[2:])

	var posArgs []string
	if pa, ok := flags["_posargs_"]; ok && pa != "" {
		posArgs = strings.Fields(pa)
	}

	delete(flags, "_posargs_")

	action := rest[1]
	posArgs = append([]string{rest[0]}, posArgs...)

	return action, posArgs, flags, true
}

func resolveDefault(registry *CommandRegistry, resource string, stdout io.Writer) string {
	acts := registry.Actions(resource)
	if acts == nil {
		return defaultAction
	}

	if !hasAction(acts, defaultAction) {
		printHelp(registry, stdout, resource)

		return ""
	}

	return defaultAction
}

func hasAction(acts []string, target string) bool {
	for _, a := range acts {
		if a == target {
			return true
		}
	}

	return false
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "--help" || arg == "-h"
}

func indexOfHelp(args []string) int {
	for i, a := range args {
		if isHelpArg(a) {
			return i
		}
	}

	return -1
}

func isIDFirstResource(resource string) bool {
	switch resource {
	case "activity", "shared-event":
		return true
	}

	return false
}

func commandRequiresAuth(resource, _ string) bool {
	if resource == "config" || resource == "zepp" {
		return false
	}

	return true
}

func executeCommand(
	registry *CommandRegistry,
	resource, action string,
	posArgs []string,
	flags map[string]string,
	_, stderr io.Writer,
) int {
	cmd, ok := registry.Lookup(resource, action)
	if !ok {
		fmt.Fprintf(stderr, "Unknown command: %s %s\n", resource, action)
		fmt.Fprintf(stderr, "Run 'icu help' for available commands.\n")

		return 1
	}

	var apiKey string
	if commandRequiresAuth(resource, action) {
		apiKey = icu.ResolveAPIKey(flags)
		if apiKey == "" {
			fmt.Fprintln(stderr, "Error: API key required. Set INTERVALS_ICU_API_KEY or use --api-key.")

			return 1
		}
	}

	athleteID := icu.ResolveAthleteID(flags)
	client := icu.NewClient(apiKey, athleteID)

	if err := cmd.Run(posArgs, flags, client); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)

		return 1
	}

	return 0
}

func parseFlags(args []string) map[string]string {
	flags := map[string]string{}

	var positional []string

	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]

		switch {
		case strings.HasPrefix(arg, "--"):
			name := strings.TrimPrefix(arg, "--")
			idx = parseLongFlag(flags, args, idx, name)
		case strings.HasPrefix(arg, "-") && len(arg) == 2:
			name := arg[1:]
			idx = parseShortFlag(flags, args, idx, name)
		default:
			positional = append(positional, arg)
		}
	}

	flags["_posargs_"] = strings.Join(positional, " ")

	return flags
}

const splitCount = 2

func parseLongFlag(flags map[string]string, args []string, idx int, name string) int {
	key := strings.ReplaceAll(name, "_", "-")

	switch {
	case strings.Contains(name, "="):
		parts := strings.SplitN(name, "=", splitCount)
		flags[strings.ReplaceAll(parts[0], "_", "-")] = parts[1]
	case idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "--"):
		flags[key] = args[idx+1]

		return idx + 1
	default:
		flags[key] = ""
	}

	return idx
}

func parseShortFlag(flags map[string]string, args []string, idx int, name string) int {
	switch {
	case idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-"):
		flags[name] = args[idx+1]

		return idx + 1
	default:
		flags[name] = ""
	}

	return idx
}

func printHelp(registry *CommandRegistry, w io.Writer, resource string) {
	if resource != "" {
		acts := registry.Actions(resource)
		if acts == nil {
			resource = ""
		}
	}

	if resource == "" {
		printGlobalHelp(registry, w)

		return
	}

	printResourceHelp(registry, w, resource)
}

func printGlobalHelp(registry *CommandRegistry, w io.Writer) {
	fmt.Fprintln(w, "icu - Intervals.icu CLI")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: icu <resource> <action> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Global flags:")
	fmt.Fprintln(w, "  --api-key KEY     API key from intervals.icu/settings (or INTERVALS_ICU_API_KEY env)")
	fmt.Fprintln(w, "  --athlete-id ID   Athlete ID (default: 0 for self, or INTERVALS_ICU_ATHLETE_ID env)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resources:")

	resources := registry.Resources()

	sort.Strings(resources)

	for _, r := range resources {
		acts := registry.Actions(r)
		sort.Strings(acts)
		fmt.Fprintf(w, "  %-15s  %s\n", r, strings.Join(acts, ", "))
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  icu athlete show")
	fmt.Fprintln(w, "  icu activities list --oldest 2026-05-20 --newest 2026-05-24")
	fmt.Fprintln(w, "  icu wellness get 2026-05-24")
	fmt.Fprintln(w, "  icu ftp show")
}

func printResourceHelp(registry *CommandRegistry, w io.Writer, resource string) {
	acts := registry.Actions(resource)
	sort.Strings(acts)

	fmt.Fprintf(w, "Commands for %s:\n\n", resource)

	for _, action := range acts {
		cmd, ok := registry.Lookup(resource, action)
		if !ok {
			continue
		}

		if cmd.Usage != "" {
			fmt.Fprintf(w, "  icu %s\n", cmd.Usage)
		} else {
			fmt.Fprintf(w, "  icu %s %s\n", resource, action)
		}

		if cmd.Description != "" {
			fmt.Fprintf(w, "      %s\n", cmd.Description)
		}

		fmt.Fprintln(w)
	}
}
