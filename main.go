package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	resource := os.Args[1]
	action := "show"
	args := os.Args[2:]

	idFirst := isIDFirstResource(resource)
	if idFirst && len(args) >= 2 && !strings.HasPrefix(args[0], "--") && !strings.HasPrefix(args[1], "--") {
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

	apiKey := ResolveAPIKey(flags)
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

	athleteID := ResolveAthleteID(flags)
	client := NewClient(apiKey, athleteID)

	if err := cmd.Run(posArgs, flags, client); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) map[string]string {
	flags := map[string]string{}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--"):
			name := strings.TrimPrefix(arg, "--")
			i = parseLongFlag(flags, args, i, name)
		case strings.HasPrefix(arg, "-") && len(arg) == 2:
			name := arg[1:]
			i = parseShortFlag(flags, args, i, name)
		default:
			positional = append(positional, arg)
		}
	}
	flags["_posargs_"] = strings.Join(positional, " ")
	return flags
}

func parseLongFlag(flags map[string]string, args []string, i int, name string) int {
	switch {
	case strings.Contains(name, "="):
		parts := strings.SplitN(name, "=", 2)
		flags[parts[0]] = parts[1]
	case i+1 < len(args) && !strings.HasPrefix(args[i+1], "--"):
		flags[name] = args[i+1]
		return i + 1
	default:
		flags[name] = "true"
	}
	return i
}

func parseShortFlag(flags map[string]string, args []string, i int, name string) int {
	switch {
	case i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
		flags[name] = args[i+1]
		return i + 1
	default:
		flags[name] = "true"
	}
	return i
}

func printHelp() {
	fmt.Println("icu - Intervals.icu CLI")
	fmt.Println()
	fmt.Println("Usage: icu <resource> <action> [flags]")
	fmt.Println()
	fmt.Println("Global flags:")
	fmt.Println("  --api-key KEY     API key from intervals.icu/settings (or INTERVALS_ICU_API_KEY env)")
	fmt.Println("  --athlete-id ID   Athlete ID (default: 0 for self, or INTERVALS_ICU_ATHLETE_ID env)")
	fmt.Println("  --output FORMAT   Output format: json (default), csv, table")
	fmt.Println()
	fmt.Println("Resources:")

	resources := make([]string, 0, len(commands))
	for k := range commands {
		resources = append(resources, k)
	}
	sort.Strings(resources)

	for _, r := range resources {
		acts := ActionsForResource(r)
		sort.Strings(acts)
		fmt.Printf("  %-15s  %s\n", r, strings.Join(acts, ", "))
	}
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  icu athlete show")
	fmt.Println("  icu activities list --oldest 2026-05-20 --newest 2026-05-24")
	fmt.Println("  icu wellness get 2026-05-24")
	fmt.Println("  icu ftp show")
}
