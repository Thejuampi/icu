package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestBuildPath(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			got := icu.BuildPath(tt.athleteID, tt.resource, tt.parts...)
			if got != tt.want {
				t.Errorf("BuildPath(%q, %q, %v) = %q, want %q",
					tt.athleteID, tt.resource, tt.parts, got, tt.want)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		path  string
		query map[string]string
		want  string
	}{
		{
			"no query", "/api/v1/athlete/0", nil,
			"https://intervals.icu/api/v1/athlete/0",
		},
		{
			"empty query", "/api/v1/athlete/0",
			map[string]string{},
			"https://intervals.icu/api/v1/athlete/0",
		},
		{
			"single param", "/api/v1/athlete/0/activities",
			map[string]string{"oldest": "2026-05-20"},
			"https://intervals.icu/api/v1/athlete/0/activities?oldest=2026-05-20",
		},
		{
			"multiple params", "/api/v1/athlete/0/activities",
			map[string]string{"oldest": "2026-05-20", "newest": "2026-05-24"},
			"https://intervals.icu/api/v1/athlete/0/activities?newest=2026-05-24&oldest=2026-05-20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := icu.BuildURL(tt.path, tt.query)
			if got != tt.want {
				t.Errorf("BuildURL(%q, %v) = %q, want %q", tt.path, tt.query, got, tt.want)
			}
		})
	}
}

func TestBuildURLEscapesQueryComponents(t *testing.T) {
	t.Parallel()

	got := icu.BuildURL("/api/v1/athlete/0/activities", map[string]string{
		"fields": "id,name/start",
		"q":      "café ride",
	})
	want := "https://intervals.icu/api/v1/athlete/0/activities?fields=id%2Cname%2Fstart&q=caf%C3%A9+ride"

	if got != want {
		t.Fatalf("BuildURL escaped query = %q, want %q", got, want)
	}
}

func BenchmarkBuildPathAthleteResource(b *testing.B) {
	var got string

	b.ReportAllocs()

	for range b.N {
		got = icu.BuildPath("i445643", "activities", "search-full")
	}

	if got == "" {
		b.Fatal("empty path")
	}
}

func BenchmarkBuildURLMultipleParams(b *testing.B) {
	var got string

	query := map[string]string{
		"oldest": "2026-05-20",
		"newest": "2026-05-24",
		"fields": "id,name,start_date_local,type,moving_time,icu_training_load",
		"limit":  "200",
	}

	b.ReportAllocs()

	for range b.N {
		got = icu.BuildURL("/api/v1/athlete/i445643/activities", query)
	}

	if got == "" {
		b.Fatal("empty URL")
	}
}
