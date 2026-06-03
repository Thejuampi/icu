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

	writeTableLine(&sb, headers)
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

func writeTableLine(sb *strings.Builder, cells []string) {
	for idx, cell := range cells {
		sb.WriteString(cell)

		if idx < len(cells)-1 {
			sb.WriteString("  ")
		}
	}

	sb.WriteString("\n")
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
	for idx, cell := range row {
		if idx < len(widths) {
			writePaddedCell(sb, widths[idx], cell)

			if idx < len(widths)-1 {
				sb.WriteString("  ")
			}
		}
	}

	sb.WriteString("\n")
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
