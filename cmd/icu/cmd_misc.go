package main

import icu "github.com/Thejuampi/icu"

func registerMiscCommands(registry *CommandRegistry) {
	registry.Register("fitness-events", "list", listAllCommand[[]icu.Event]("fitness-model-events", "fitness-events list", "List fitness model events (FITNESS_DAYS, SET_FITNESS, SET_EFTP)."))
	registry.Register("tags", "activities", listAllCommand[[]string]("activity-tags", "tags activities", "List activity tags."))
	registry.Register("tags", "events", listAllCommand[[]string]("event-tags", "tags events", "List event tags."))
	registry.Register("tags", "workouts", listAllCommand[[]string]("workout-tags", "tags workouts", "List workout tags."))
	registry.Register("shared-event", "get", getByIDCommand[any]("shared-event", "shared-event get <id>", "Get a shared event.", "shared icu.Event id", nil))
}
