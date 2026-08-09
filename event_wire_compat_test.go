package icu_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// eventWireGoldenFile mirrors testdata/event_indoor_wire_golden_7a706cb9.json:
// pre-wave-1a wire bytes for representative Indoor states, generated and
// committed at the base SHA (W1a-R03). It is loaded, never recomputed from
// current code — recomputing would assert the code equals itself.
type eventWireGoldenFile struct {
	Fixtures map[string]string `json:"fixtures"`
}

const eventWireGoldenPath = "testdata/event_indoor_wire_golden_7a706cb9.json"

func loadEventWireGolden(t *testing.T) eventWireGoldenFile {
	t.Helper()

	data, err := os.ReadFile(filepath.FromSlash(eventWireGoldenPath))
	if err != nil {
		t.Fatalf("read golden fixture %s: %v", eventWireGoldenPath, err)
	}

	var golden eventWireGoldenFile
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("parse golden fixture %s: %v", eventWireGoldenPath, err)
	}

	return golden
}

// eventIndoorWireFixtures are the same three events used to generate the
// golden file, expressed against the current *bool Indoor field. "absent"
// and "false" must produce byte-identical wire output (both omit "indoor")
// despite differing in every other field — the fixture triple D-25 requires
// so a nil-vs-non-nil-only marshaller cannot pass by accident.
func eventIndoorWireFixtures() map[string]icu.Event {
	return map[string]icu.Event{
		"absent": {
			ID: 1, StartDateLocal: "2026-06-01T08:00:00", Category: "WORKOUT", Type: "Ride",
			Name: "Absent Flag Ride", Description: "steady endurance", TrainingLoad: 60,
			MovingTime: 3600, Distance: 30000, Color: "blue",
			FTP: 270, Tags: []string{"endurance"}, ExternalID: "ext-abs-1",
		},
		"false": {
			ID: 2, StartDateLocal: "2026-06-02T08:00:00", Category: "WORKOUT", Type: "Ride",
			Name: "False Flag Ride", Description: "tempo", TrainingLoad: 70,
			MovingTime: 4200, Distance: 35000, Color: "green", Indoor: falseFlag(),
			FTP: 270, Tags: []string{"tempo"}, ExternalID: "ext-false-2",
		},
		"true": {
			ID: 3, StartDateLocal: "2026-06-03T08:00:00", Category: "WORKOUT", Type: "VirtualRide",
			Name: "True Flag Ride", Description: "zwift", TrainingLoad: 80,
			MovingTime: 3000, Color: "red", Indoor: trueFlag(),
			FTP: 285, Tags: []string{"zwift"}, ExternalID: "ext-true-3",
		},
	}
}

// TestEventMarshalJSONByteIdenticalAcrossContainerShapes is W1a-R03. It tests
// call shapes, not only data: a value-receiver MarshalJSON is reachable from
// bare, pointer, slice, slice-of-pointer and map-value containers alike,
// where a pointer-receiver version would silently be skipped for the
// non-addressable ones (map values, interface-held values).
func TestEventMarshalJSONByteIdenticalAcrossContainerShapes(t *testing.T) {
	t.Parallel()

	golden := loadEventWireGolden(t)
	fixtures := eventIndoorWireFixtures()

	for name, event := range fixtures {
		want := golden.Fixtures[name]

		if want == "" {
			t.Fatalf("golden fixture %s has no entry named %q", eventWireGoldenPath, name)
		}

		t.Run(name+"/bare", func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("marshal bare Event: %v", err)
			}

			if string(data) != want {
				t.Fatalf("bare Event marshal = %s, want %s", data, want)
			}
		})

		t.Run(name+"/pointer", func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(&event)
			if err != nil {
				t.Fatalf("marshal *Event: %v", err)
			}

			if string(data) != want {
				t.Fatalf("*Event marshal = %s, want %s", data, want)
			}
		})

		t.Run(name+"/slice", func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal([]icu.Event{event})
			if err != nil {
				t.Fatalf("marshal []Event: %v", err)
			}

			var elements []json.RawMessage
			if err := json.Unmarshal(data, &elements); err != nil || len(elements) != 1 {
				t.Fatalf("unexpected []Event wire shape %s (err=%v)", data, err)
			}

			if string(elements[0]) != want {
				t.Fatalf("[]Event[0] marshal = %s, want %s", elements[0], want)
			}
		})

		t.Run(name+"/slice_of_pointer", func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal([]*icu.Event{&event})
			if err != nil {
				t.Fatalf("marshal []*Event: %v", err)
			}

			var elements []json.RawMessage
			if err := json.Unmarshal(data, &elements); err != nil || len(elements) != 1 {
				t.Fatalf("unexpected []*Event wire shape %s (err=%v)", data, err)
			}

			if string(elements[0]) != want {
				t.Fatalf("[]*Event[0] marshal = %s, want %s", elements[0], want)
			}
		})

		t.Run(name+"/map_value", func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(map[string]icu.Event{"only": event})
			if err != nil {
				t.Fatalf("marshal map[string]Event: %v", err)
			}

			var byKey map[string]json.RawMessage
			if err := json.Unmarshal(data, &byKey); err != nil {
				t.Fatalf("unexpected map[string]Event wire shape %s (err=%v)", data, err)
			}

			if string(byKey["only"]) != want {
				t.Fatalf("map[string]Event[only] marshal = %s, want %s", byKey["only"], want)
			}
		})
	}
}

// F-26/D-25: write bodies are EventEx, whose Indoor field stays a plain bool
// (unaffected by this wave). This check fails loudly the day that stops
// being true by construction — e.g. if Body were ever loosened to `any` —
// rather than depending on nobody noticing.
func TestRebalanceOperationBodyCannotHoldAnEvent(t *testing.T) {
	t.Parallel()

	bodyField, ok := reflect.TypeOf(icu.RebalanceOperation{}).FieldByName("Body")
	if !ok {
		t.Fatal("icu.RebalanceOperation has no Body field")
	}

	wantType := reflect.TypeOf(icu.EventEx{})
	if bodyField.Type != wantType {
		t.Fatalf("icu.RebalanceOperation.Body type = %s, want %s — a write payload (also aliased as "+
			"icu.CalendarOperation, used by plan.go) must never be able to hold icu.Event (F-26)",
			bodyField.Type, wantType)
	}
}
