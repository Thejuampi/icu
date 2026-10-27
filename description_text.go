package icu

import (
	"errors"
	"strings"
)

// ErrEscapedDescription reports a description that carries the two
// characters `\` `n` (or `\r`, `\t`) instead of a real line break. Intervals.icu
// stores them literally, so the workout renders on one line and its steps
// are not parsed.
var ErrEscapedDescription = errors.New(
	"description contains literal escape sequences (\\n, \\r or \\t): use real line breaks, " +
		"e.g. --desc-file PATH or bash $'line 1\\nline 2'",
)

// ValidateDescriptionText rejects literal escape sequences in a description.
func ValidateDescriptionText(description string) error {
	for _, escape := range []string{`\n`, `\r`, `\t`} {
		if strings.Contains(description, escape) {
			return ErrEscapedDescription
		}
	}

	return nil
}

// RenderDescriptionLines is the description as Intervals.icu will show it,
// one entry per line.
func RenderDescriptionLines(description string) []string {
	if description == "" {
		return nil
	}

	return strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n")
}
