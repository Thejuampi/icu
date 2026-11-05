package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	Zones    *eventPowerZones
	// TargetWarnings: steps above icu.WarnPercentFTPTarget % FTP (priced,
	// printed to stderr and returned in JSON).
	TargetWarnings []string
}

type calculatedEventResponse struct {
	Event    icu.Event                `json:"event"`
	Estimate *icu.PlannedLoadEstimate `json:"estimate,omitempty"`
	Zones    *eventPowerZones         `json:"powerZones,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
}

// eventPowerZones says how "Zn" steps were priced. Source is
// "sport-settings" (the event type's power_zones) or "default-fallback"
// (intervals.icu default bands, used only when the athlete has no power zones
// configured for the sport; Note then says so and is also printed to stderr).
type eventPowerZones struct {
	Source    string    `json:"source"`
	Sport     string    `json:"sport"`
	Bounds    []int     `json:"boundsPercentFtp"`
	Midpoints []float64 `json:"midpointsPercentFtp"`
	Note      string    `json:"note,omitempty"`
}

// powerZoneReader reads a sport's power_zones (%FTP upper bounds).
type powerZoneReader func(sport string) ([]int, error)

// sportSettingsPowerZones asks GET sport-settings/{type} with the event type
// as given (--type, else the stored event type); the client does not map
// VirtualRide etc. to Ride itself. Per the OpenAPI spec the id may be an
// activity type, which intervals.icu resolves to the sport settings whose
// types list it. The mock serves by path only, so a type without its own
// fixture (VirtualRide) is a 404 there and takes the announced fallback.
func sportSettingsPowerZones(client *icu.Client) powerZoneReader {
	return func(sport string) ([]int, error) {
		var settings icu.SportSettings
		if err := client.Get("sport-settings", []string{sport}, nil, &settings); err != nil {
			return nil, err //nolint:wrapcheck // reported verbatim in the fallback note
		}

		return settings.PowerZones, nil
	}
}

// powerZonesNotConfigured decides which sport-settings read failures mean
// "the athlete has no zones for this sport" (fall back to the default bands)
// rather than "we could not ask" (abort, write nothing). Only a 404 counts;
// 401/403, 5xx and network errors abort so no PUT goes out with guessed
// bands. Kept in one place so the policy is easy to change.
func powerZonesNotConfigured(err error) bool {
	var status *icu.HTTPStatusError

	return errors.As(err, &status) && status.StatusCode == http.StatusNotFound
}

// resolveEventPowerZones prices zones from the event type's sport settings.
// The hardcoded fallback (intervals.icu default bands) is the client's
// decision, used only when no zones are configured (404, or empty or missing
// power_zones); it is always announced. Any other read failure is an error.
func resolveEventPowerZones(read powerZoneReader, sport string) (*eventPowerZones, error) {
	var bounds []int
	var err error
	if read != nil {
		bounds, err = read(sport)
	}
	if err != nil && !powerZonesNotConfigured(err) {
		return nil, fmt.Errorf("read %s sport-settings power zones (nothing written; Zn steps need them): %w", sport, err)
	}
	if read != nil && err == nil && len(bounds) > 0 {
		midpoints, zoneErr := icu.PowerZoneMidpoints(bounds)
		if zoneErr != nil {
			return nil, fmt.Errorf("%s sport-settings power zones (nothing written; Zn steps need them): %w", sport, zoneErr)
		}

		return &eventPowerZones{Source: "sport-settings", Sport: sport, Bounds: bounds, Midpoints: midpoints}, nil
	}
	reason := "no power_zones configured for type " + sport
	if err != nil {
		reason = "sport-settings for requested type " + sport + " not found (404)"
	} else if read == nil {
		reason = "not read"
	}
	bounds = icu.DefaultPowerZoneBounds()

	return &eventPowerZones{
		Source: "default-fallback", Sport: sport, Bounds: bounds, Midpoints: icu.DefaultPowerZoneMidpoints(),
		Note: fmt.Sprintf("could not use %s sport-settings power zones (%s); priced Z1-Z%d with the intervals.icu default bands %v",
			sport, reason, len(bounds), bounds),
	}, nil
}

func noteEventZones(stderr io.Writer, request *calculatedEventRequest) {
	if request.Zones != nil && request.Zones.Note != "" {
		fmt.Fprintln(stderr, "note: "+request.Zones.Note)
	}
	for _, warning := range request.TargetWarnings {
		fmt.Fprintln(stderr, "warning: "+warning)
	}
}

func osReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G703: the user names this file on the command line; using it is the point.
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
			" [--moving-time SECS] [--training-load N] [--desc DESC | --description DESC | --desc-file PATH] [--calculate-load --ftp N] [--uid UID --upsert] [--color COLOR] [--indoor] [--external-id ID]",
		Description: "Create calendar event.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			request, err := buildEventCreate(flags, sportSettingsPowerZones(client))
			if err != nil {
				return wrapCommandError(err)
			}
			noteEventZones(stderr, &request)

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
				return writeJSON(stdout, calculatedEventOutput(&result, &request))
			}

			return writeJSON(stdout, result)
		},
	}
}

func eventsUpdateCommand() *Command {
	return &Command{
		Name: "",
		Usage: "events update <id> [--name NAME] [--type TYPE] [--desc DESC | --description DESC | --desc-file PATH] " +
			"[--training-load N] [--moving-time SECS] [--calculate-load --ftp N] [--force] [--dry-run]",
		Description: "Update calendar event.",
		Run: func(args []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
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
			last, err := findLastWrite(eventID)
			if err != nil {
				return err
			}
			// Validate (third-party edit, then the request itself) before the
			// backup is written, so a rejected update leaves no backup behind.
			var request calculatedEventRequest
			validate := func(raw json.RawMessage) error {
				if err := checkThirdPartyEdit(eventID, last, raw, BoolFlag(flags, "force")); err != nil {
					return err
				}
				var current icu.Event
				if err := json.Unmarshal(raw, &current); err != nil {
					return fmt.Errorf("decode event %d: %w", eventID, err)
				}
				built, err := buildEventUpdate(flags, current.Type, sportSettingsPowerZones(client))
				if err != nil {
					return wrapCommandError(err)
				}
				request = built

				return nil
			}
			// --dry-run validates against the live event (GET only) and
			// prints the body it would send: no backup, no PUT.
			if BoolFlag(flags, "dry-run") {
				var raw json.RawMessage
				if err := client.Get("events", []string{args[0]}, nil, &raw); err != nil {
					return wrapCommandError(err)
				}
				if err := validate(raw); err != nil {
					return err
				}
				noteEventZones(stderr, &request)
				out := map[string]any{
					"id": eventID, "dryRun": true, "applied": false, "body": request.Body,
					"changes": eventUpdateChanges(raw, request.Body), "message": dryRunNothingWritten,
				}
				if request.Zones != nil {
					out["powerZones"] = request.Zones
				}
				if len(request.TargetWarnings) > 0 {
					out["warnings"] = request.TargetWarnings
				}

				return writeJSON(stdout, out)
			}
			path, _, err := backupEventChecked(client, eventID, "events update", validate)
			if err != nil {
				return err
			}
			noteEventZones(stderr, &request)

			var result icu.Event
			if err := client.Put("events", []string{args[0]}, nil, request.Body, &result); err != nil {
				return wrapCommandError(err)
			}
			if err := recordPostWrite(path, &result); err != nil {
				return err
			}
			if request.Estimate != nil {
				return writeJSON(stdout, calculatedEventOutput(&result, &request))
			}

			return writeJSON(stdout, result)
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

func buildEventCreate(flags map[string]string, zones powerZoneReader) (calculatedEventRequest, error) {
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

	return applyCalculatedEventLoad(flags, &body, body.Type, zones)
}

// buildEventUpdate builds the PUT body. currentType is the stored event type,
// used to decide FTP pricing when --type is not given.
func buildEventUpdate(flags map[string]string, currentType string, zones powerZoneReader) (calculatedEventRequest, error) {
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

	return applyCalculatedEventLoad(flags, &body, eventType, zones)
}

func applyCalculatedEventLoad(flags map[string]string, body *icu.EventEx, eventType string, zones powerZoneReader) (calculatedEventRequest, error) {
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

	var err error
	midpoints := icu.DefaultPowerZoneMidpoints()
	if icu.WorkoutDescriptionUsesPowerZones(body.Description) {
		if request.Zones, err = resolveEventPowerZones(zones, eventType); err != nil {
			return request, err
		}
		midpoints = request.Zones.Midpoints
	}
	doc, err := icu.ParseWorkoutDescriptionWithZones(body.Description, midpoints)
	if err != nil {
		return request, fmt.Errorf("parse workout description: %w", err)
	}
	ftp := IntFlag(flags, "ftp", 0)
	request.TargetWarnings = icu.ValidateWorkoutTargets(doc.Steps, ftp)

	estimate := icu.EstimatePlannedLoad(doc, icu.FTPSelectionFromWatts(ftp))
	if len(estimate.Warnings) > 0 {
		return request, fmt.Errorf("calculate event load: %s", estimate.Warnings[0])
	}

	body.MovingTime = estimate.DurationSeconds
	body.TrainingLoad = estimate.TrainingLoad
	request.Body = *body
	request.Estimate = &estimate

	return request, nil
}

func calculatedEventOutput(event *icu.Event, request *calculatedEventRequest) calculatedEventResponse {
	return calculatedEventResponse{
		Event:    *event,
		Estimate: request.Estimate,
		Zones:    request.Zones,
		Warnings: append(calculatedEventWarnings(event, request.Estimate), request.TargetWarnings...),
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
		Run: func(args []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			if len(args) == 0 {
				return errMissing("event id")
			}

			ext := icu.StringFlag(flags, "ext", "zwo")

			data, err := client.Download("events", []string{args[0], "download." + ext}, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(stdout, data)
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
	backedUp map[int]string // event id -> backup path
	maxID    int
	searched bool
}

// check fails loudly when the upsert updated an event that existed before
// (its id is not newer than every id the search saw) but was not backed up.
// intervals.icu event ids increase, so a freshly created event has an id
// above maxID.
func (targets *upsertTargets) check(result *icu.Event) error {
	if !targets.searched || result.ID == 0 {
		return nil
	}
	if path, ok := targets.backedUp[result.ID]; ok {
		return recordPostWrite(path, result)
	}
	if result.ID > targets.maxID {
		return nil
	}

	return fmt.Errorf("upsert ALREADY OVERWROTE existing event %d (now dated %s, named %q, uid %q) and NO BACKUP "+
		"was taken: its uid was not found within %d days of the new date, so its previous content is lost; "+
		"check it in Intervals.icu", result.ID, eventDay(result.StartDateLocal), result.Name, result.UID,
		upsertSearchDays)
}

// backupUpsertTargets backs up the events an upsert would overwrite:
// POST events?upsertOnUid=true updates the event whose uid equals the body's
// uid (intervals.icu API docs; external_id is only used by events/bulk
// ?upsert=true), so the same uid is searched here.
func backupUpsertTargets(client *icu.Client, body *icu.EventEx) (upsertTargets, error) {
	targets := upsertTargets{backedUp: map[int]string{}}
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
			path, err := backupEvent(client, events[index].ID, "events create --upsert")
			if err != nil {
				return targets, err
			}
			targets.backedUp[events[index].ID] = path
		}
	}

	return targets, nil
}

// eventDay is the YYYY-MM-DD part of a local start date ("" if missing).
func eventDay(startDateLocal string) string {
	if len(startDateLocal) < len("2006-01-02") {
		return startDateLocal
	}

	return startDateLocal[:len("2006-01-02")]
}

type eventFieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// eventUpdateChanges diffs the update body against the live event, by wire
// field name, for --dry-run output.
func eventUpdateChanges(current json.RawMessage, body any) []eventFieldChange {
	var before, after map[string]any
	_ = json.Unmarshal(current, &before)
	data, err := json.Marshal(body)
	if err != nil || json.Unmarshal(data, &after) != nil {
		return nil
	}
	keys := make([]string, 0, len(after))
	for key := range after {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	changes := []eventFieldChange{}
	for _, key := range keys {
		if !reflect.DeepEqual(before[key], after[key]) {
			changes = append(changes, eventFieldChange{Field: key, Before: before[key], After: after[key]})
		}
	}

	return changes
}
