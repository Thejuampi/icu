package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

const okJSON = `{"ok":true}`

type commandCase struct {
	Name       string
	Resource   string
	Action     string
	Args       []string
	Flags      map[string]string
	WantMethod string
	WantPath   string
	WantOutput string
}

type requestRecord struct {
	Method string
	Path   string
	Query  string
}

type commandServer struct {
	Requests []requestRecord
}

func TestRegisteredCommandsFunctionalHTTPFlows(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAllCommands(registry)

	serverState := &commandServer{Requests: nil}
	server := httptest.NewServer(http.HandlerFunc(serverState.handle))
	t.Cleanup(server.Close)

	client := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(server.Client()),
		icu.WithBaseURL(server.URL),
	)

	uploadFile := commandTestFile(t, "activity.fit", "fit-data")
	wellnessCSV := commandTestFile(t, "wellness.csv", "date,weight\n2026-06-01,80")
	wellnessBulk := commandTestFile(t, "wellness.json", `[{"id":"2026-06-01","weight":80}]`)

	for _, tc := range functionalCommandCases(uploadFile, wellnessCSV, wellnessBulk) {
		runCommandCase(t, registry, client, serverState, &tc)
	}
}

func TestRegisteredCommandsReturnErrorsOnClientFailure(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAllCommands(registry)

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	t.Cleanup(errServer.Close)

	errClient := icu.NewClient(
		"test-key",
		"0",
		icu.WithHTTPClient(errServer.Client()),
		icu.WithBaseURL(errServer.URL),
	)

	for _, tc := range errorCommandCases() {
		cmd, ok := registry.Lookup(tc.Resource, tc.Action)
		if !ok {
			t.Fatalf("%s: missing command %s %s", tc.Name, tc.Resource, tc.Action)
		}

		if err := cmd.Run(tc.Args, tc.Flags, errClient); err == nil {
			t.Fatalf("%s: expected error, got nil", tc.Name)
		}
	}
}

func TestCLIParsingAndHelpFunctional(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAllCommands(registry)

	var buf bytes.Buffer

	printHelp(registry, &buf, "")

	output := buf.String()
	if !strings.Contains(output, "icu - Intervals.icu CLI") {
		t.Fatalf("printHelp output missing header: %s", output)
	}

	if !strings.Contains(output, "Resources:") {
		t.Fatalf("printHelp output missing Resources: %s", output)
	}

	flags := parseFlags([]string{"--oldest", "2026-06-01", "--limit=10", "positional", "-h"})

	if flags["oldest"] != "2026-06-01" || flags["limit"] != "10" || flags["h"] != strTrue ||
		flags["_posargs_"] != "positional" {
		t.Fatalf("parse flags = %+v", flags)
	}
}

func TestRegisteredCommandsValidateRequiredArgs(t *testing.T) {
	t.Parallel()

	registry := NewCommandRegistry()
	registerAllCommands(registry)

	client := icu.NewClient("test-key", "0")

	for _, tc := range requiredArgCommandCases() {
		cmd, ok := registry.Lookup(tc.Resource, tc.Action)
		if !ok {
			t.Fatalf("%s: missing command %s %s", tc.Name, tc.Resource, tc.Action)
		}

		err := cmd.Run(tc.Args, tc.Flags, client)
		if !errors.Is(err, errMissingRequired) {
			t.Fatalf("%s: error = %v, want missing required", tc.Name, err)
		}
	}
}

func functionalCommandCases(uploadFile, wellnessCSV, wellnessBulk string) []commandCase {
	var cases []commandCase
	cases = append(cases, activityListCommandCases(uploadFile)...)
	cases = append(cases, activityDetailCommandCases()...)
	cases = append(cases, athleteAndAnalysisCommandCases()...)
	cases = append(cases, wellnessCommandCases(wellnessCSV, wellnessBulk)...)
	cases = append(cases, eventCommandCases()...)
	cases = append(cases, libraryCommandCases()...)
	cases = append(cases, sportCommandCases()...)
	cases = append(cases, utilityCommandCases()...)

	return cases
}

func errorCommandCases() []commandCase {
	return []commandCase{
		commandFlow("error activity show", "activity", "show", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity intervals", "activity", "intervals", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity streams", "activity", "streams", []string{"i1"}, flags("types", "watts"), "", "", ""),
		commandFlow("error activity power-curve", "activity", "power-curve", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity hr-curve", "activity", "hr-curve", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity pace-curve", "activity", "pace-curve", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity power-vs-hr", "activity", "power-vs-hr", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity best-efforts", "activity", "best-efforts", []string{"i1"}, flags("duration", "300"), "", "", ""),
		commandFlow("error activity map", "activity", "map", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity weather", "activity", "weather", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity file", "activity", "file", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activities get", "activities", "get", []string{"i1", "i2"}, nil, "", "", ""),
		commandFlow("error activities search", "activities", "search", []string{"tempo"}, flags("limit", "1"), "", "", ""),
		commandFlow("error activities around", "activities", "around", []string{"i1"}, flags("limit", "2", "route-id", "r1"), "", "", ""),
		commandFlow("error athlete get", "athlete", "show", nil, nil, "", "", ""),
		commandFlow("error athlete summary", "athlete", "summary", nil, nil, "", "", ""),
		commandFlow("error athlete profile", "athlete", "profile", nil, nil, "", "", ""),
		commandFlow("error wellness list", "wellness", "list", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error wellness get", "wellness", "get", nil, flags("oldest", "2026-06-01"), "", "", ""),
		commandFlow("error events list", "events", "list", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error workouts list", "workouts", "list", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error ftp show", "ftp", "show", nil, nil, "", "", ""),
		commandFlow("error gear list", "gear", "list", nil, nil, "", "", ""),
		commandFlow("error gear update", "gear", "update", []string{"g1"}, flags("name", "Bike", "distance", "2000"), "", "", ""),
		commandFlow("error gear create", "gear", "create", nil, flags("name", "Bike", "type", "Bike", "distance", "1000"), "", "", ""),
		commandFlow("error routes list", "routes", "list", nil, nil, "", "", ""),
		commandFlow("error routes get", "routes", "get", []string{"r1"}, nil, "", "", ""),
		commandFlow("error chats list", "chats", "list", nil, nil, "", "", ""),
		commandFlow("error chats get", "chats", "get", []string{"c1"}, nil, "", "", ""),

		commandFlow("error sports list", "sports", "list", nil, nil, "", "", ""),
		commandFlow("error sports get", "sports", "get", []string{"Ride"}, nil, "", "", ""),
		commandFlow("error folders list", "folders", "list", nil, nil, "", "", ""),
		commandFlow("error folders create", "folders", "create", nil, flags("name", "Test"), "", "", ""),
		commandFlow("error events delete", "events", "delete", []string{"1"}, nil, "", "", ""),
		commandFlow("error events download", "events", "download", []string{"1"}, nil, "", "", ""),
		commandFlow("error workouts delete", "workouts", "delete", []string{"1"}, nil, "", "", ""),
		commandFlow("error wellness update", "wellness", "update", []string{"2026-06-01"}, flags("weight", "80"), "", "", ""),
		commandFlow("error curves power", "curves", "power", nil, nil, "", "", ""),
		commandFlow("error curves hr", "curves", "hr", nil, nil, "", "", ""),
		commandFlow("error custom list", "custom", "list", nil, nil, "", "", ""),
		commandFlow("error custom get", "custom", "get", []string{"1"}, nil, "", "", ""),
		commandFlow("error fitness-events list", "fitness-events", "list", nil, nil, "", "", ""),
		commandFlow("error shared-event get", "shared-event", "get", []string{"1"}, nil, "", "", ""),
		commandFlow("error weather forecast", "weather", "forecast", nil, nil, "", "", ""),
		commandFlow("error weather config", "weather", "config", nil, nil, "", "", ""),
		commandFlow("error chats messages", "chats", "messages", []string{"c1"}, nil, "", "", ""),
		commandFlow("error chats send", "chats", "send", nil, flags("message", "hello"), "", "", ""),
		commandFlow("error tags activities", "tags", "activities", nil, nil, "", "", ""),
		commandFlow("error tags events", "tags", "events", nil, nil, "", "", ""),
		commandFlow("error tags workouts", "tags", "workouts", nil, nil, "", "", ""),
		commandFlow("error activity delete", "activity", "delete", []string{"i1"}, nil, "", "", ""),
		commandFlow("error ftp update", "ftp", "update", nil, flags("ftp", "290"), "", "", ""),
		commandFlow("error sports update", "sports", "update", []string{"Ride"}, flags("ftp", "290"), "", "", ""),
		commandFlow("error sports delete", "sports", "delete", []string{"Ride"}, nil, "", "", ""),
		commandFlow("error curves pace", "curves", "pace", nil, nil, "", "", ""),
		commandFlow("error curves power-hr", "curves", "power-hr", nil, nil, "", "", ""),
		commandFlow("error curves mmp", "curves", "mmp", nil, nil, "", "", ""),
		commandFlow("error custom create", "custom", "create", nil, flags("name", "Test", "type", "NUMBER"), "", "", ""),
		commandFlow("error custom update", "custom", "update", []string{"1"}, flags("name", "Test"), "", "", ""),
		commandFlow("error folders update", "folders", "update", []string{"f1"}, flags("name", "Test"), "", "", ""),
		commandFlow("error analysis cycling", "analysis", "cycling", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error analysis wellness", "analysis", "wellness", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error analysis plan", "analysis", "plan", nil, flags("plan-start", "2026-06-01", "plan-end", "2026-06-07"), "", "", ""),
		commandFlow("error analysis adaptation", "analysis", "adaptation", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error events create", "events", "create", nil, flags("name", "Test"), "", "", ""),
		commandFlow("error events update", "events", "update", []string{"1"}, flags("name", "Test"), "", "", ""),
		commandFlow("error activity best-efforts", "activity", "best-efforts", []string{"i1"}, flags("duration", "300"), "", "", ""),
		commandFlow("error activity map", "activity", "map", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activities csv", "activities", "csv", nil, nil, "", "", ""),
		commandFlow("error activities manual", "activities", "manual", nil, flags("type", "Ride", "name", "Manual", "moving-time", "3600"), "", "", ""),
		commandFlow("error athlete update", "athlete", "update", nil, flags("weight", "81"), "", "", ""),
		commandFlow("error athlete settings", "athlete", "settings", nil, nil, "", "", ""),
		commandFlow("error athlete plan", "athlete", "plan", nil, nil, "", "", ""),
		commandFlow("error analysis plan with plan-start", "analysis", "plan", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07", "plan-start", "2026-06-01"), "", "", ""),
		commandFlow("error events tags", "events", "tags", nil, nil, "", "", ""),
		commandFlow("error workouts list", "workouts", "list", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"), "", "", ""),
		commandFlow("error workouts get", "workouts", "get", []string{"1"}, nil, "", "", ""),
		commandFlow("error workouts tags", "workouts", "tags", nil, nil, "", "", ""),
		commandFlow("error custom delete", "custom", "delete", []string{"1"}, nil, "", "", ""),
		commandFlow("error activity fit-file", "activity", "fit-file", []string{"i1"}, nil, "", "", ""),
		commandFlow("error activity gpx-file", "activity", "gpx-file", []string{"i1"}, nil, "", "", ""),
		commandFlow("error workouts tags second", "workouts", "tags", nil, nil, "", "", ""),
		commandFlow("error ftp show again", "ftp", "show", nil, nil, "", "", ""),
		commandFlow("error gear delete", "gear", "delete", []string{"g1"}, nil, "", "", ""),
		commandFlow("error curves pace again", "curves", "pace", nil, nil, "", "", ""),
		commandFlow("error shared-event get again", "shared-event", "get", []string{"1"}, nil, "", "", ""),
	}
}

func activityListCommandCases(uploadFile string) []commandCase {
	return []commandCase{
		commandFlow("activities list", "activities", "list", nil,
			flags("oldest", "2026-06-01", "newest", "2026-06-07", "limit", "2"),
			http.MethodGet, "/api/v1/athlete/0/activities", "Ride"),
		commandFlow("activities get", "activities", "get", []string{"i1", "i2"}, nil,
			http.MethodGet, "/api/v1/athlete/0/activities/i1,i2", "Ride"),
		commandFlow("activities upload", "activities", "upload", []string{uploadFile}, flags("name", "Upload"),
			http.MethodPost, "/api/v1/athlete/0/activities", ""),
		commandFlow("activities csv", "activities", "csv", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/activities", "download-data"),
		commandFlow("activities search", "activities", "search", []string{"tempo"}, flags("limit", "1"),
			http.MethodGet, "/api/v1/athlete/0/activities/search", "Search Ride"),
		commandFlow("activities search full", "activities", "search-full", []string{"tempo"}, flags("limit", "1"),
			http.MethodGet, "/api/v1/athlete/0/activities/search-full", "Ride"),
		commandFlow("activities interval search", "activities", "interval-search", nil,
			flags("minSecs", "60", "maxSecs", "300"),
			http.MethodGet, "/api/v1/athlete/0/activities/interval-search", "Ride"),
		commandFlow("activities around", "activities", "around", []string{"i1"},
			flags("limit", "2", "route-id", "r1"),
			http.MethodGet, "/api/v1/athlete/0/activities/around", "Ride"),
		commandFlow("activities manual", "activities", "manual", nil,
			flags("type", "Ride", "name", "Manual", "moving-time", "3600", "distance", "25000", "training-load", "50"),
			http.MethodPost, "/api/v1/athlete/0/activities/manual", "Ride"),
	}
}

func activityDetailCommandCases() []commandCase {
	return []commandCase{
		commandFlow("activity show", "activity", "show", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1", "Ride"),
		commandFlow("activity update", "activity", "update", []string{"i1"}, flags("name", "Updated", "type", "Ride"),
			http.MethodPut, "/api/v1/activity/i1", "Ride"),
		commandFlow("activity delete", "activity", "delete", []string{"i1"}, nil,
			http.MethodDelete, "/api/v1/activity/i1", "deleted"),
		commandFlow("activity intervals", "activity", "intervals", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/intervals", "icuIntervals"),
		commandFlow("activity streams", "activity", "streams", []string{"i1"}, flags("types", "watts"),
			http.MethodGet, "/api/v1/activity/i1/streams", "watts"),
		commandFlow("activity power curve", "activity", "power-curve", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/power-curve", "ok"),
		commandFlow("activity hr curve", "activity", "hr-curve", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/hr-curve", "ok"),
		commandFlow("activity pace curve", "activity", "pace-curve", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/pace-curve", "ok"),
		commandFlow("activity power vs hr", "activity", "power-vs-hr", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/power-vs-hr", "ok"),
		commandFlow("activity detail", "activity", "best-efforts", []string{"i1"}, flags("duration", "300"),
			http.MethodGet, "/api/v1/activity/i1/best-efforts", "ok"),
		commandFlow("activity map", "activity", "map", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/map", "ok"),
		commandFlow("activity weather", "activity", "weather", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/weather-summary", "ok"),
		commandFlow("activity file", "activity", "file", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/file", "download-data"),
		commandFlow("activity fit file", "activity", "fit-file", []string{"i1"}, nil,
			http.MethodGet, "/api/v1/activity/i1/fit-file", "download-data"),
	}
}

func athleteAndAnalysisCommandCases() []commandCase {
	return []commandCase{
		commandFlow("athlete show", "athlete", "show", nil, nil,
			http.MethodGet, "/api/v1/athlete/0", "Tester"),
		commandFlow("athlete update", "athlete", "update", nil, flags("weight", "80", "height", "1.81", "name", "Tester"),
			http.MethodPut, "/api/v1/athlete/0", "Tester"),
		commandFlow("athlete profile", "athlete", "profile", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/profile", "Tester"),
		commandFlow("athlete summary", "athlete", "summary", nil, flags("start", "2026-06-01", "end", "2026-06-07"),
			http.MethodGet, "/api/v1/athlete/0/athlete-summary", "Tester"),
		commandFlow("athlete plan get", "athlete", "plan", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/training-plan", "trainingPlanId"),
		commandFlow("athlete plan update", "athlete", "plan", nil, flags("plan-id", "12", "start-date", "2026-06-01"),
			http.MethodPut, "/api/v1/athlete/0/training-plan", "trainingPlanId"),
		commandFlow("athlete settings", "athlete", "settings", nil, flags("device", "desktop"),
			http.MethodGet, "/api/v1/athlete/0/settings/desktop", "ok"),
		commandFlow("analysis cycling", "analysis", "cycling", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"),
			http.MethodGet, "/api/v1/athlete/0/activities", "cyclingActivities"),
		commandFlow("analysis wellness", "analysis", "wellness", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"),
			http.MethodGet, "/api/v1/athlete/0/wellness", "coverage"),
		commandFlow("analysis plan", "analysis", "plan", nil,
			flags(
				"history-oldest", "2026-05-01",
				"history-newest", "2026-05-31",
				"plan-oldest", "2026-06-01",
				"plan-newest", "2026-06-07",
			),
			http.MethodGet, "/api/v1/athlete/0/events", "plannedSessions"),
		commandFlow("analysis adaptation", "analysis", "adaptation", nil,
			flags("oldest", "2026-05-01", "newest", "2026-06-01"),
			http.MethodGet, "/api/v1/athlete/0/wellness", "powerCurveDeltas"),
	}
}

func wellnessCommandCases(wellnessCSV, wellnessBulk string) []commandCase {
	return []commandCase{
		commandFlow("wellness list", "wellness", "list", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07"),
			http.MethodGet, "/api/v1/athlete/0/wellness", "2026-06-01"),
		commandFlow("wellness get", "wellness", "get", []string{"2026-06-01"}, nil,
			http.MethodGet, "/api/v1/athlete/0/wellness/2026-06-01", "2026-06-01"),
		commandFlow("wellness update", "wellness", "update", []string{"2026-06-01"},
			flags(
				"weight", "80",
				"resting-hr", "50",
				"hrv", "42",
				"sleep-secs", "28800",
				"sleep-score", "82",
				"locked", "true",
			),
			http.MethodPut, "/api/v1/athlete/0/wellness/2026-06-01", "2026-06-01"),
		commandFlow("wellness bulk", "wellness", "bulk", nil, flags("file", wellnessBulk),
			http.MethodPut, "/api/v1/athlete/0/wellness-bulk", ""),
		commandFlow("wellness upload", "wellness", "upload", []string{wellnessCSV}, nil,
			http.MethodPost, "/api/v1/athlete/0/wellness", ""),
	}
}

func eventCommandCases() []commandCase {
	return []commandCase{
		commandFlow("events list", "events", "list", nil, flags("oldest", "2026-06-01", "newest", "2026-06-07", "resolve", "true"),
			http.MethodGet, "/api/v1/athlete/0/events", "Workout"),
		commandFlow("events get", "events", "get", []string{"1"}, nil,
			http.MethodGet, "/api/v1/athlete/0/events/1", "Workout"),
		commandFlow("events create", "events", "create", nil,
			flags("name", "Workout", "start-date", "2026-06-01", "moving-time", "3600", "training-load", "50", "indoor", "true"),
			http.MethodPost, "/api/v1/athlete/0/events", "Workout"),
		commandFlow("events update", "events", "update", []string{"1"},
			flags("name", "Workout", "desc", "- Easy 10m 55%", "training-load", "50"),
			http.MethodPut, "/api/v1/athlete/0/events/1", "Workout"),
		commandFlow("events delete", "events", "delete", []string{"1"}, nil,
			http.MethodDelete, "/api/v1/athlete/0/events/1", ""),
		commandFlow("events download", "events", "download", []string{"1"}, flags("ext", "zwo"),
			http.MethodGet, "/api/v1/athlete/0/events/1/download.zwo", "download-data"),
		commandFlow("events tags", "events", "tags", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/event-tags", "tag-a"),
	}
}

func libraryCommandCases() []commandCase {
	return append(workoutCommandCases(), folderCommandCases()...)
}

func workoutCommandCases() []commandCase {
	return []commandCase{
		commandFlow("workouts list", "workouts", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/workouts", "Workout"),
		commandFlow("workouts get", "workouts", "get", []string{"1"}, nil,
			http.MethodGet, "/api/v1/athlete/0/workouts/1", "Workout"),
		commandFlow("workouts create", "workouts", "create", nil,
			flags("name", "Workout", "type", "Ride", "folder-id", "2", "training-load", "50", "moving-time", "3600"),
			http.MethodPost, "/api/v1/athlete/0/workouts", "Workout"),
		commandFlow("workouts update", "workouts", "update", []string{"1"}, flags("name", "Workout", "desc", "Updated"),
			http.MethodPut, "/api/v1/athlete/0/workouts/1", "Workout"),
		commandFlow("workouts delete", "workouts", "delete", []string{"1"}, nil,
			http.MethodDelete, "/api/v1/athlete/0/workouts/1", ""),
		commandFlow("workouts tags", "workouts", "tags", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/workouts/tags", "tag-a"),
	}
}

func folderCommandCases() []commandCase {
	return []commandCase{
		commandFlow("folders list", "folders", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/folders", "Folder"),
		commandFlow("folders create", "folders", "create", nil, flags("name", "Folder", "type", "PLAN"),
			http.MethodPost, "/api/v1/athlete/0/folders", "Folder"),
		commandFlow("folders update", "folders", "update", []string{"1"}, flags("name", "Folder"),
			http.MethodPut, "/api/v1/athlete/0/folders/1", "Folder"),
		commandFlow("folders delete", "folders", "delete", []string{"1"}, nil,
			http.MethodDelete, "/api/v1/athlete/0/folders/1", ""),
	}
}

func sportCommandCases() []commandCase {
	return []commandCase{
		commandFlow("sports list", "sports", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/sport-settings", "Ride"),
		commandFlow("sports get", "sports", "get", []string{"Ride"}, nil,
			http.MethodGet, "/api/v1/athlete/0/sport-settings/Ride", "Ride"),
		commandFlow("sports update", "sports", "update", []string{"Ride"},
			flags("ftp", "285", "lthr", "178", "max-hr", "198", "indoor-ftp", "280"),
			http.MethodPut, "/api/v1/athlete/0/sport-settings/Ride", "Ride"),
		commandFlow("sports delete", "sports", "delete", []string{"1"}, nil,
			http.MethodDelete, "/api/v1/athlete/0/sport-settings/1", ""),
		commandFlow("ftp show", "ftp", "show", nil, flags("sport", "Ride"),
			http.MethodGet, "/api/v1/athlete/0/sport-settings/Ride", "ftp"),
		commandFlow("ftp update", "ftp", "update", nil, flags("sport", "Ride", "value", "285", "indoor", "true"),
			http.MethodPut, "/api/v1/athlete/0/sport-settings/Ride", "Ride"),
	}
}

func utilityCommandCases() []commandCase {
	var cases []commandCase
	cases = append(cases, curveCommandCases()...)
	cases = append(cases, gearAndRouteCommandCases()...)
	cases = append(cases, weatherAndChatCommandCases()...)
	cases = append(cases, customAndMiscCommandCases()...)

	return cases
}

func curveCommandCases() []commandCase {
	return []commandCase{
		commandFlow("curves power", "curves", "power", nil, flags("type", "Ride", "curves", "42d"),
			http.MethodGet, "/api/v1/athlete/0/power-curves", "ok"),
		commandFlow("curves hr", "curves", "hr", nil, flags("type", "Ride"),
			http.MethodGet, "/api/v1/athlete/0/hr-curves", "ok"),
		commandFlow("curves pace", "curves", "pace", nil, flags("type", "Run"),
			http.MethodGet, "/api/v1/athlete/0/pace-curves", "ok"),
		commandFlow("curves power hr", "curves", "power-hr", nil, flags("start", "2026-06-01", "end", "2026-06-07"),
			http.MethodGet, "/api/v1/athlete/0/power-hr-curve", "ok"),
		commandFlow("curves mmp", "curves", "mmp", nil, flags("type", "Ride"),
			http.MethodGet, "/api/v1/athlete/0/mmp-model", "criticalPower"),
	}
}

func gearAndRouteCommandCases() []commandCase {
	return []commandCase{
		commandFlow("gear list", "gear", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/gear", "Bike"),
		commandFlow("gear create", "gear", "create", nil, flags("name", "Bike", "type", "Bike", "distance", "1000"),
			http.MethodPost, "/api/v1/athlete/0/gear", "Bike"),
		commandFlow("gear update", "gear", "update", []string{"g1"}, flags("name", "Bike", "distance", "2000"),
			http.MethodPut, "/api/v1/athlete/0/gear/g1", "Bike"),
		commandFlow("gear delete", "gear", "delete", []string{"g1"}, nil,
			http.MethodDelete, "/api/v1/athlete/0/gear/g1", ""),
		commandFlow("routes list", "routes", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/routes", "ok"),
		commandFlow("routes get", "routes", "get", []string{"1"}, flags("include-path", "true"),
			http.MethodGet, "/api/v1/athlete/0/routes/1", "Route"),
	}
}

func weatherAndChatCommandCases() []commandCase {
	return []commandCase{
		commandFlow("weather config get", "weather", "config", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/weather-config", "forecasts"),
		commandFlow("weather config update", "weather", "config", nil,
			flags("lat", "1.2", "lon", "3.4", "label", "Home", "enabled", "true"),
			http.MethodPut, "/api/v1/athlete/0/weather-config", "forecasts"),
		commandFlow("weather forecast", "weather", "forecast", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/weather-forecast", "forecasts"),
		commandFlow("chats list", "chats", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/chats", "Chat"),
		commandFlow("chats get", "chats", "get", []string{"1"}, nil,
			http.MethodGet, "/api/v1/athlete/0/chats/1", "Chat"),
		commandFlow("chats messages", "chats", "messages", []string{"1"}, flags("limit", "2"),
			http.MethodGet, "/api/v1/athlete/0/chats/1/messages", "hello"),
		commandFlow("chats send", "chats", "send", nil, flags("content", "hello", "to", "i2"),
			http.MethodPost, "/api/v1/chats/send-message", "hello"),
	}
}

func customAndMiscCommandCases() []commandCase {
	return []commandCase{
		commandFlow("custom list", "custom", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/custom-item", "Custom"),
		commandFlow("custom get", "custom", "get", []string{"1"}, nil,
			http.MethodGet, "/api/v1/athlete/0/custom-item/1", "Custom"),
		commandFlow("custom create", "custom", "create", nil, flags("name", "Custom", "type", "FITNESS_CHART"),
			http.MethodPost, "/api/v1/athlete/0/custom-item", "Custom"),
		commandFlow("custom update", "custom", "update", []string{"1"}, flags("name", "Custom", "type", "FITNESS_CHART"),
			http.MethodPut, "/api/v1/athlete/0/custom-item/1", "Custom"),
		commandFlow("custom delete", "custom", "delete", []string{"1"}, nil,
			http.MethodDelete, "/api/v1/athlete/0/custom-item/1", ""),
		commandFlow("fitness events", "fitness-events", "list", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/fitness-model-events", "Workout"),
		commandFlow("tags activities", "tags", "activities", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/activity-tags", "tag-a"),
		commandFlow("tags events", "tags", "events", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/event-tags", "tag-a"),
		commandFlow("tags workouts", "tags", "workouts", nil, nil,
			http.MethodGet, "/api/v1/athlete/0/workout-tags", "tag-a"),
		commandFlow("shared event", "shared-event", "get", []string{"1"}, nil,
			http.MethodGet, "/api/v1/shared-event/1", "ok"),
	}
}

func requiredArgCommandCases() []commandCase {
	return []commandCase{
		missingArg("activities get", "activities", "get"),
		missingArg("activities search", "activities", "search"),
		missingArg("activities around", "activities", "around"),
		missingArg("activity update", "activity", "update"),
		missingArg("activity intervals", "activity", "intervals"),
		missingArg("events get", "events", "get"),
		missingArg("events update", "events", "update"),
		missingArg("events download", "events", "download"),
		missingArg("workouts get", "workouts", "get"),
		missingArg("wellness get", "wellness", "get"),
		missingArg("sports get", "sports", "get"),
		missingArg("ftp update", "ftp", "update"),
		missingArg("routes get", "routes", "get"),
		missingArg("chats get", "chats", "get"),
		missingArg("custom get", "custom", "get"),
		missingArg("shared event get", "shared-event", "get"),
	}
}

func commandFlow(
	name string,
	resource string,
	action string,
	args []string,
	flags map[string]string,
	wantMethod string,
	wantPath string,
	wantOutput string,
) commandCase {
	return commandCase{
		Name:       name,
		Resource:   resource,
		Action:     action,
		Args:       args,
		Flags:      flags,
		WantMethod: wantMethod,
		WantPath:   wantPath,
		WantOutput: wantOutput,
	}
}

func missingArg(name, resource, action string) commandCase {
	return commandFlow(name, resource, action, nil, nil, "", "", "")
}

func flags(values ...string) map[string]string {
	result := map[string]string{}

	for index := 0; index < len(values); index += 2 {
		result[values[index]] = values[index+1]
	}

	return result
}

func runCommandCase(
	t *testing.T,
	registry *CommandRegistry,
	client *icu.Client,
	serverState *commandServer,
	tc *commandCase,
) {
	t.Helper()

	cmd, ok := registry.Lookup(tc.Resource, tc.Action)
	if !ok {
		t.Fatalf("%s: missing command %s %s", tc.Name, tc.Resource, tc.Action)
	}

	serverState.Requests = nil

	if err := cmd.Run(tc.Args, tc.Flags, client); err != nil {
		t.Fatalf("%s: Run error: %v", tc.Name, err)
	}

	if tc.WantMethod != "" {
		assertLastRequest(t, serverState, tc)
	}
}

func assertLastRequest(t *testing.T, serverState *commandServer, tc *commandCase) {
	t.Helper()

	if len(serverState.Requests) == 0 {
		t.Fatalf("%s: no HTTP request recorded", tc.Name)
	}

	last := serverState.Requests[len(serverState.Requests)-1]
	if last.Method != tc.WantMethod || last.Path != tc.WantPath {
		t.Fatalf("%s: request = %s %s, want %s %s", tc.Name, last.Method, last.Path, tc.WantMethod, tc.WantPath)
	}
}

func commandTestFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	return path
}

func (server *commandServer) handle(response http.ResponseWriter, request *http.Request) {
	server.Requests = append(server.Requests, requestRecord{
		Method: request.Method,
		Path:   request.URL.Path,
		Query:  request.URL.RawQuery,
	})

	_, _ = io.Copy(io.Discard, request.Body)

	if isDownloadRequest(request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("download-data"))

		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, _ = response.Write([]byte(functionalJSONResponse(request.Method, request.URL.Path)))
}

func isDownloadRequest(request *http.Request) bool {
	return request.Header.Get("Accept") == "application/octet-stream" ||
		strings.Contains(request.URL.Path, "download.") ||
		isActivityDownloadPath(request.URL.Path)
}

func isActivityDownloadPath(path string) bool {
	return strings.HasSuffix(path, "/file") || strings.HasSuffix(path, "/fit-file") || strings.HasSuffix(path, "/gpx-file")
}

func functionalJSONResponse(method, path string) string {
	responders := []func(string, string) (string, bool){
		deleteResponse,
		tagResponse,
		activityResponse,
		athleteResponse,
		sportResponse,
		wellnessResponse,
		eventResponse,
		libraryResponse,
		utilityResponse,
	}

	for _, responder := range responders {
		body, ok := responder(method, path)
		if ok {
			return body
		}
	}

	return okJSON
}

func deleteResponse(method, _ string) (string, bool) {
	if method == http.MethodDelete {
		return `{"id":"deleted"}`, true
	}

	return "", false
}

func tagResponse(_, path string) (string, bool) {
	if strings.HasSuffix(path, "/activity-tags") || strings.HasSuffix(path, "/event-tags") ||
		strings.HasSuffix(path, "/workout-tags") || strings.HasSuffix(path, "/workouts/tags") {
		return `["tag-a","tag-b"]`, true
	}

	return "", false
}

func activityResponse(method, path string) (string, bool) {
	switch {
	case strings.Contains(path, "/activities/search"):
		return `[{"id":"i1","name":"Search Ride"}]`, true
	case strings.HasSuffix(path, "/activities") && method == http.MethodPost:
		return `{"id":"upload","activities":[{"id":"i1"}]}`, true
	case strings.Contains(path, "/activities/manual"):
		return activityJSON(), true
	case strings.Contains(path, "/activities"):
		return `[` + activityJSON() + `]`, true
	case strings.Contains(path, "/activity/") && strings.HasSuffix(path, "/intervals"):
		return `{"id":"i1","icuIntervals":[{"startIndex":1,"endIndex":2}]}`, true
	case strings.Contains(path, "/activity/") && strings.HasSuffix(path, "/streams"):
		return `[{"type":"watts","data":[1,2]}]`, true
	case strings.Contains(path, "/activity/") && strings.Count(path, "/") == 4:
		return activityJSON(), true
	case strings.Contains(path, "/activity/"):
		return okJSON, true
	default:
		return "", false
	}
}

func athleteResponse(method, path string) (string, bool) {
	switch {
	case strings.HasSuffix(path, "/profile"):
		return `{"athlete":{"id":"0","name":"Tester"}}`, true
	case strings.HasSuffix(path, "/athlete-summary"):
		return `[{"date":"2026-06-01","athleteId":"0","athleteName":"Tester","fitness":50}]`, true
	case strings.HasSuffix(path, "/training-plan"):
		return `{"athlete_id":"0","training_plan_id":12,"training_plan_start_date":"2026-06-01"}`, true
	case strings.Contains(path, "/settings/"):
		return okJSON, true
	case strings.HasSuffix(path, "/athlete/0") && (method == http.MethodGet || method == http.MethodPut):
		return `{"id":"0","name":"Tester","firstname":"Tester","icuFtp":285}`, true
	default:
		return "", false
	}
}

func sportResponse(method, path string) (string, bool) {
	if strings.Contains(path, "/sport-settings") && strings.HasSuffix(path, "/sport-settings") && method == http.MethodGet {
		return `[` + sportJSON() + `]`, true
	}

	if strings.Contains(path, "/sport-settings") {
		return sportJSON(), true
	}

	return "", false
}

func wellnessResponse(method, path string) (string, bool) {
	switch {
	case strings.Contains(path, "/wellness-bulk"):
		return okJSON, true
	case strings.Contains(path, "/wellness") && strings.HasSuffix(path, "/wellness") && method == http.MethodGet:
		return `[` + wellnessJSON() + `]`, true
	case strings.Contains(path, "/wellness"):
		return wellnessJSON(), true
	default:
		return "", false
	}
}

func eventResponse(method, path string) (string, bool) {
	if strings.Contains(path, "/events") && strings.HasSuffix(path, "/events") && method == http.MethodGet {
		return `[` + eventJSON() + `]`, true
	}

	if strings.Contains(path, "/events") {
		return eventJSON(), true
	}

	return "", false
}

func libraryResponse(method, path string) (string, bool) {
	switch {
	case strings.Contains(path, "/workouts") && strings.HasSuffix(path, "/workouts") && method == http.MethodGet:
		return `[` + workoutJSON() + `]`, true
	case strings.Contains(path, "/workouts"):
		return workoutJSON(), true
	case strings.Contains(path, "/folders") && strings.HasSuffix(path, "/folders") && method == http.MethodGet:
		return `[` + folderJSON() + `]`, true
	case strings.Contains(path, "/folders"):
		return folderJSON(), true
	default:
		return "", false
	}
}

func utilityResponse(method, path string) (string, bool) {
	responders := []func(string, string) (string, bool){
		gearRouteResponse,
		weatherChatResponse,
		customMiscResponse,
	}

	for _, responder := range responders {
		body, ok := responder(method, path)
		if ok {
			return body, true
		}
	}

	return "", false
}

func gearRouteResponse(method, path string) (string, bool) {
	switch {
	case strings.Contains(path, "/gear") && strings.HasSuffix(path, "/gear") && method == http.MethodGet:
		return `[` + gearJSON() + `]`, true
	case strings.Contains(path, "/gear"):
		return gearJSON(), true
	case strings.Contains(path, "/routes") && strings.HasSuffix(path, "/routes"):
		return okJSON, true
	case strings.Contains(path, "/routes"):
		return `{"routeId":1,"name":"Route"}`, true
	default:
		return "", false
	}
}

func weatherChatResponse(_, path string) (string, bool) {
	switch {
	case strings.Contains(path, "/weather-config"), strings.Contains(path, "/weather-forecast"):
		return `{"forecasts":[{"id":1,"label":"Home","lat":1.2,"lon":3.4,"enabled":true}]}`, true
	case strings.HasSuffix(path, "/chats"):
		return `[{"id":1,"name":"Chat"}]`, true
	case strings.HasSuffix(path, "/messages"):
		return `[{"id":1,"content":"hello"}]`, true
	case strings.Contains(path, "/send-message"):
		return `{"id":1,"message":{"id":1,"content":"hello"}}`, true
	case strings.Contains(path, "/chats/"):
		return `{"id":1,"name":"Chat"}`, true
	default:
		return "", false
	}
}

func customMiscResponse(method, path string) (string, bool) {
	switch {
	case strings.Contains(path, "/custom-item") && strings.HasSuffix(path, "/custom-item") && method == http.MethodGet:
		return `[{"id":1,"name":"Custom","type":"FITNESS_CHART"}]`, true
	case strings.Contains(path, "/custom-item"):
		return `{"id":1,"name":"Custom","type":"FITNESS_CHART"}`, true
	case strings.Contains(path, "/fitness-model-events"):
		return `[` + eventJSON() + `]`, true
	case strings.Contains(path, "/shared-event"):
		return okJSON, true
	case strings.Contains(path, "/mmp-model"):
		return `{"criticalPower":285,"wPrime":21000,"ftp":285}`, true
	case strings.Contains(path, "/power-curves"):
		return `[{"id":"ok","label":"42d","days":42,"secs":[60,300],"values":[520,410]},` +
			`{"id":"baseline","label":"1y","days":365,"secs":[60,300],"values":[500,420]}]`, true
	case strings.Contains(path, "curves"), strings.Contains(path, "power-hr-curve"):
		return okJSON, true
	default:
		return "", false
	}
}

func activityJSON() string {
	return `{"id":"i1","name":"Ride","type":"Ride","startDateLocal":"2026-06-01T08:00:00","movingTime":3600,"icuTrainingLoad":50}`
}

func sportJSON() string {
	return `{"id":1,"types":["Ride"],"ftp":285,"indoorFtp":280,"lthr":178,"maxHr":198}`
}

func wellnessJSON() string {
	return `{"id":"2026-06-01","weight":80,"restingHr":50,"hrv":42,"sleepScore":82}`
}

func eventJSON() string {
	return `{"id":1,"category":"WORKOUT","type":"Ride","name":"Workout",` +
		`"startDateLocal":"2026-06-01T00:00:00","icuTrainingLoad":50,"movingTime":3600}`
}

func workoutJSON() string {
	return `{"id":1,"name":"Workout","type":"Ride","icuTrainingLoad":50,"movingTime":3600}`
}

func folderJSON() string {
	return `{"id":1,"name":"Folder","type":"PLAN"}`
}

func gearJSON() string {
	return `{"id":"g1","name":"Bike","type":"Bike","distance":1000}`
}
