package main

type Command struct {
	Name        string
	Usage       string
	Description string
	Run         func(args []string, flags map[string]string, client *Client) error
}

var commands = map[string]map[string]*Command{}

func RegisterCommand(resource, action string, cmd *Command) {
	if commands[resource] == nil {
		commands[resource] = map[string]*Command{}
	}
	commands[resource][action] = cmd
}

func LookupCommand(resource, action string) (*Command, bool) {
	if r, ok := commands[resource]; ok {
		if c, ok := r[action]; ok {
			return c, true
		}
	}
	return nil, false
}

func AllResources() []string {
	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}
	return keys
}

func ActionsForResource(resource string) []string {
	if r, ok := commands[resource]; ok {
		var acts []string
		for k := range r {
			acts = append(acts, k)
		}
		return acts
	}
	return nil
}
