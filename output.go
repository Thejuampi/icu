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
	return enc.Encode(v)
}

func WriteCompactJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
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
		sb.WriteString(fmt.Sprintf("%-*s", widths[i]+2, h))
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
				sb.WriteString(fmt.Sprintf("%-*s", widths[i]+2, cell))
			}
		}
		sb.WriteString("\n")
	}
	_, err := fmt.Fprint(w, sb.String())
	return err
}
