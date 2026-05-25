package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	icu "github.com/Thejuampi/icu"
)

func main() {
	const minArgs = 2

	if len(os.Args) < minArgs {
		printHelp()
		os.Exit(1)
	}

	resource := os.Args[1]
	action := "show"
	args := os.Args[2:]

	idFirst := isIDFirstResource(resource)
	if idFirst && len(args) >= minArgs && !strings.HasPrefix(args[0], "--") && !strings.HasPrefix(args[1], "--") {
		flags := parseFlags(args[2:])

		var posArgs []string
		if pa, ok := flags["_posargs_"]; ok && pa != "" {
			posArgs = strings.Fields(pa)
		}

		delete(flags, "_posargs_")

		action = args[1]
		posArgs = append([]string{args[0]}, posArgs...)
		executeCommand(resource, action, posArgs, flags)

		return
	}

	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		action = args[0]
		args = args[1:]
	}

	flags := parseFlags(args)

	var posArgs []string
	if pa, ok := flags["_posargs_"]; ok && pa != "" {
		posArgs = strings.Fields(pa)
	}

	delete(flags, "_posargs_")
	executeCommand(resource, action, posArgs, flags)
}

func isIDFirstResource(resource string) bool {
	switch resource {
	case "activity", "shared-event":
		return true
	}

	return false
}

func executeCommand(resource, action string, posArgs []string, flags map[string]string) {
	if _, ok := flags["help"]; ok || resource == "help" {
		printHelp()

		return
	}

	apiKey := icu.ResolveAPIKey(flags)
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key required. Set INTERVALS_ICU_API_KEY or use --api-key.")
		os.Exit(1)
	}

	cmd, ok := LookupCommand(resource, action)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s %s\n", resource, action)
		fmt.Fprintf(os.Stderr, "Run 'icu help' for available commands.\n")
		os.Exit(1)
	}

	athleteID := icu.ResolveAthleteID(flags)
	client := icu.NewClient(apiKey, athleteID)

	if err := cmd.Run(posArgs, flags, client); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
	switch {
	case strings.Contains(name, "="):
		parts := strings.SplitN(name, "=", splitCount)
		flags[parts[0]] = parts[1]
	case idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "--"):
		flags[name] = args[idx+1]

		return idx + 1
	default:
		flags[name] = strTrue
	}

	return idx
}

func parseShortFlag(flags map[string]string, args []string, idx int, name string) int {
	switch {
	case idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-"):
		flags[name] = args[idx+1]

		return idx + 1
	default:
		flags[name] = strTrue
	}

	return idx
}

func printHelp() {
	fmt.Fprintln(os.Stdout, "icu - Intervals.icu CLI")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Usage: icu <resource> <action> [flags]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Global flags:")
	fmt.Fprintln(os.Stdout, "  --api-key KEY     API key from intervals.icu/settings (or INTERVALS_ICU_API_KEY env)")
	fmt.Fprintln(os.Stdout, "  --athlete-id ID   Athlete ID (default: 0 for self, or INTERVALS_ICU_ATHLETE_ID env)")
	fmt.Fprintln(os.Stdout, "  --output FORMAT   Output format: json (default), csv, table")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Resources:")

	resources := make([]string, 0, len(commands))
	for k := range commands {
		resources = append(resources, k)
	}

	sort.Strings(resources)

	for _, r := range resources {
		acts := ActionsForResource(r)
		sort.Strings(acts)
		fmt.Fprintf(os.Stdout, "  %-15s  %s\n", r, strings.Join(acts, ", "))
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Examples:")
	fmt.Fprintln(os.Stdout, "  icu athlete show")
	fmt.Fprintln(os.Stdout, "  icu activities list --oldest 2026-05-20 --newest 2026-05-24")
	fmt.Fprintln(os.Stdout, "  icu wellness get 2026-05-24")
	fmt.Fprintln(os.Stdout, "  icu ftp show")
}
