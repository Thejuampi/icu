package icu

import (
	"strconv"
	"testing"
	"time"
)

// Anonymized real shape of HRVRMSSD/real_data (value object with per-minute
// samples; s = ms offset from startTime). 0 and 255 are failed reads.
const zeppHRVRMSSDRealShape = `{"items":[
 {"timestamp":1793330400000,"eventType":"HRVRMSSD","subType":"real_data","userId":"anon",
  "value":{"startTime":1793330400000,"deviceId":"ANON","samples":[
   {"hrv":36,"s":0},{"hrv":0,"s":60000},{"hrv":38,"s":120000},{"hrv":255,"s":180000},{"hrv":40,"s":240000}]}},
 {"timestamp":1793416800000,"eventType":"HRVRMSSD","subType":"real_data",
  "value":{"startTime":1793416800000,"samples":[{"hrv":0,"s":0},{"hrv":-1,"s":60000}]}},
 {"timestamp":1793503200000,"eventType":"HRVRMSSD","subType":"real_data","value":{"startTime":1793503200000,"samples":[]}},
 {"timestamp":1793589600000,"eventType":"HRVRMSSD","subType":"real_data","value":"{\"startTime\":1793589600000,\"samples\":[{\"hrv\":\"44\",\"s\":0}]}"},
 {"timestamp":1793676000000,"value":"0"},
 {"timestamp":1793676000000,"value":0},
 {"timestamp":1793676000000}
]}`

func TestDecodeHRVRealShape(t *testing.T) {
	t.Parallel()
	got, err := DecodeHRV([]byte(zeppHRVRMSSDRealShape), "rmssd")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 valid nights (0/neg/empty/missing skipped), got %+v", got)
	}
	if got[0].Value != 38 || got[0].Samples != 3 || got[0].Timestamp != 1793330400000 || got[0].Source != "samples" {
		t.Fatalf("night 1 = %+v, want mean 38 of 3 valid samples", got[0])
	}
	if got[1].Value != 44 || got[1].Samples != 1 {
		t.Fatalf("JSON-in-string night = %+v", got[1])
	}
	for _, ev := range got {
		if ev.Value <= 0 {
			t.Fatalf("zero leaked: %+v", ev)
		}
	}
}

func TestDecodeHRVSDNNSampleKey(t *testing.T) {
	t.Parallel()
	raw := `{"items":[{"timestamp":1,"value":{"startTime":1793330400000,"samples":[{"sdnn":50,"s":0},{"sdnn":0,"s":60000}]}}]}`
	got, err := DecodeHRV([]byte(raw), "sdnn")
	if err != nil || len(got) != 1 || got[0].Value != 50 || got[0].Metric != "sdnn" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if got, _ := DecodeHRV([]byte(`{"items":[]}`), "sdnn"); got == nil || len(got) != 0 {
		t.Fatalf("empty items must be [] not null: %#v", got)
	}
}

// Anonymized real shape of readiness/watch_score: value object, sleepHRV =
// nightly rMSSD in ms, 255 = not measured.
func TestDecodeSleepHRV(t *testing.T) {
	t.Parallel()
	raw := `{"items":[
	 {"timestamp":1793347200000,"eventType":"readiness","subType":"watch_score","value":{"sleepHRV":38,"hrvBaseline":40,"hrvScore":66,"afibScore":255,"timestamp":1793332800000}},
	 {"timestamp":1793433600000,"value":{"sleepHRV":0}},
	 {"timestamp":1793520000000,"value":{"sleepHRV":255}},
	 {"timestamp":1793606400000,"value":{"hrvScore":50}},
	 {"timestamp":1793692800000,"value":{"sleepHRV":"44"}}
	]}`
	got, err := DecodeSleepHRV([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Value != 38 || got[0].Timestamp != 1793332800000 || got[1].Value != 44 || got[1].Timestamp != 1793692800000 {
		t.Fatalf("got %+v", got)
	}
}

func TestBuildZeppWellnessDaysRealShapeDayMapping(t *testing.T) {
	t.Parallel()
	ny := mustLoc(t, "America/New_York")
	// Sleep window starting 23:30 local on 10-24 belongs to 10-25.
	start := time.Date(2026, 10, 24, 23, 30, 0, 0, ny).UnixMilli()
	raw := `{"items":[{"timestamp":` + itoaTest(start) + `,"value":{"startTime":` + itoaTest(start) +
		`,"samples":[{"hrv":37,"s":0},{"hrv":0,"s":60000},{"hrv":39,"s":120000}]}}]}`
	samples, err := DecodeHRV([]byte(raw), "rmssd")
	if err != nil {
		t.Fatal(err)
	}
	got := BuildZeppWellnessDays(nil, samples, ny, "2026-10-25", "2026-10-26")
	if len(got) != 1 || got[0].Date != "2026-10-25" || got[0].HRV != 38 {
		t.Fatalf("got %+v", got)
	}

	// readiness sleepHRV (stamped local midnight of the scored day) beats
	// the sample mean for the same day, even when earlier.
	midnight := time.Date(2026, 10, 25, 0, 0, 0, 0, ny).UnixMilli()
	sleep := []ZeppHRVEvent{{Timestamp: midnight, Value: 38, Source: zeppHRVSourceSleep}}
	mixed := append(append([]ZeppHRVEvent{}, samples...), sleep...)
	mixed[0].Value = 41
	got = BuildZeppWellnessDays(nil, mixed, ny, "2026-10-25", "2026-10-25")
	if len(got) != 1 || got[0].HRV != 38 {
		t.Fatalf("sleepHRV must win, got %+v", got)
	}
	// Invalid readings never become values.
	bad := []ZeppHRVEvent{{Timestamp: midnight, Value: 0, Source: zeppHRVSourceSleep}, {Timestamp: midnight, Value: -5}, {Timestamp: midnight, Value: 9999}}
	if got := BuildZeppWellnessDays(nil, bad, ny, "2026-10-25", "2026-10-25"); len(got) != 0 {
		t.Fatalf("invalid leaked: %+v", got)
	}
}

func itoaTest(n int64) string { return strconv.FormatInt(n, 10) }
