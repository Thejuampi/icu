package icu_test

import (
	"encoding/json"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestRedactSampleOffsetNineDigits(t *testing.T) {
	t.Parallel()

	for _, v := range []any{int64(123456789), "123456789", int64(999999999), int64(1234567890)} {
		if got := redactOne(t, "s", v); got != `"[redacted]"` {
			t.Errorf("s=%v kept: %s", v, got)
		}
	}
	for _, v := range []any{int64(0), int64(180000), int64(12345678)} {
		if got := redactOne(t, "s", v); got == `"[redacted]"` {
			t.Errorf("s=%v (offset) lost", v)
		}
	}
	// Nested as in HRVRMSSD samples.
	out, err := icu.RedactZeppJSON([]byte(`{"items":[{"value":{"samples":[{"s":123456789,"hrv":41},{"s":180000,"hrv":40}]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "123456789") || !strings.Contains(string(out), `"s":180000`) || !strings.Contains(string(out), `"hrv":41`) {
		t.Fatalf("nested samples: %s", out)
	}
}

func TestRedactDeviceIdentityFields(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"deviceSource", "device_source", "DEVICESOURCE", "deviceType", "deviceModel", "deviceVersion", "devSourceId",
		"firmwareVersion", "hardwareVersion", "hwVersion", "fwVer", "productId", "productVersion", "model",
		"vendorId", "brand", "sourceId", "bluetoothAddr",
	} {
		for _, v := range []any{int64(1234567), "1234567", int64(42), "Model X"} {
			if got := redactOne(t, key, v); got != `"[redacted]"` {
				t.Errorf("%s=%v kept: %s", key, v, got)
			}
		}
	}
	// Metrics next to them survive.
	for key, v := range map[string]any{"sleepHRV": int64(41), "hrv": int64(40), "score": int64(77)} {
		if got := redactOne(t, key, v); got == `"[redacted]"` {
			t.Errorf("%s lost", key)
		}
	}
}

// realShapeEvents mirrors the structure of real readiness, HRVRMSSD and
// HybridCharge responses with synthetic values, and plants numeric ids in
// every identity field seen there.
const realShapeEvents = `{"items":[
 {"userId":7000001,"eventType":"readiness","subType":"watch_score","timestamp":1894622400000,
  "value":{"rdnsInsight":16,"hrvInsight":255,"insightId":"7000002","deviceId":7000003,"deviceSource":7000004,
   "timestamp":1894597200000,"timestampUpdate":1894622400000,"timezoneId":"Etc/Fake","status":200,
   "sleepHRV":41,"sleepRHR":52,"algVer":4,"hrvBaseline":40,"skinTempBaseLine":3}},
 {"userId":"7000005","eventType":"HRVRMSSD","subType":"real_data","timestamp":1894579200000,
  "value":{"startTime":1894590000000,"deviceId":"7000006","deviceSN":"7000007","deviceSource":"7000008",
   "deviceType":7000009,"timeZone":"Etc/Fake","samples":[{"s":0,"u":512,"hrv":48},{"s":180000,"u":513,"hrv":51}]}},
 {"userId":7000010,"eventType":"Charge","subType":"insight_data","timestamp":1894579200000,
  "value":{"startTime":1894597200000,"deviceId":7000011,"deviceSN":7000012,"deviceSource":7000013,"deviceType":"7000014",
   "samples":[{"s":0,"u":600,"type":4,"insight":30,"insightId":7000015,"trackId":7000016,
    "jsonExtra":"{\"hcInsightId\":7000017,\"finalInsightId\":\"7000018\",\"minsBelowThreshold\":0,\"deviceSource\":7000019}"}]}}
]}`

// numericIDSurvivors returns every integer of 6+ digits left outside a time
// key or a sample offset, plus every planted 70000xx id, after redaction.
func numericIDSurvivors(t *testing.T, raw []byte) []string {
	t.Helper()
	var out []string
	var walk func(key string, v any)
	walk = func(key string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				walk(k, c)
			}
		case []any:
			for _, c := range x {
				walk(key, c)
			}
		case string:
			if inner, ok := decodeNumbers(x); ok && strings.HasPrefix(x, "{") {
				walk(key, inner)
			} else if strings.Contains(x, "70000") {
				out = append(out, key+"="+x)
			}
		case json.Number:
			k := strings.ToLower(key)
			if strings.Contains(x.String(), "70000") ||
				(len(strings.TrimLeft(x.String(), "-")) >= 6 && !strings.Contains(k, "time") && k != "s") {
				out = append(out, key+"="+x.String())
			}
		}
	}
	v, ok := decodeNumbers(string(raw))
	if !ok {
		t.Fatalf("invalid JSON: %s", raw)
	}
	walk("", v)

	return out
}

func TestRedactRealShapeLeavesNoNumericIDs(t *testing.T) {
	t.Parallel()

	if got := numericIDSurvivors(t, []byte(realShapeEvents)); len(got) < 19 {
		t.Fatalf("scanner is blind: found %d planted ids before redaction: %v", len(got), got)
	}
	out, err := icu.RedactZeppJSON([]byte(realShapeEvents))
	if err != nil {
		t.Fatal(err)
	}
	if got := numericIDSurvivors(t, out); len(got) != 0 {
		t.Fatalf("numeric ids survived redaction: %v\n%s", got, out)
	}
	for _, keep := range []string{`"sleepHRV":41`, `"hrv":48`, `"eventType":"readiness"`, `"subType":"watch_score"`, `"timestamp":1894597200000`, `"s":180000`} {
		if !strings.Contains(string(out), keep) {
			t.Errorf("lost %s: %s", keep, out)
		}
	}
}

func decodeNumbers(s string) (any, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any

	return v, dec.Decode(&v) == nil
}
