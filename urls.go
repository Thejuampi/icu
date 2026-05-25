package main

import (
	"net/url"
	"strings"
)

const BaseURL = "https://intervals.icu"

//nolint:gocyclo,cyclop
func BuildPath(athleteID, resource string, parts ...string) string {
	switch resource {
	case "activity":
		if len(parts) == 0 {
			return "/api/v1/activity/" + athleteID
		}

		return "/api/v1/activity/" + strings.Join(parts, "/")
	case "shared-event":
		if len(parts) > 0 {
			return "/api/v1/shared-event/" + parts[0]
		}

		return "/api/v1/shared-event/" + athleteID
	case "download-workout":
		if len(parts) > 0 {
			return "/api/v1/download-workout" + parts[0]
		}

		return "/api/v1/download-workout"
	case "disconnect-app":
		return "/api/v1/disconnect-app"
	case "pace-distances":
		return "/api/v1/pace_distances"
	case "athlete-plans":
		return "/api/v1/athlete-plans"
	case "chats":
		if len(parts) == 0 {
			return "/api/v1/athlete/" + athleteID + "/chats"
		}

		if parts[0] == "send-message" {
			return "/api/v1/chats/send-message"
		}

		return "/api/v1/athlete/" + athleteID + "/chats/" + strings.Join(parts, "/")
	case "athlete":
		if len(parts) == 0 {
			return "/api/v1/athlete/" + athleteID
		}

		return "/api/v1/athlete/" + athleteID + "/" + strings.Join(parts, "/")
	case "activities", "events", "wellness", "wellness-bulk", "workouts", "folders", "gear",
		"routes", "sport-settings", "custom-item", "custom-item-indexes",
		"weather-config", "weather-forecast", "fitness-model-events", "training-plan",
		"apply-plan-changes", "athlete-summary", "profile", "settings",
		"power-curves", "hr-curves", "pace-curves", "mmp-model", "power-hr-curve",
		"activity-power-curves", "activity-hr-curves", "activity-pace-curves",
		"activity-tags", "event-tags", "workout-tags":
		if len(parts) == 0 {
			return "/api/v1/athlete/" + athleteID + "/" + resource
		}

		return "/api/v1/athlete/" + athleteID + "/" + resource + "/" + strings.Join(parts, "/")
	default:
		if len(parts) == 0 {
			return "/api/v1/" + resource
		}

		all := append([]string{resource}, parts...)

		return "/api/v1/" + strings.Join(all, "/")
	}
}

func BuildURL(path string, query map[string]string) string {
	if len(query) == 0 {
		return BaseURL + path
	}

	values := url.Values{}
	for k, v := range query {
		values.Set(k, v)
	}

	return BaseURL + path + "?" + values.Encode()
}
