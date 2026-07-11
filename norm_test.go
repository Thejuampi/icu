package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestNormalizeStreamsEmpty(t *testing.T) {
	t.Parallel()

	var raw []icu.ActivityStream
	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams empty returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("NormalizeStreams empty = %v, want empty map", got)
	}
}

func TestNormalizeStreamsSingleFloat64(t *testing.T) {
	t.Parallel()

	var raw []icu.ActivityStream
	raw = append(raw, icu.ActivityStream{
		Type: "watts",
		Name: "Power",
		Data: []float64{100, 200, 300},
	})

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams returned error: %v", err)
	}

	watts, ok := got["watts"]
	if !ok {
		t.Fatal("NormalizeStreams missing watts key")
	}
	if len(watts) != 3 {
		t.Fatalf("watts length = %d, want 3", len(watts))
	}
}

func TestNormalizeStreamsMultipleTypes(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{
			Type: "watts",
			Name: "Power",
			Data: []float64{100, 200, 300},
		},
		{
			Type: "heartrate",
			Name: "HR",
			Data: []float64{120, 130, 140},
		},
	}

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams returned error: %v", err)
	}

	watts, ok := got["watts"]
	if !ok {
		t.Fatal("missing watts")
	}
	hr, ok := got["heartrate"]
	if !ok {
		t.Fatal("missing heartrate")
	}
	if len(watts) != 3 || len(hr) != 3 {
		t.Fatalf("lengths: watts=%d, hr=%d, want both 3", len(watts), len(hr))
	}
}

func TestNormalizeStreamsIntData(t *testing.T) {
	t.Parallel()

	var raw []icu.ActivityStream
	raw = append(raw, icu.ActivityStream{
		Type: "watts",
		Name: "Power",
		Data: []int{150, 250, 350},
	})

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams returned error: %v", err)
	}

	watts, ok := got["watts"]
	if !ok {
		t.Fatal("missing watts")
	}
	if len(watts) != 3 {
		t.Fatalf("watts length = %d, want 3", len(watts))
	}
	if watts[0] != 150 {
		t.Fatalf("watts[0] = %v, want 150", watts[0])
	}
}

func TestNormalizeStreamsComplexData(t *testing.T) {
	t.Parallel()

	var raw []icu.ActivityStream
	raw = append(raw, icu.ActivityStream{
		Type: "watts",
		Name: "Power",
		Data: []any{float64(100), float64(200), float64(300)},
	})

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams returned error: %v", err)
	}

	watts, ok := got["watts"]
	if !ok {
		t.Fatal("missing watts")
	}
	if len(watts) != 3 {
		t.Fatalf("watts length = %d, want 3", len(watts))
	}
}

func TestNormalizeStreamsLengthMismatch(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{
			Type: "watts",
			Name: "Power",
			Data: []float64{100, 200, 300},
		},
		{
			Type: "heartrate",
			Name: "HR",
			Data: []float64{120, 130},
		},
	}

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("NormalizeStreams returned error: %v", err)
	}

	watts := got["watts"]
	hr := got["heartrate"]
	minLen := len(watts)
	if len(hr) < minLen {
		minLen = len(hr)
	}
	if len(watts) != minLen || len(hr) != minLen {
		t.Fatalf("expected aligned lengths: watts=%d, hr=%d, want both %d", len(watts), len(hr), minLen)
	}
}

func TestNormalizeStreamsDuplicateType(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "watts", Data: []float64{100, 200}},
		{Type: "watts", Data: []float64{300, 400}},
	}

	_, err := icu.NormalizeStreams(raw)
	if err == nil {
		t.Fatal("expected error for duplicate stream type, got nil")
	}
}

func TestNormalizeStreamsUnsupportedType(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "watts", Data: "not a slice"},
	}

	_, err := icu.NormalizeStreams(raw)
	if err == nil {
		t.Fatal("expected error for unsupported data type, got nil")
	}
}

func TestNormalizeStreamsEmptyType(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "", Data: []float64{100, 200}},
		{Type: "watts", Data: []float64{300, 400}},
	}

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := got[""]; ok {
		t.Fatal("empty-type stream should be skipped")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(got))
	}
}

func TestNormalizeStreamsNilData(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "watts", Data: nil},
		{Type: "heartrate", Data: []float64{100}},
	}

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 stream (nil data skipped), got %d", len(got))
	}
}

func TestNormalizeStreamsNullElementsBecomeZero(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "cadence", Data: []any{nil, float64(80)}},
		{Type: "heartrate", Data: []float64{100, 101}},
	}

	got, err := icu.NormalizeStreams(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	cadence, ok := got["cadence"]
	if !ok {
		t.Fatal("cadence stream with null samples should be kept")
	}
	if len(cadence) != 2 || cadence[0] != 0 || cadence[1] != 80 {
		t.Fatalf("cadence = %v, want [0 80]", cadence)
	}
}
