package icu_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	type S struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err := icu.WriteJSON(&buf, S{Name: "Juan", Age: 36})
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

	err := icu.WriteCompactJSON(&buf, S{Name: "Juan"})
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

	err := icu.WriteCSV(&buf, []string{"name", "age"}, [][]string{{"Juan", "36"}, {"Ana", "28"}})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "name,age") || !strings.Contains(got, "Juan,36") {
		t.Errorf("WriteCSV = %q", got)
	}
}

func TestWriteJSONError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := icu.WriteJSON(&buf, func() {})
	if err == nil {
		t.Fatal("expected error for non-encodable value")
	}
}

func TestWriteCompactJSONError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := icu.WriteCompactJSON(&buf, func() {})
	if err == nil {
		t.Fatal("expected error for non-encodable value")
	}
}

func TestWriteCSVError(t *testing.T) {
	t.Parallel()

	r, w, _ := os.Pipe()
	w.Close()

	err := icu.WriteCSV(w, []string{"h"}, [][]string{{"v"}})
	if err == nil {
		t.Fatal("expected error for write failure")
	}
	r.Close()
}

func TestWriteTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := icu.WriteTable(&buf, []string{"Name", "Age"}, [][]string{{"Juan", "36"}})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Name") || !strings.Contains(got, "Juan") {
		t.Errorf("WriteTable = %q", got)
	}
}
