package icu_test

import (
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestPreserveNullableStreamsKeepsNullsAbsent(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{{
		Type: "cadence",
		Data: []any{80.0, nil, 0.0},
	}}
	got, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		t.Fatalf("PreserveNullableStreams: %v", err)
	}
	series := got["cadence"]
	if series.Len() != 3 {
		t.Fatalf("len=%d, want 3", series.Len())
	}
	if _, ok := series.At(1); ok {
		t.Fatal("index 1 should be absent (null)")
	}
	v0, ok0 := series.At(0)
	if !ok0 || v0 != 80 {
		t.Fatalf("index 0 = %v present=%v, want 80 true", v0, ok0)
	}
	v2, ok2 := series.At(2)
	if !ok2 || v2 != 0 {
		t.Fatalf("index 2 = %v present=%v, want explicit zero present", v2, ok2)
	}
}

func TestPreserveNullableStreamsDenseValuesAllPresent(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{{
		Type: "watts",
		Data: []float64{100, 0, 200},
	}}
	got, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		t.Fatalf("PreserveNullableStreams: %v", err)
	}
	series := got["watts"]
	for index := range 3 {
		if !series.Present[index] {
			t.Fatalf("index %d should be present", index)
		}
	}
	if series.Values[1] != 0 {
		t.Fatalf("explicit zero became %v", series.Values[1])
	}
}

func TestPreserveNullableStreamsDuplicateTypeErrors(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "watts", Data: []float64{1}},
		{Type: "watts", Data: []float64{2}},
	}
	_, err := icu.PreserveNullableStreams(raw)
	if err == nil {
		t.Fatal("expected duplicate type error")
	}
}

func TestNullableSeriesDenseOrZeroFillsMissing(t *testing.T) {
	t.Parallel()

	series := icu.NullableSeries{
		Values:  []float64{10, 99, 20},
		Present: []bool{true, false, true},
	}
	dense := series.DenseOrZero()
	if dense[1] != 0 {
		t.Fatalf("missing sample dense=%v, want 0", dense[1])
	}
	if dense[0] != 10 || dense[2] != 20 {
		t.Fatalf("dense=%v", dense)
	}
}

func TestPreserveNullableStreamsEmptyAndIntTypes(t *testing.T) {
	t.Parallel()

	got, err := icu.PreserveNullableStreams(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty: got=%v err=%v", got, err)
	}

	raw := []icu.ActivityStream{
		{Type: "", Data: []float64{1}},
		{Type: "cadence", Data: []int{80, 90}},
		{Type: "watts", Data: []int64{100, 110}},
		{Type: "hr", Data: []float32{140, 141}},
	}
	got, err = icu.PreserveNullableStreams(raw)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got["cadence"].Values[1] != 90 {
		t.Fatalf("cadence=%v", got["cadence"].Values)
	}
	if got["watts"].Values[0] != 100 {
		t.Fatalf("watts=%v", got["watts"].Values)
	}
}

func TestPreserveNullableStreamsUnsupportedType(t *testing.T) {
	t.Parallel()

	_, err := icu.PreserveNullableStreams([]icu.ActivityStream{
		{Type: "watts", Data: "bad"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNullableSeriesAtBounds(t *testing.T) {
	t.Parallel()

	series := icu.NullableSeries{Values: []float64{1}, Present: []bool{true}}
	if _, ok := series.At(-1); ok {
		t.Fatal("negative index")
	}
	if _, ok := series.At(5); ok {
		t.Fatal("oob index")
	}
	empty := icu.NullableSeries{}
	if empty.DenseOrZero() != nil {
		t.Fatal("empty dense should be nil")
	}
}

func TestPreserveNullableStreamsInt32AndEmptyData(t *testing.T) {
	t.Parallel()

	raw := []icu.ActivityStream{
		{Type: "a", Data: []int32{1, 2, 3}},
		{Type: "b", Data: []any{}},
		{Type: "c", Data: nil},
	}
	got, err := icu.PreserveNullableStreams(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["a"].Values[2] != 3 {
		t.Fatalf("int32 series=%v", got["a"].Values)
	}
	if _, ok := got["b"]; ok {
		t.Fatal("empty series should be skipped")
	}
}

func TestPreserveNullableStreamsNonNumericAny(t *testing.T) {
	t.Parallel()

	_, err := icu.PreserveNullableStreams([]icu.ActivityStream{
		{Type: "watts", Data: []any{"x"}},
	})
	if err == nil {
		t.Fatal("expected non-numeric error")
	}
}

func TestNullableStreamNilMap(t *testing.T) {
	t.Parallel()

	series := icu.NullableStream(nil, "watts")
	if series.Len() != 0 {
		t.Fatalf("len=%d", series.Len())
	}
}
