package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OutputFormat int

const (
	FormatJSON OutputFormat = iota
	FormatCSV
	FormatTable
)

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

func WriteCompactJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)

	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding compact JSON: %w", err)
	}

	return nil
}

func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return fmt.Errorf("writing CSV headers: %w", err)
	}

	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}

	cw.Flush()

	if err := cw.Error(); err != nil {
		return fmt.Errorf("flushing CSV: %w", err)
	}

	return nil
}

//nolint:gocognit
func WriteTable(writer io.Writer, headers []string, rows [][]string) error {
	cols := len(headers)
	widths := make([]int, cols)

	for i, h := range headers {
		widths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < cols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder

	for i, h := range headers {
		sb.WriteString(h)

		if i < cols-1 {
			sb.WriteString("  ")
		}
	}

	sb.WriteString("\n")

	for i, w := range widths {
		sb.WriteString(strings.Repeat("-", w))

		if i < cols-1 {
			sb.WriteString("  ")
		}
	}

	sb.WriteString("\n")

	for _, row := range rows {
		for i, cell := range row {
			if i < cols {
				writePaddedCell(&sb, widths[i], cell)

				if i < cols-1 {
					sb.WriteString("  ")
				}
			}
		}

		sb.WriteString("\n")
	}

	if _, err := fmt.Fprint(writer, sb.String()); err != nil {
		return fmt.Errorf("writing table: %w", err)
	}

	return nil
}

const paddingSpaces = 2

func writePaddedCell(sb *strings.Builder, width int, cell string) {
	padding := width - len(cell)
	if padding > 0 {
		sb.WriteString(cell)
		sb.WriteString(strings.Repeat(" ", padding+paddingSpaces))
	} else {
		sb.WriteString(cell)
		sb.WriteString("  ")
	}
}
