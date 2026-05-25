package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	type S struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err := WriteJSON(&buf, S{Name: "Juan", Age: 36})
	if err != nil {
		t.Fatal(err)
	}

	var got S
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "Juan" || got.Age != 36 {
		t.Errorf("WriteJSON roundtrip failed: %+v", got)
	}
}

func TestWriteCompactJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	type S struct {
		Name string `json:"name"`
	}

	err := WriteCompactJSON(&buf, S{Name: "Juan"})
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, `"name":"Juan"`) {
		t.Errorf("WriteCompactJSON = %q, want compact JSON", out)
	}
}

func TestWriteCSV(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := WriteCSV(&buf, []string{"name", "age"}, [][]string{{"Juan", "36"}, {"Ana", "28"}})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "name,age") || !strings.Contains(got, "Juan,36") {
		t.Errorf("WriteCSV = %q", got)
	}
}

func TestWriteTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := WriteTable(&buf, []string{"Name", "Age"}, [][]string{{"Juan", "36"}})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Name") || !strings.Contains(got, "Juan") {
		t.Errorf("WriteTable = %q", got)
	}
}
