package main

import (
	"fmt"
	"io"

	icu "github.com/Thejuampi/icu"
)

func registerFTPCommands(registry *CommandRegistry) {
	registry.Register("ftp", "show", ftpShowCommand())
	registry.Register("ftp", "update", ftpUpdateCommand())
}

func ftpShowCommand() *Command {
	return &Command{
		Name:        "",
		Usage:       "ftp show [--sport Ride]",
		Description: "Show FTP for a sport type.",
		Run: func(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
			sport := icu.StringFlag(flags, "sport", "Ride")

			var ss icu.SportSettings
			if err := client.Get("sport-settings", []string{sport}, nil, &ss); err != nil {
				return wrapCommandError(err)
			}

			type FTPInfo struct {
				Sport     string `json:"sport"`
				FTP       int    `json:"ftp"`
				IndoorFTP int    `json:"indoorFtp,omitempty"`
				LTHR      int    `json:"lthr,omitempty"`
			}

			return writeJSON(stdout, FTPInfo{
				Sport:     sport,
				FTP:       ss.FTP,
				IndoorFTP: ss.IndoorFTP,
				LTHR:      ss.LTHR,
			})
		},
	}
}

type ftpUpdateResult struct {
	Sport    string             `json:"sport"`
	Field    string             `json:"field"`
	Before   int                `json:"before"`
	After    int                `json:"after"`
	Body     map[string]int     `json:"body"`
	DryRun   bool               `json:"dryRun"`
	Applied  bool               `json:"applied"`
	Message  string             `json:"message,omitempty"`
	ReadBack *icu.SportSettings `json:"readBack,omitempty"`
}

func ftpUpdateCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "ftp update --value WATTS [--sport Ride] [--indoor] [--dry-run]",
		Description: "Update FTP (or indoor FTP with --indoor) for a sport. Shows before/after and a read-back; " +
			"--dry-run only previews. A read-back mismatch exits non-zero.",
		Run: runFTPUpdate,
	}
}

func runFTPUpdate(_ []string, flags map[string]string, client *icu.Client, stdout, stderr io.Writer) error {
	sport := icu.StringFlag(flags, "sport", "Ride")
	val := IntFlag(flags, "value", 0)
	if val <= 0 {
		return errMissing("--value (watts)")
	}

	// Wire names are snake_case (spec SportSettings: ftp, indoor_ftp).
	field := "ftp"
	if BoolFlag(flags, "indoor") {
		field = "indoor_ftp"
	}
	pick := func(s *icu.SportSettings) int {
		if field == "indoor_ftp" {
			return s.IndoorFTP
		}

		return s.FTP
	}

	var current icu.SportSettings
	if err := client.Get("sport-settings", []string{sport}, nil, &current); err != nil {
		return wrapCommandError(err)
	}
	result := ftpUpdateResult{Sport: sport, Field: field, Before: pick(&current), After: val, Body: map[string]int{field: val}}
	if result.Before == val {
		result.Message = "no changes: value already matches"

		return writeJSON(stdout, result)
	}
	if BoolFlag(flags, "dry-run") {
		result.DryRun = true
		result.Message = dryRunNothingWritten

		return writeJSON(stdout, result)
	}

	if err := client.Put("sport-settings", []string{sport}, map[string]string{"recalcHrZones": "false"}, result.Body, nil); err != nil {
		return wrapCommandError(err)
	}
	result.Applied = true

	var after icu.SportSettings
	if err := client.Get("sport-settings", []string{sport}, nil, &after); err != nil {
		return wrapCommandError(fmt.Errorf("write succeeded but read-back failed: %w", err))
	}
	result.ReadBack = &after
	if got := pick(&after); got != val {
		result.Message = fmt.Sprintf("read-back mismatch: %s = %d, want %d", field, got, val)
		if err := writeJSON(stdout, result); err != nil {
			return err
		}

		return fmt.Errorf("ftp update: read-back mismatch on %s", field)
	}

	return writeJSON(stdout, result)
}
