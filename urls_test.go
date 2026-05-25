package main

import "testing"

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name      string
		athleteID string
		resource  string
		parts     []string
		want      string
	}{
		{"athlete self", "0", "athlete", nil, "/api/v1/athlete/0"},
		{"athlete specific", "i445643", "athlete", nil, "/api/v1/athlete/i445643"},
		{"activities with query", "0", "activities", nil, "/api/v1/athlete/0/activities"},
		{"single activity", "0", "activity", []string{"i123"}, "/api/v1/activity/i123"},
		{"activity intervals", "0", "activity", []string{"i123", "intervals"}, "/api/v1/activity/i123/intervals"},
		{"wellness date", "0", "wellness", []string{"2026-05-24"}, "/api/v1/athlete/0/wellness/2026-05-24"},
		{"event by id", "0", "events", []string{"456"}, "/api/v1/athlete/0/events/456"},
		{"sport settings by type", "0", "sport-settings", []string{"Ride"}, "/api/v1/athlete/0/sport-settings/Ride"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPath(tt.athleteID, tt.resource, tt.parts...)
			if got != tt.want {
				t.Errorf("BuildPath(%q, %q, %v) = %q, want %q",
					tt.athleteID, tt.resource, tt.parts, got, tt.want)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		query map[string]string
		want  string
	}{
		{"no query", "/api/v1/athlete/0", nil, "https://intervals.icu/api/v1/athlete/0"},
		{"empty query", "/api/v1/athlete/0", map[string]string{}, "https://intervals.icu/api/v1/athlete/0"},
		{"single param", "/api/v1/athlete/0/activities", map[string]string{"oldest": "2026-05-20"}, "https://intervals.icu/api/v1/athlete/0/activities?oldest=2026-05-20"},
		{"multiple params", "/api/v1/athlete/0/activities", map[string]string{"oldest": "2026-05-20", "newest": "2026-05-24"}, "https://intervals.icu/api/v1/athlete/0/activities?newest=2026-05-24&oldest=2026-05-20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildURL(tt.path, tt.query)
			if got != tt.want {
				t.Errorf("BuildURL(%q, %v) = %q, want %q", tt.path, tt.query, got, tt.want)
			}
		})
	}
}
