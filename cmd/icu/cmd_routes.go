package main

import icu "github.com/Thejuampi/icu"

func registerRoutesCommands(registry *CommandRegistry) {
	registry.Register("routes", "list", listAllCommand[any]("routes", "routes list", "List routes with activity counts."))
	registry.Register("routes", "get", getByIDCommand[icu.Route]("routes", "routes get <id> [--include-path]", "Get route by ID.", "icu.Route id", routesQuery))
}

func routesQuery(flags map[string]string) map[string]string {
	q := map[string]string{}
	if BoolFlag(flags, "include-path") {
		q["includePath"] = strTrue
	}

	return q
}
