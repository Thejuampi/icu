package main

import icu "github.com/Thejuampi/icu"

type Command struct {
	Name        string
	Usage       string
	Description string
	Run         func(args []string, flags map[string]string, client *icu.Client) error
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

	registry.commands[resource][action] = cmd
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
