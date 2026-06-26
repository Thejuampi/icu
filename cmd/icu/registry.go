package main

func registerAllCommands(registry *CommandRegistry) {
	registerAnalysisCommands(registry)
	registerActivitiesCommands(registry)
	registerActivityCommands(registry)
	registerAthleteCommands(registry)
	registerChatsCommands(registry)
	registerConfigCommands(registry)
	registerCurvesCommands(registry)
	registerCustomCommands(registry)
	registerEventsCommands(registry)
	registerFoldersCommands(registry)
	registerFTPCommands(registry)
	registerGearCommands(registry)
	registerMiscCommands(registry)
	registerRebalanceCommands(registry)
	registerRoutesCommands(registry)
	registerSportsCommands(registry)
	registerWeatherCommands(registry)
	registerWellnessCommands(registry)
	registerWorkoutsCommands(registry)
	registerZeppCommands(registry)
}
