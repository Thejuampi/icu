package icu

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

func WriteTable(writer io.Writer, headers []string, rows [][]string) error {
	var sb strings.Builder

	widths := columnWidths(headers, rows)

	writeTableRow(&sb, widths, headers)
	writeDividerLine(&sb, widths)
	writeTableRows(&sb, widths, rows)

	if _, err := fmt.Fprint(writer, sb.String()); err != nil {
		return fmt.Errorf("writing table: %w", err)
	}

	return nil
}

func columnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))

	for idx, header := range headers {
		widths[idx] = len(header)
	}

	for _, row := range rows {
		for idx, cell := range row {
			if idx < len(widths) && len(cell) > widths[idx] {
				widths[idx] = len(cell)
			}
		}
	}

	return widths
}

func writeDividerLine(sb *strings.Builder, widths []int) {
	for idx, width := range widths {
		sb.WriteString(strings.Repeat("-", width))

		if idx < len(widths)-1 {
			sb.WriteString("  ")
		}
	}

	sb.WriteString("\n")
}

func writeTableRows(sb *strings.Builder, widths []int, rows [][]string) {
	for _, row := range rows {
		writeTableRow(sb, widths, row)
	}
}

func writeTableRow(sb *strings.Builder, widths []int, row []string) {
	for idx := range widths {
		cell := ""
		if idx < len(row) {
			cell = row[idx]
		}
		writePaddedCell(sb, widths[idx], cell)

		if idx < len(widths)-1 {
			sb.WriteString("  ")
		}
	}

	sb.WriteString("\n")
}

func writePaddedCell(sb *strings.Builder, width int, cell string) {
	sb.WriteString(cell)
	if padding := width - len(cell); padding > 0 {
		sb.WriteString(strings.Repeat(" ", padding))
	}
}
