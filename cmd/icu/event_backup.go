package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	eventBackupDirEnv       = "ICU_BACKUP_DIR"
	eventBackupFileMode     = 0o600
	eventBackupDirMode      = 0o700
	eventBackupReasonDrop   = "calendar " + icu.CalendarActionCancel
	eventBackupReasonDelete = "events delete"
	eventBackupReasonRevert = "events revert"
)

// eventBackup is the on-disk copy of an event taken before it is changed.
// Event is the raw Intervals.icu JSON so a revert restores every field,
// including ones this CLI does not model.
type eventBackup struct {
	EventID int             `json:"eventId"`
	Reason  string          `json:"reason"`
	TakenAt string          `json:"takenAt"`
	Event   json.RawMessage `json:"event"`
}

func eventBackupDir() string {
	if dir := os.Getenv(eventBackupDirEnv); dir != "" {
		return dir
	}

	return filepath.Join(filepath.Dir(icu.ConfigPath()), "backups", "events")
}

// backupEvent GETs the event and writes it to the backup directory before
// any change. A failed backup aborts the change.
func backupEvent(client *icu.Client, eventID int, reason string) (string, error) {
	path, _, err := backupEventRaw(client, eventID, reason)

	return path, err
}

// backupEventRaw is backupEvent that also returns the fetched event JSON, so
// a caller that needs the current event does not GET it twice.
func backupEventRaw(client *icu.Client, eventID int, reason string) (string, json.RawMessage, error) {
	var raw json.RawMessage
	if err := client.Get("events", []string{strconv.Itoa(eventID)}, nil, &raw); err != nil {
		return "", nil, fmt.Errorf("backup event %d: %w", eventID, wrapCommandError(err))
	}
	path, err := writeEventBackup(&eventBackup{EventID: eventID, Reason: reason, Event: raw}, time.Now())

	return path, raw, err
}

func writeEventBackup(backup *eventBackup, now time.Time) (string, error) {
	dir := eventBackupDir()
	if err := os.MkdirAll(dir, eventBackupDirMode); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	backup.TakenAt = now.UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal event backup: %w", err)
	}
	name := fmt.Sprintf("%d-%s.json", backup.EventID, now.UTC().Format("20060102T150405.000000000Z"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, eventBackupFileMode); err != nil {
		return "", fmt.Errorf("write event backup: %w", err)
	}

	return path, nil
}

func readEventBackup(path string) (eventBackup, error) {
	data, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec // the user names their own backup file.
	if err != nil {
		return eventBackup{}, fmt.Errorf("read event backup: %w", err)
	}
	var backup eventBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return eventBackup{}, fmt.Errorf("parse event backup: %w", err)
	}
	if backup.EventID == 0 || len(backup.Event) == 0 {
		return eventBackup{}, fmt.Errorf("event backup %s has no event", path)
	}

	return backup, nil
}

// listEventBackups returns backup paths, newest first, optionally for one
// event id.
func listEventBackups(eventID int) ([]string, error) {
	entries, err := os.ReadDir(eventBackupDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list event backups: %w", err)
	}
	prefix := ""
	if eventID != 0 {
		prefix = strconv.Itoa(eventID) + "-"
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || !strings.HasPrefix(name, prefix) {
			continue
		}
		paths = append(paths, filepath.Join(eventBackupDir(), name))
	}
	sort.Slice(paths, func(left, right int) bool {
		return backupTimestamp(paths[left]) > backupTimestamp(paths[right])
	})

	return paths, nil
}

func backupTimestamp(path string) string {
	name := filepath.Base(path)
	if index := strings.Index(name, "-"); index >= 0 {
		return name[index+1:]
	}

	return name
}

func eventsBackupsCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "events backups [--id ID]",
		Description: "List local event backups (newest first). Local only.",
		Run: func(_ []string, flags map[string]string, _ *icu.Client) error {
			paths, err := listEventBackups(IntFlag(flags, "id", 0))
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(paths))
			for _, path := range paths {
				backup, err := readEventBackup(path)
				if err != nil {
					continue
				}
				out = append(out, map[string]any{
					"path": path, "eventId": backup.EventID, "reason": backup.Reason, "takenAt": backup.TakenAt,
				})
			}

			return writeJSON(out)
		},
	}
}

type eventRevertResult struct {
	Action          string `json:"action"`
	EventID         int    `json:"eventId"`
	RestoredEventID int    `json:"restoredEventId,omitempty"`
	BackupPath      string `json:"backupPath"`
	PreRevertBackup string `json:"preRevertBackup,omitempty"`
	DryRun          bool   `json:"dryRun,omitempty"`
}

func eventsRevertCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "events revert (--backup PATH | --id ID) [--recreate] [--force] [--dry-run]",
		Description: "Restore a calendar event from a local backup taken by events update / plan accept / " +
			"rebalance accept / events delete. Updates the event in place (after backing up its current state); an " +
			"event the backup recorded as cancelled or deleted, or --recreate, is created again instead. --id picks " +
			"the newest backup that is not itself a pre-revert snapshot. Aborts when the event already matches " +
			"the backup unless --force.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			path, err := resolveRevertBackup(flags)
			if err != nil {
				return err
			}
			backup, err := readEventBackup(path)
			if err != nil {
				return err
			}
			recreate := BoolFlag(flags, "recreate") || backup.Reason == eventBackupReasonDrop ||
				backup.Reason == eventBackupReasonDelete
			result := eventRevertResult{Action: "update", EventID: backup.EventID, BackupPath: path}
			if recreate {
				result.Action = "create"
			}
			if BoolFlag(flags, "dry-run") {
				result.DryRun = true

				return writeJSON(result)
			}
			if err := applyEventRevert(client, &backup, recreate, BoolFlag(flags, "force"), &result); err != nil {
				return err
			}

			return writeJSON(result)
		},
	}
}

func resolveRevertBackup(flags map[string]string) (string, error) {
	if path := icu.StringFlag(flags, "backup", ""); path != "" {
		return path, nil
	}
	eventID := IntFlag(flags, "id", 0)
	if eventID == 0 {
		return "", errMissing("backup or id")
	}
	paths, err := listEventBackups(eventID)
	if err != nil {
		return "", err
	}
	// A pre-revert snapshot holds the state the revert replaced; picking it
	// would make a second revert undo the first.
	for _, path := range paths {
		backup, err := readEventBackup(path)
		if err != nil || backup.Reason == eventBackupReasonRevert {
			continue
		}

		return path, nil
	}

	return "", fmt.Errorf("no local backup for event %d in %s", eventID, eventBackupDir())
}

func applyEventRevert(client *icu.Client, backup *eventBackup, recreate, force bool, result *eventRevertResult) error {
	if recreate {
		body, err := withoutEventID(backup.Event)
		if err != nil {
			return err
		}
		var created icu.Event
		if err := client.Post("events", nil, nil, body, &created); err != nil {
			return wrapCommandError(err)
		}
		result.RestoredEventID = created.ID

		return nil
	}
	// One GET serves both the pre-revert backup and the comparison.
	preRevert, current, err := backupEventRaw(client, backup.EventID, eventBackupReasonRevert)
	if err != nil {
		return err
	}
	if !force && sameEventJSON(current, backup.Event) {
		return fmt.Errorf("event %d already matches backup %s; nothing to revert (use --force to PUT anyway)",
			backup.EventID, result.BackupPath)
	}
	result.PreRevertBackup = preRevert
	var updated icu.Event
	if err := client.Put("events", []string{strconv.Itoa(backup.EventID)}, nil, backup.Event, &updated); err != nil {
		return wrapCommandError(err)
	}
	result.RestoredEventID = backup.EventID

	return nil
}

func withoutEventID(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("parse backed-up event: %w", err)
	}
	delete(fields, "id")

	return fields, nil
}

// sameEventJSON compares two event JSON documents ignoring key order and
// whitespace.
func sameEventJSON(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	leftJSON, leftErr := json.Marshal(leftValue)
	rightJSON, rightErr := json.Marshal(rightValue)

	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func eventsDeleteCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "events delete <id>",
		Description: "Delete calendar event. Backs the event up locally first; restore it with " +
			"events revert --id ID.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("event id")
			}
			eventID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%w: event id %q", errInvalidValue, args[0])
			}
			path, err := backupEvent(client, eventID, eventBackupReasonDelete)
			if err != nil {
				return err
			}
			if err := client.Delete("events", []string{args[0]}, nil, nil); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(map[string]any{"deleted": eventID, "backupPath": path})
		},
	}
}
