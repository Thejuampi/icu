package icu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fixtures in testdata/zepp/real_shape keep the structure of a real
// readiness / HRVRMSSD / hrv_sdnn / Charge response (keys, nesting, types,
// 255 sentinels, empty hrv_sdnn) with every value synthetic and every
// identifier redacted. This file pins, fixture by fixture, what the decoders
// actually prove against those real shapes — and, just as importantly, what
// they do NOT prove (object `value` payloads, which the CLI cannot turn into
// a scalar without inventing a mapping).
func TestRealShapeReadinessDecode(t *testing.T) {
	t.Parallel()

	// SleepHRV: the nightly value the app shows. All 5 events are valid
	// (1..254), each stamped from value.timestamp, each metric "sleephrv".
	sleep, err := DecodeSleepHRV(readRealShape(t, "readiness.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sleep) != 5 {
		t.Fatalf("DecodeSleepHRV: got %d events, want 5", len(sleep))
	}
	want := []float64{41, 40, 36, 39, 47}
	for i, ev := range sleep {
		if ev.Value != want[i] {
			t.Errorf("event %d: value = %v, want %v", i, ev.Value, want[i])
		}
		if ev.Metric != "sleephrv" || ev.Source != zeppHRVSourceSleep {
			t.Errorf("event %d: metric/source = %q/%q, want sleephrv/%q", i, ev.Metric, ev.Source, zeppHRVSourceSleep)
		}
		if ev.Timestamp <= 0 {
			t.Errorf("event %d: timestamp not stamped", i)
		}
	}

	// Readiness: the real shape carries `value` as an object, so no scalar
	// score exists. Score must stay 0 and the full object must survive in
	// extra.value — never a guessed score.
	readiness, err := DecodeReadiness(readRealShape(t, "readiness.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(readiness) != 5 {
		t.Fatalf("DecodeReadiness: got %d events, want 5", len(readiness))
	}
	for i, ev := range readiness {
		if ev.Score != 0 {
			t.Errorf("event %d: score = %v, want 0 (object value has no scalar)", i, ev.Score)
		}
		value, ok := ev.Extra["value"].(map[string]any)
		if !ok {
			t.Errorf("event %d: extra.value missing or not an object: %#v", i, ev.Extra["value"])
			continue
		}
		if _, ok := value["sleepHRV"]; !ok {
			t.Errorf("event %d: extra.value lost sleepHRV: %#v", i, value)
		}
		if _, ok := value["rdnsScore"]; !ok {
			t.Errorf("event %d: extra.value lost rdnsScore: %#v", i, value)
		}
	}
}

func TestRealShapeHRVRMSSDDecode(t *testing.T) {
	t.Parallel()

	got, err := DecodeHRV(readRealShape(t, "hrv-rmssd.json"), "rmssd")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("DecodeHRV rmssd: got %d events, want 5", len(got))
	}
	wantValue := []float64{52.3, 59, 49, 45, 57}
	wantSamples := []int{3, 2, 3, 2, 3}
	for i, ev := range got {
		if ev.Value != wantValue[i] {
			t.Errorf("event %d: value = %v, want %v", i, ev.Value, wantValue[i])
		}
		if ev.Samples != wantSamples[i] {
			t.Errorf("event %d: samples = %d, want %d", i, ev.Samples, wantSamples[i])
		}
		if ev.Metric != "rmssd" || ev.Source != zeppHRVSourceSamples {
			t.Errorf("event %d: metric/source = %q/%q, want rmssd/%q", i, ev.Metric, ev.Source, zeppHRVSourceSamples)
		}
		if ev.Timestamp <= 0 {
			t.Errorf("event %d: timestamp not stamped", i)
		}
	}
}

func TestRealShapeHRVSDNNDecode(t *testing.T) {
	t.Parallel()

	// The real hrv_sdnn shape is empty (no items): the decoder must return
	// an empty slice, never a fabricated reading.
	got, err := DecodeHRV(readRealShape(t, "hrv-sdnn.json"), "sdnn")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("DecodeHRV sdnn: got %+v, want empty non-nil", got)
	}
}

func TestRealShapeHybridChargeDecode(t *testing.T) {
	t.Parallel()

	// HybridCharge/insight_data carries `value` as an object with samples.
	// No scalar score exists, so DecodeHybridCharge must report Score 0 and
	// keep the full object in extra.value.
	charge, err := DecodeHybridCharge(readRealShape(t, "hybridcharge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(charge) != 5 {
		t.Fatalf("DecodeHybridCharge: got %d events, want 5", len(charge))
	}
	for i, ev := range charge {
		if ev.Score != 0 {
			t.Errorf("event %d: score = %v, want 0 (object value has no scalar)", i, ev.Score)
		}
		value, ok := ev.Extra["value"].(map[string]any)
		if !ok {
			t.Errorf("event %d: extra.value missing or not an object: %#v", i, ev.Extra["value"])
			continue
		}
		if _, ok := value["samples"]; !ok {
			t.Errorf("event %d: extra.value lost samples: %#v", i, value)
		}
	}

	// The same shape must not yield HRV: HybridCharge carries no HRV.
	hrv, err := DecodeHRV(readRealShape(t, "hybridcharge.json"), "rmssd")
	if err != nil {
		t.Fatal(err)
	}
	if len(hrv) != 0 {
		t.Fatalf("DecodeHRV on HybridCharge: got %+v, want none", hrv)
	}
}

func TestRealShapeSleepFixtureIsSynthetic(t *testing.T) {
	t.Parallel()

	// The only sleep fixture is synthetic by design (it documents the
	// minutes-since-midnight convention with a wrap, not a real band_data
	// response). It must still decode, and it must carry the comment that
	// says so — if a real sleep fixture lands later, this test is where the
	// honesty note moves to.
	raw, err := readSleepFixture(t)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Comment string `json:"_comment"`
		Stages  []struct {
			Start int `json:"start"`
			Stop  int `json:"stop"`
			Mode  int `json:"mode"`
		} `json:"stage"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Comment == "" {
		t.Errorf("sleep fixture lost its _comment honesty marker")
	}
	if len(probe.Stages) == 0 {
		t.Errorf("sleep fixture has no stages")
	}
}

func readSleepFixture(t *testing.T) ([]byte, error) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "zepp", "sleep_cross_midnight.json"))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func TestRealShapeBodyBatteryDecode(t *testing.T) {
	t.Parallel()

	// The real backend serves an object with per-minute samples; no scalar
	// level exists, so Level must stay 0 and the object must survive.
	got, err := DecodeBodyBattery(readRealShape(t, "body-battery.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("DecodeBodyBattery: got %d events, want 1", len(got))
	}
	if got[0].Level != 0 {
		t.Errorf("level = %v, want 0 (object value has no scalar)", got[0].Level)
	}
	value, ok := got[0].Extra["value"].(map[string]any)
	if !ok {
		t.Fatalf("extra.value missing or not an object: %#v", got[0].Extra["value"])
	}
	samples, ok := value["samples"].([]any)
	if !ok || len(samples) != 2 {
		t.Fatalf("samples = %v, want 2 samples", value["samples"])
	}
	first, ok := samples[0].(map[string]any)
	if !ok {
		t.Fatalf("sample 0 = %#v, want object", samples[0])
	}
	if total, _ := first["total"].(float64); total != 50 {
		t.Errorf("sample 0 total = %v, want 50", first["total"])
	}
}

func TestRealShapeHealthSummaryDecode(t *testing.T) {
	t.Parallel()

	got, err := DecodeHealthSummary(readRealShape(t, "health-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("DecodeHealthSummary: got %d events, want 1", len(got))
	}
	if got[0].Value != 0 {
		t.Errorf("value = %v, want 0 (object value has no scalar)", got[0].Value)
	}
	value, ok := got[0].Extra["value"].(map[string]any)
	if !ok {
		t.Fatalf("extra.value missing or not an object: %#v", got[0].Extra["value"])
	}
	samples, ok := value["samples"].([]any)
	if !ok || len(samples) != 1 {
		t.Fatalf("samples = %v, want 1 sample", value["samples"])
	}
	first, ok := samples[0].(map[string]any)
	if !ok {
		t.Fatalf("sample 0 = %#v, want object", samples[0])
	}
	if steps, _ := first["totalSteps"].(float64); steps != 1234 {
		t.Errorf("sample 0 totalSteps = %v, want 1234", first["totalSteps"])
	}
}

func TestRealShapeStressMinuteDecode(t *testing.T) {
	t.Parallel()

	got, err := DecodeStressMinute(readRealShape(t, "stress-minute.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("DecodeStressMinute: got %d events, want 1", len(got))
	}
	if got[0].Stress != 0 {
		t.Errorf("stress = %v, want 0 (object value has no scalar)", got[0].Stress)
	}
	value, ok := got[0].Extra["value"].(map[string]any)
	if !ok {
		t.Fatalf("extra.value missing or not an object: %#v", got[0].Extra["value"])
	}
	samples, ok := value["samples"].([]any)
	if !ok || len(samples) != 2 {
		t.Fatalf("samples = %v, want 2 samples", value["samples"])
	}
	first, ok := samples[0].(map[string]any)
	if !ok {
		t.Fatalf("sample 0 = %#v, want object", samples[0])
	}
	if minute, _ := first["minuteOfDay"].(float64); minute != 480 {
		t.Errorf("sample 0 minuteOfDay = %v, want 480", first["minuteOfDay"])
	}
}

func TestRealShapeRespiratoryRateDecode(t *testing.T) {
	t.Parallel()

	got, err := DecodeRespiratoryRate(readRealShape(t, "respiratory-rate.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("DecodeRespiratoryRate: got %d events, want 1", len(got))
	}
	if got[0].Rate != 0 {
		t.Errorf("rate = %v, want 0 (object value has no scalar)", got[0].Rate)
	}
	value, ok := got[0].Extra["value"].(map[string]any)
	if !ok {
		t.Fatalf("extra.value missing or not an object: %#v", got[0].Extra["value"])
	}
	if _, ok := value["measurements"]; !ok {
		t.Errorf("extra.value lost measurements: %#v", value)
	}
}

func TestRealShapeHeartRateAppDecode(t *testing.T) {
	t.Parallel()

	// The real backend serves one flat item per window; the base64 payload
	// format is unverified, so HR stays 0 and every raw field is preserved.
	got, err := DecodeHeartRateEndpoint(readRealShape(t, "heart-rate-app.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("DecodeHeartRateEndpoint: got %d events, want 1", len(got))
	}
	// generatedTime is epoch seconds; the reading timestamp is milliseconds.
	if got[0].Timestamp != 123456789000 {
		t.Errorf("timestamp = %d, want 123456789000", got[0].Timestamp)
	}
	if got[0].HR != 0 {
		t.Errorf("HR = %v, want 0 (payload format unverified)", got[0].HR)
	}
	for _, key := range []string{"type", "generatedTime", "deviceSource", "timeZone", "heartRateData", "deviceId"} {
		if _, ok := got[0].Extra[key]; !ok {
			t.Errorf("extra lost %s: %#v", key, got[0].Extra)
		}
	}
}
