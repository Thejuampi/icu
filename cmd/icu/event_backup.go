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
	// RestoredEventID is the id a recreate revert created from this backup,
	// written back to the file so the revert is not repeated.
	RestoredEventID int `json:"restoredEventId,omitempty"`
	// PostWriteHash is icu.EventBackupHash of the event this CLI wrote right
	// after the backup (the PUT/POST response), so a later revert or update
	// can tell whether someone else edited the event since.
	PostWriteHash string `json:"postWriteHash,omitempty"`
	PostWriteAt   string `json:"postWriteAt,omitempty"`
	// WritePending is set when the backup is taken and cleared by
	// recordPostWrite. A backup still pending means its write never happened
	// (or failed), so it says nothing about what this CLI last wrote.
	WritePending bool `json:"writePending,omitempty"`
}

// recordPostWrite stores the hash of the event just written in the backup
// taken before the write.
func recordPostWrite(path string, written *icu.Event) error {
	if path == "" || written == nil {
		return nil
	}
	backup, err := readEventBackup(path)
	if err != nil {
		return err
	}
	backup.PostWriteHash = icu.EventBackupHash(written)
	backup.PostWriteAt = time.Now().UTC().Format(time.RFC3339Nano)
	backup.WritePending = false

	return rewriteEventBackup(path, &backup)
}

// lastWrite is what the newest backup of an event says about this CLI's
// last write to it.
type lastWrite struct {
	path   string
	hash   string
	legacy bool // newest backup predates postWriteHash
}

// findLastWrite reads the newest backup of eventID that records a write.
// Backups taken before a delete or cancel record no write to this event id;
// backups whose write never happened (writePending) are skipped.
func findLastWrite(eventID int) (lastWrite, error) {
	paths, err := listEventBackups(eventID)
	if err != nil {
		return lastWrite{}, err
	}
	for _, path := range paths {
		backup, err := readEventBackup(path)
		if err != nil {
			return lastWrite{}, nil //nolint:nilerr // an unreadable backup carries no write record.
		}
		if backup.Reason == eventBackupReasonDelete || backup.Reason == eventBackupReasonDrop {
			return lastWrite{}, nil
		}
		if backup.WritePending {
			continue
		}

		return lastWrite{path: path, hash: backup.PostWriteHash, legacy: backup.PostWriteHash == ""}, nil
	}

	return lastWrite{}, nil
}

// checkThirdPartyEdit aborts (unless force) when the event changed since this
// CLI last wrote it, or when the newest backup is too old to tell.
func checkThirdPartyEdit(eventID int, last lastWrite, current json.RawMessage, force bool) error {
	if force || last.path == "" {
		return nil
	}
	if last.legacy {
		return fmt.Errorf("event %d: newest backup %s has no post-write hash (taken by an older version), so "+
			"edits made in Intervals.icu since cannot be detected; re-run with --force to overwrite", eventID, last.path)
	}
	var event icu.Event
	if err := json.Unmarshal(current, &event); err != nil {
		return fmt.Errorf("decode event %d: %w", eventID, err)
	}
	if got := icu.EventBackupHash(&event); got != last.hash {
		return fmt.Errorf("event %d was changed outside this CLI since it was last written (hash %s, expected %s "+
			"from %s); not overwriting (use --force to overwrite)", eventID, got, last.hash, last.path)
	}

	return nil
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
	return backupEventChecked(client, eventID, reason, nil)
}

// backupEventChecked GETs the event, runs check on it (an error aborts with
// no backup written, so a refused write leaves the backup history as it
// was), then writes the backup.
func backupEventChecked(
	client *icu.Client, eventID int, reason string, check func(json.RawMessage) error,
) (string, json.RawMessage, error) {
	var raw json.RawMessage
	if err := client.Get("events", []string{strconv.Itoa(eventID)}, nil, &raw); err != nil {
		return "", nil, fmt.Errorf("backup event %d: %w", eventID, wrapCommandError(err))
	}
	if check != nil {
		if err := check(raw); err != nil {
			return "", nil, err
		}
	}
	path, err := writeEventBackup(&eventBackup{EventID: eventID, Reason: reason, Event: raw, WritePending: true}, time.Now())

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
			if recreate {
				backup.RestoredEventID = result.RestoredEventID
				if err := rewriteEventBackup(path, &backup); err != nil {
					return err
				}
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
		if !force {
			if err := checkNotRecreated(client, backup); err != nil {
				return err
			}
		}
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
	last, err := findLastWrite(backup.EventID)
	if err != nil {
		return err
	}
	// One GET serves the pre-revert backup and both comparisons; a refused
	// revert writes no snapshot.
	preRevert, _, err := backupEventChecked(client, backup.EventID, eventBackupReasonRevert,
		func(current json.RawMessage) error {
			if err := checkThirdPartyEdit(backup.EventID, last, current, force); err != nil {
				return err
			}
			if !force && sameEventJSON(current, backup.Event) {
				return fmt.Errorf("event %d already matches backup %s; nothing to revert (use --force to PUT anyway)",
					backup.EventID, result.BackupPath)
			}

			return nil
		})
	if err != nil {
		return err
	}
	result.PreRevertBackup = preRevert
	var updated icu.Event
	if err := client.Put("events", []string{strconv.Itoa(backup.EventID)}, nil, backup.Event, &updated); err != nil {
		return wrapCommandError(err)
	}
	result.RestoredEventID = backup.EventID
	if err := recordPostWrite(preRevert, &updated); err != nil {
		return err
	}

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

// checkNotRecreated aborts a recreate that already happened: the backup
// records a restored id, or the calendar already has an event on the same
// date with the same uid, or the same name, category, type and start time.
func checkNotRecreated(client *icu.Client, backup *eventBackup) error {
	if backup.RestoredEventID != 0 {
		return fmt.Errorf("backup of event %d was already restored as event %d (use --force to create another copy)",
			backup.EventID, backup.RestoredEventID)
	}
	var stored icu.Event
	if err := json.Unmarshal(backup.Event, &stored); err != nil {
		return fmt.Errorf("decode backup event %d: %w", backup.EventID, err)
	}
	day := stored.StartDateLocal
	if len(day) < len("2006-01-02") {
		return nil
	}
	day = day[:len("2006-01-02")]
	events, err := readRebalanceEvents(client, day, day)
	if err != nil {
		return err
	}
	for index := range events {
		event := &events[index]
		sameUID := stored.UID != "" && event.UID == stored.UID
		// Name alone false-positives on a day with two similar sessions; a
		// copy must also share the category, type and start time.
		sameEvent := stored.Name != "" && event.Name == stored.Name && event.Category == stored.Category &&
			event.Type == stored.Type && event.StartDateLocal == stored.StartDateLocal
		if sameUID || sameEvent {
			return fmt.Errorf("event %d on %s already matches backup of event %d (uid, or name/category/type/start); "+
				"not recreating (use --force to create another copy)", event.ID, day, backup.EventID)
		}
	}

	return nil
}

func rewriteEventBackup(path string, backup *eventBackup) error {
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal event backup: %w", err)
	}
	if err := writeFileAtomic(filepath.Clean(path), data); err != nil {
		return fmt.Errorf("record restored id in %s: %w", path, err)
	}

	return nil
}

// writeFileAtomic replaces path with data via a synced temp file in the same
// directory and a rename, so a crash never leaves a truncated backup.
func writeFileAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	name := temp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = temp.Close()
			_ = os.Remove(name) //nolint:gosec // name is our own CreateTemp file in the backup dir.
		}
	}()
	if err := temp.Chmod(eventBackupFileMode); err != nil {
		return fmt.Errorf("chmod %s: %w", name, err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", name, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err := os.Rename(name, path); err != nil { //nolint:gosec // path is a backup file inside the backup dir.
		return fmt.Errorf("rename %s: %w", name, err)
	}
	ok = true

	return nil
}
