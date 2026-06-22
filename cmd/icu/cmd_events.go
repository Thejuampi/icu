package main

import (
	"encoding/json"
	"fmt"
	"os"

	icu "github.com/Thejuampi/icu"
)

const (
	eventLoadTolerance       = 2
	eventMovingTimeTolerance = 1
	eventIntensityTolerance  = 0.01
)

type calculatedEventRequest struct {
	Body     icu.EventEx
	Estimate *icu.PlannedLoadEstimate
}

type calculatedEventResponse struct {
	Event    icu.Event                `json:"event"`
	Estimate *icu.PlannedLoadEstimate `json:"estimate,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
}

func osReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}

	return data, nil
}

func jsonUnmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	return nil
}

func registerEventsCommands(registry *CommandRegistry) {
	registry.Register(
		"events", "list",
		listQueryCommand[[]icu.Event](
			"events",
			"events list --oldest DATE --newest DATE [--category WORKOUT] [--ext zwo|mrc|erg|fit] [--resolve]",
			"List calendar events.",
			eventsListQuery,
		),
	)
	registry.Register("events", "get", getByIDCommand[icu.Event]("events", "events get <id>", "Get event by ID.", "event id", nil))

	registry.Register(
		"events", "create",
		eventsCreateCommand(),
	)

	registry.Register(
		"events", "update",
		eventsUpdateCommand(),
	)

	registry.Register("events", "delete", deleteByIDCommand("events", "events delete <id>", "Delete calendar event.", "event id"))
	registry.Register("events", "download", eventsDownloadCommand())
	registry.Register("events", "tags", listAllCommand[[]string]("event-tags", "events tags", "List event tags."))
}

func eventsCreateCommand() *Command {
	return &Command{
		Name: "",
		Usage: "events create --category WORKOUT --type Ride --name NAME --start-date DATE" +
			" [--moving-time SECS] [--training-load N] [--desc DESC] [--calculate-load --ftp N] [--upsert]",
		Description: "Create calendar event.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			request, err := buildEventCreate(flags)
			if err != nil {
				return wrapCommandError(err)
			}

			var result icu.Event
			if err := client.Post("events", nil, eventsCreateQuery(flags), request.Body, &result); err != nil {
				return wrapCommandError(err)
			}
			if request.Estimate != nil {
				return writeJSON(calculatedEventOutput(&result, request.Estimate))
			}

			return writeJSON(result)
		},
	}
}

func eventsUpdateCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events update <id> --name NAME [--desc DESC] [--calculate-load --ftp N]",
		Description: "Update calendar event.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			request, err := buildEventUpdate(flags)
			if err != nil {
				return wrapCommandError(err)
			}

			var result icu.Event
			if err := client.Put("events", []string{args[0]}, nil, request.Body, &result); err != nil {
				return wrapCommandError(err)
			}
			if request.Estimate != nil {
				return writeJSON(calculatedEventOutput(&result, request.Estimate))
			}

			return writeJSON(result)
		},
	}
}

func buildEventCreate(flags map[string]string) (calculatedEventRequest, error) {
	body := icu.EventEx{
		Category:       icu.StringFlag(flags, "category", "WORKOUT"),
		Type:           icu.StringFlag(flags, "type", "Ride"),
		Name:           icu.StringFlag(flags, "name", ""),
		StartDateLocal: icu.StringFlag(flags, "start-date", ""),
		MovingTime:     IntFlag(flags, "moving-time", 0),
		TrainingLoad:   IntFlag(flags, "training-load", 0),
		Description:    stringFlagAlias(flags, "desc", "description"),
		Color:          icu.StringFlag(flags, "color", ""),
		Indoor:         BoolFlag(flags, "indoor"),
		ExternalID:     icu.StringFlag(flags, "external-id", ""),
	}

	return applyCalculatedEventLoad(flags, &body)
}

func buildEventUpdate(flags map[string]string) (calculatedEventRequest, error) {
	var body icu.EventEx
	if v := flags["name"]; v != "" {
		body.Name = v
	}

	if v := stringFlagAlias(flags, "desc", "description"); v != "" {
		body.Description = v
	}

	if v := flags["training-load"]; v != "" {
		body.TrainingLoad = IntFlag(flags, "training-load", 0)
	}

	if v := flags["moving-time"]; v != "" {
		body.MovingTime = IntFlag(flags, "moving-time", 0)
	}

	return applyCalculatedEventLoad(flags, &body)
}

func applyCalculatedEventLoad(flags map[string]string, body *icu.EventEx) (calculatedEventRequest, error) {
	request := calculatedEventRequest{Body: *body}
	if !BoolFlag(flags, "calculate-load") {
		return request, nil
	}
	if body.Description == "" {
		return request, errMissing("desc")
	}

	doc, err := icu.ParseWorkoutDescription(body.Description)
	if err != nil {
		return request, fmt.Errorf("parse workout description: %w", err)
	}

	estimate := icu.EstimatePlannedLoad(doc, IntFlag(flags, "ftp", 0))
	if len(estimate.Warnings) > 0 {
		return request, fmt.Errorf("calculate event load: %s", estimate.Warnings[0])
	}

	body.MovingTime = estimate.DurationSeconds
	body.TrainingLoad = estimate.TrainingLoad
	request.Body = *body
	request.Estimate = &estimate

	return request, nil
}

func calculatedEventOutput(event *icu.Event, estimate *icu.PlannedLoadEstimate) calculatedEventResponse {
	return calculatedEventResponse{
		Event:    *event,
		Estimate: estimate,
		Warnings: calculatedEventWarnings(event, estimate),
	}
}

func calculatedEventWarnings(event *icu.Event, estimate *icu.PlannedLoadEstimate) []string {
	if estimate == nil {
		return nil
	}

	var warnings []string
	if event.MovingTime > 0 && absInt(event.MovingTime-estimate.DurationSeconds) > eventMovingTimeTolerance {
		warnings = append(warnings, fmt.Sprintf("server movingTime %d differs from local %d", event.MovingTime, estimate.DurationSeconds))
	}
	if event.TrainingLoad > 0 && absInt(event.TrainingLoad-estimate.TrainingLoad) > eventLoadTolerance {
		warnings = append(warnings, fmt.Sprintf("server icuTrainingLoad %d differs from local %d", event.TrainingLoad, estimate.TrainingLoad))
	}
	if event.Intensity > 0 && absFloat(event.Intensity-estimate.IntensityFactor) > eventIntensityTolerance {
		warnings = append(warnings, fmt.Sprintf("server icuIntensity %.3f differs from local %.3f", event.Intensity, estimate.IntensityFactor))
	}

	return warnings
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func eventsListQuery(flags map[string]string) map[string]string {
	q := queryFromFlags(flags, "oldest", "newest", "category", "ext", "limit", "calendar-id")
	if BoolFlag(flags, "resolve") {
		q["resolve"] = strTrue
	}

	return q
}

func eventsCreateQuery(flags map[string]string) map[string]string {
	q := map[string]string{
		"upsertOnUid": "false",
	}
	if BoolFlag(flags, "upsert") {
		q["upsertOnUid"] = "true"
	}

	return q
}

func eventsDownloadCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events download <id> --ext zwo|mrc|erg|fit",
		Description: "Download planned workout file.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			ext := icu.StringFlag(flags, "ext", "zwo")

			data, err := client.Download("events", []string{args[0], "download." + ext}, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(data)
		},
	}
}
