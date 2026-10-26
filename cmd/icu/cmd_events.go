package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	registry.Register("events", "delete", eventsDeleteCommand())
	registry.Register("events", "download", eventsDownloadCommand())
	registry.Register("events", "revert", eventsRevertCommand())
	registry.Register("events", "backups", eventsBackupsCommand())
	registry.Register("events", "tags", listAllCommand[[]string]("event-tags", "events tags", "List event tags."))
}

func eventsCreateCommand() *Command {
	return &Command{
		Name: "",
		Usage: "events create --category WORKOUT --type Ride --name NAME --start-date DATE" +
			" [--moving-time SECS] [--training-load N] [--desc DESC | --desc-file PATH] [--calculate-load --ftp N] [--uid UID --upsert]",
		Description: "Create calendar event.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			request, err := buildEventCreate(flags)
			if err != nil {
				return wrapCommandError(err)
			}

			var targets upsertTargets
			if BoolFlag(flags, "upsert") {
				if request.Body.UID == "" {
					return fmt.Errorf("%w: --upsert updates the event with the same --uid; pass --uid", errInvalidValue)
				}
				if targets, err = backupUpsertTargets(client, &request.Body); err != nil {
					return err
				}
			}
			var result icu.Event
			if err := client.Post("events", nil, eventsCreateQuery(flags), request.Body, &result); err != nil {
				return wrapCommandError(err)
			}
			if err := targets.check(&result); err != nil {
				return err
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
		Usage:       "events update <id> [--name NAME] [--type TYPE] [--desc DESC | --desc-file PATH] [--calculate-load --ftp N]",
		Description: "Update calendar event.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			eventID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%w: event id %q", errInvalidValue, args[0])
			}

			if _, err := eventDescription(flags); err != nil {
				return wrapCommandError(err)
			}
			_, raw, err := backupEventRaw(client, eventID, "events update")
			if err != nil {
				return err
			}
			var current icu.Event
			if err := json.Unmarshal(raw, &current); err != nil {
				return fmt.Errorf("decode event %d: %w", eventID, err)
			}

			request, err := buildEventUpdate(flags, current.Type)
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

// eventDescription reads --desc/--description or --desc-file (verbatim, real
// line breaks preserved) and rejects literal escape sequences.
func eventDescription(flags map[string]string) (string, error) {
	description := stringFlagAlias(flags, "desc", "description")
	if path := flags["desc-file"]; path != "" {
		if description != "" {
			return "", fmt.Errorf("%w: use either --desc or --desc-file", errInvalidValue)
		}
		data, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec // G703: the user names their own local description file
		if err != nil {
			return "", fmt.Errorf("read --desc-file: %w", err)
		}
		description = strings.TrimRight(string(data), "\r\n")
		if strings.TrimSpace(description) == "" {
			return "", fmt.Errorf("%w: --desc-file %s is empty", errInvalidValue, path)
		}
	}
	if err := icu.ValidateDescriptionText(description); err != nil {
		return "", err
	}

	return description, nil
}

func buildEventCreate(flags map[string]string) (calculatedEventRequest, error) {
	description, err := eventDescription(flags)
	if err != nil {
		return calculatedEventRequest{}, err
	}
	body := icu.EventEx{
		Category:       icu.StringFlag(flags, "category", "WORKOUT"),
		Type:           icu.StringFlag(flags, "type", "Ride"),
		Name:           icu.StringFlag(flags, "name", ""),
		StartDateLocal: icu.StringFlag(flags, "start-date", ""),
		MovingTime:     IntFlag(flags, "moving-time", 0),
		TrainingLoad:   IntFlag(flags, "training-load", 0),
		Description:    description,
		Color:          icu.StringFlag(flags, "color", ""),
		Indoor:         BoolFlag(flags, "indoor"),
		ExternalID:     icu.StringFlag(flags, "external-id", ""),
		UID:            icu.StringFlag(flags, "uid", ""),
	}

	return applyCalculatedEventLoad(flags, &body, body.Type)
}

// buildEventUpdate builds the PUT body. currentType is the stored event type,
// used to decide FTP pricing when --type is not given.
func buildEventUpdate(flags map[string]string, currentType string) (calculatedEventRequest, error) {
	var body icu.EventEx
	if v := flags["name"]; v != "" {
		body.Name = v
	}
	if v := flags["type"]; v != "" {
		body.Type = v
	}

	description, err := eventDescription(flags)
	if err != nil {
		return calculatedEventRequest{}, err
	}
	body.Description = description

	if v := flags["training-load"]; v != "" {
		body.TrainingLoad = IntFlag(flags, "training-load", 0)
	}

	if v := flags["moving-time"]; v != "" {
		body.MovingTime = IntFlag(flags, "moving-time", 0)
	}

	eventType := body.Type
	if eventType == "" {
		eventType = currentType
	}

	return applyCalculatedEventLoad(flags, &body, eventType)
}

func applyCalculatedEventLoad(flags map[string]string, body *icu.EventEx, eventType string) (calculatedEventRequest, error) {
	request := calculatedEventRequest{Body: *body}
	if !BoolFlag(flags, "calculate-load") {
		return request, nil
	}
	if !icu.IsCyclingActivityType(eventType) {
		return request, fmt.Errorf(
			"%w: --calculate-load prices power steps against FTP and only applies to cycling types; "+
				"%q is not FTP-priced, pass --moving-time/--training-load instead",
			errInvalidValue, eventType,
		)
	}
	if body.Description == "" {
		return request, errMissing("desc")
	}

	doc, err := icu.ParseWorkoutDescription(body.Description)
	if err != nil {
		return request, fmt.Errorf("parse workout description: %w", err)
	}

	estimate := icu.EstimatePlannedLoad(doc, icu.FTPSelectionFromWatts(IntFlag(flags, "ftp", 0)))
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

// upsertSearchDays is how far either side of the new event's date the uid
// search reads. intervals.icu matches upsertOnUid across the whole calendar
// and the events list needs a range, so the read spans ten years each way;
// upsertTargets.check catches a match outside it after the POST.
const upsertSearchDays = 3650

// upsertTargets records what the pre-upsert search saw.
type upsertTargets struct {
	backedUp map[int]bool
	maxID    int
	searched bool
}

// check fails loudly when the upsert updated an event that existed before
// (its id is not newer than every id the search saw) but was not backed up.
// intervals.icu event ids increase, so a freshly created event has an id
// above maxID.
func (targets *upsertTargets) check(result *icu.Event) error {
	if !targets.searched || result.ID == 0 || targets.backedUp[result.ID] || result.ID > targets.maxID {
		return nil
	}

	return fmt.Errorf("upsert updated existing event %d, which was not backed up (its uid %q was not found "+
		"within %d days of the new date); restore it by hand if needed", result.ID, result.UID, upsertSearchDays)
}

// backupUpsertTargets backs up the events an upsert would overwrite:
// POST events?upsertOnUid=true updates the event whose uid equals the body's
// uid (intervals.icu API docs; external_id is only used by events/bulk
// ?upsert=true), so the same uid is searched here.
func backupUpsertTargets(client *icu.Client, body *icu.EventEx) (upsertTargets, error) {
	targets := upsertTargets{backedUp: map[int]bool{}}
	if body.UID == "" || len(body.StartDateLocal) < len("2006-01-02") {
		return targets, nil
	}
	day := body.StartDateLocal[:len("2006-01-02")]
	oldest, err := addDays(day, -upsertSearchDays)
	if err != nil {
		return targets, err
	}
	newest, err := addDays(day, upsertSearchDays)
	if err != nil {
		return targets, err
	}
	events, err := readRebalanceEvents(client, oldest, newest)
	if err != nil {
		return targets, err
	}
	targets.searched = true
	for index := range events {
		targets.maxID = max(targets.maxID, events[index].ID)
		if events[index].UID == body.UID {
			if _, err := backupEvent(client, events[index].ID, "events create --upsert"); err != nil {
				return targets, err
			}
			targets.backedUp[events[index].ID] = true
		}
	}

	return targets, nil
}
