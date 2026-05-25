package main

import (
	"fmt"
	"net/url"
	"strings"
)

const BaseURL = "https://intervals.icu"

func BuildPath(athleteID, resource string, parts ...string) string {
	switch resource {
	case "activity":
		if len(parts) == 0 {
			return fmt.Sprintf("/api/v1/activity/%s", athleteID)
		}
		return fmt.Sprintf("/api/v1/activity/%s", strings.Join(parts, "/"))
	case "shared-event":
		if len(parts) > 0 {
			return fmt.Sprintf("/api/v1/shared-event/%s", parts[0])
		}
		return fmt.Sprintf("/api/v1/shared-event/%s", athleteID)
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
			return fmt.Sprintf("/api/v1/athlete/%s/chats", athleteID)
		}
		if parts[0] == "send-message" {
			return "/api/v1/chats/send-message"
		}
		return fmt.Sprintf("/api/v1/athlete/%s/chats/%s", athleteID, strings.Join(parts, "/"))
	case "athlete":
		if len(parts) == 0 {
			return fmt.Sprintf("/api/v1/athlete/%s", athleteID)
		}
		return fmt.Sprintf("/api/v1/athlete/%s/%s", athleteID, strings.Join(parts, "/"))
	case "activities", "events", "wellness", "wellness-bulk", "workouts", "folders", "gear",
		"routes", "sport-settings", "custom-item", "custom-item-indexes",
		"weather-config", "weather-forecast", "fitness-model-events", "training-plan",
		"apply-plan-changes", "athlete-summary", "profile", "settings",
		"power-curves", "hr-curves", "pace-curves", "mmp-model", "power-hr-curve",
		"activity-power-curves", "activity-hr-curves", "activity-pace-curves",
		"activity-tags", "event-tags", "workout-tags":
		if len(parts) == 0 {
			return fmt.Sprintf("/api/v1/athlete/%s/%s", athleteID, resource)
		}
		return fmt.Sprintf("/api/v1/athlete/%s/%s/%s", athleteID, resource, strings.Join(parts, "/"))
	default:
		if len(parts) == 0 {
			return fmt.Sprintf("/api/v1/%s", resource)
		}
		all := append([]string{resource}, parts...)
		return fmt.Sprintf("/api/v1/%s", strings.Join(all, "/"))
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
