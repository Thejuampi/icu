package icu_test

import (
	"encoding/json"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// The API sends snake_case; camelCase (this package's own output) must
// still round-trip. Synthetic values only.
func TestIntervalsDTODecodesSnakeAndCamel(t *testing.T) {
	t.Parallel()
	snake := `{"id":"i970000003","icu_intervals":[{"id":7,"type":"WORK","start_index":10,"end_index":70,"start_time":10,` +
		`"end_time":70,"moving_time":60,"elapsed_time":61,"average_watts":250,"weighted_average_watts":252,"max_watts":300,` +
		`"average_heartrate":150,"max_heartrate":158,"decoupling":3.3,"training_load":2.5,"average_cadence":90,` +
		`"total_elevation_gain":4,"group_id":"g-syn-2","label":"syn"}],` +
		`"icu_groups":[{"id":"g-syn-2","count":1,"moving_time":60,"elapsed_time":61,"average_watts":250,` +
		`"weighted_average_watts":252,"max_watts":300,"average_heartrate":150,"max_heartrate":158,"decoupling":3.3,` +
		`"training_load":2.5,"start_index":10,"end_index":70}]}`
	var dto icu.IntervalsDTO
	if err := json.Unmarshal([]byte(snake), &dto); err != nil {
		t.Fatal(err)
	}
	want := icu.Interval{
		ID: 7, Label: "syn", StartTime: 10, EndTime: 70, ElapsedTime: 61, WeightedPower: 252, Decoupling: 3.3,
		StartIndex: 10, EndIndex: 70, Type: "WORK", MovingTime: 60, AvgPower: 250, MaxPower: 300, AvgHR: 150,
		MaxHR: 158, TrainingLoad: 2.5, AvgCadence: 90, ElevationGain: 4, GroupID: "g-syn-2",
	}
	if len(dto.Intervals) != 1 || dto.Intervals[0] != want {
		t.Fatalf("intervals %+v\nwant %+v", dto.Intervals, want)
	}
	wantG := icu.IntervalGroup{
		ID: "g-syn-2", Count: 1, MovingTime: 60, ElapsedTime: 61, AvgPower: 250, WeightedPower: 252, MaxPower: 300,
		AvgHR: 150, MaxHR: 158, Decoupling: 3.3, TrainingLoad: 2.5, StartIndex: 10, EndIndex: 70,
	}
	if len(dto.Groups) != 1 || dto.Groups[0] != wantG {
		t.Fatalf("groups %+v\nwant %+v", dto.Groups, wantG)
	}
	camel, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var back icu.IntervalsDTO
	if err := json.Unmarshal(camel, &back); err != nil || back.Intervals[0] != want || back.Groups[0] != wantG {
		t.Fatalf("camel round trip %+v err=%v", back, err)
	}
	if err := json.Unmarshal([]byte(`{"icu_intervals":5}`), &dto); err != nil {
		t.Fatalf("bad snake value must be ignored, got %v", err)
	}
	for _, bad := range []string{`[`, `{"icu_intervals":[[`} {
		var i icu.IntervalsDTO
		if json.Unmarshal([]byte(bad), &i) == nil {
			t.Fatalf("%s must fail", bad)
		}
	}
}
