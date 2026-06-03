package icu

import (
	"sort"
	"strings"
)

const BaseURL = "https://intervals.icu"

func BuildPath(athleteID, resource string, parts ...string) string {
	switch resource {
	case "activity":
		return buildActivityPath(athleteID, parts)
	case "shared-event":
		return buildSharedEventPath(athleteID, parts)
	case "download-workout":
		return buildDownloadWorkoutPath(parts)
	case "disconnect-app":
		return "/api/v1/disconnect-app"
	case "pace-distances":
		return "/api/v1/pace_distances"
	case "athlete-plans":
		return "/api/v1/athlete-plans"
	case "chats":
		return buildChatsPath(athleteID, parts)
	case "athlete":
		return buildAthletePath(athleteID, parts)
	case "activities", "events", "wellness", "wellness-bulk", "workouts", "folders", "gear",
		"routes", "sport-settings", "custom-item", "custom-item-indexes",
		"weather-config", "weather-forecast", "fitness-model-events", "training-plan",
		"apply-plan-changes", "athlete-summary", "profile", "settings",
		"power-curves", "hr-curves", "pace-curves", "mmp-model", "power-hr-curve",
		"activity-power-curves", "activity-hr-curves", "activity-pace-curves",
		"activity-tags", "event-tags", "workout-tags":
		return buildAthleteResourcePath(athleteID, resource, parts)
	default:
		return buildDefaultPath(resource, parts)
	}
}

func buildActivityPath(athleteID string, parts []string) string {
	if len(parts) == 0 {
		return "/api/v1/activity/" + athleteID
	}

	return "/api/v1/activity/" + strings.Join(parts, "/")
}

func buildSharedEventPath(athleteID string, parts []string) string {
	if len(parts) > 0 {
		return "/api/v1/shared-event/" + parts[0]
	}

	return "/api/v1/shared-event/" + athleteID
}

func buildDownloadWorkoutPath(parts []string) string {
	if len(parts) > 0 {
		return "/api/v1/download-workout" + parts[0]
	}

	return "/api/v1/download-workout"
}

func buildChatsPath(athleteID string, parts []string) string {
	if len(parts) == 0 {
		return "/api/v1/athlete/" + athleteID + "/chats"
	}

	if parts[0] == "send-message" {
		return "/api/v1/chats/send-message"
	}

	return "/api/v1/athlete/" + athleteID + "/chats/" + strings.Join(parts, "/")
}

func buildAthletePath(athleteID string, parts []string) string {
	if len(parts) == 0 {
		return "/api/v1/athlete/" + athleteID
	}

	return "/api/v1/athlete/" + athleteID + "/" + strings.Join(parts, "/")
}

func buildAthleteResourcePath(athleteID, resource string, parts []string) string {
	if len(parts) == 0 {
		return "/api/v1/athlete/" + athleteID + "/" + resource
	}

	return "/api/v1/athlete/" + athleteID + "/" + resource + "/" + strings.Join(parts, "/")
}

func buildDefaultPath(resource string, parts []string) string {
	if len(parts) == 0 {
		return "/api/v1/" + resource
	}

	all := append([]string{resource}, parts...)

	return "/api/v1/" + strings.Join(all, "/")
}

func BuildURL(path string, query map[string]string) string {
	if len(query) == 0 {
		return BaseURL + path
	}

	var builder strings.Builder

	builder.Grow(len(BaseURL) + len(path) + 1 + queryEncodedLength(query))
	builder.WriteString(BaseURL)
	builder.WriteString(path)
	builder.WriteByte('?')
	writeEncodedQuery(&builder, query)

	return builder.String()
}

const stackQueryKeys = 8

const queryPairSeparatorLength = 2

const upperHex = "0123456789ABCDEF"

func queryEncodedLength(query map[string]string) int {
	var length int

	for key, value := range query {
		length += queryComponentEncodedLength(key) + queryComponentEncodedLength(value) + queryPairSeparatorLength
	}

	return length
}

func queryComponentEncodedLength(value string) int {
	var length int

	for index := range len(value) {
		if isQuerySafe(value[index]) || value[index] == ' ' {
			length++
		} else {
			length += 3
		}
	}

	return length
}

func writeEncodedQuery(builder *strings.Builder, query map[string]string) {
	var stackKeys [stackQueryKeys]string
	keys := stackKeys[:0]

	if len(query) > len(stackKeys) {
		keys = make([]string, 0, len(query))
	}

	for key := range query {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for index, key := range keys {
		if index > 0 {
			builder.WriteByte('&')
		}

		writeQueryComponent(builder, key)
		builder.WriteByte('=')
		writeQueryComponent(builder, query[key])
	}
}

func writeQueryComponent(builder *strings.Builder, value string) {
	for index := range len(value) {
		current := value[index]

		switch {
		case isQuerySafe(current):
			builder.WriteByte(current)
		case current == ' ':
			builder.WriteByte('+')
		default:
			builder.WriteByte('%')
			builder.WriteByte(upperHex[current>>4])
			builder.WriteByte(upperHex[current&0x0f])
		}
	}
}

func isQuerySafe(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		value == '-' || value == '_' || value == '.' || value == '~'
}
