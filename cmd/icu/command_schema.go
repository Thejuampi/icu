package main

import (
	"fmt"
	"sort"
	"strconv"
)

type CommandFlagKind int

const (
	commandFlagString CommandFlagKind = iota
	commandFlagPositiveInteger
	commandFlagBoolean
)

type CommandFlag struct {
	Name        string
	ValueName   string
	Description string
	Default     string
	Kind        CommandFlagKind
}

type CommandSchema struct {
	Flags             []CommandFlag
	RejectPositionals bool
}

func validateCommandInput(command *Command, args []string, flags map[string]string) error {
	if command.Schema == nil {
		return nil
	}

	if command.Schema.RejectPositionals && len(args) > 0 {
		return fmt.Errorf("unexpected positional argument %q", args[0])
	}

	allowed := make(map[string]CommandFlag, len(command.Schema.Flags))
	for _, flag := range command.Schema.Flags {
		allowed[flag.Name] = flag
	}

	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		flag, ok := allowed[name]
		if !ok {
			return fmt.Errorf("unknown flag --%s", name)
		}
		if err := validateCommandFlag(flag, flags[name]); err != nil {
			return err
		}
	}

	if command.Validate != nil {
		return command.Validate(flags)
	}

	return nil
}

func validateCommandFlag(flag CommandFlag, value string) error {
	switch flag.Kind {
	case commandFlagString:
		if value == "" {
			return fmt.Errorf("--%s requires a value", flag.Name)
		}

		return nil
	case commandFlagBoolean:
		if value == "" || value == "true" || value == "false" || value == "1" || value == "0" {
			return nil
		}

		return fmt.Errorf("--%s must be a boolean (bare, true, false, 1, or 0)", flag.Name)
	case commandFlagPositiveInteger:
		if value == "" {
			return fmt.Errorf("--%s requires a value", flag.Name)
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("--%s must be an integer: %w", flag.Name, err)
		}
		if parsed <= 0 {
			return fmt.Errorf("--%s must be greater than 0", flag.Name)
		}

		return nil
	}

	return fmt.Errorf("--%s has an unsupported flag kind", flag.Name)
}
