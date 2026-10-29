package icu

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func timestampDateUTC(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.DateOnly)
}

// Zepp HRV shapes. Sources (public code; never our own calls to the real API):
//
//   - HRVRMSSD real_data: each item's "value" is an OBJECT
//     {"startTime": <ms>, "deviceId": ..., "samples": [{"hrv": 41, "s": 0}, ...]}
//     where "s" is a millisecond offset from startTime. See
//     github.com/lingcang728/ZeppBridge @1012a00,
//     src-tauri/crates/core/src/normalizer/metrics.rs L111-L145
//     (hrv_rmssd_samples). hrv_sdnn uses "sdnn" as the sample key. The
//     eventType/subType pairs (HRVRMSSD/real_data, hrv_sdnn/real_data) are in
//     github.com/m4ary/zepp-health-cli @a466dfa, zepp_health.py L703-L704.
//     Older or other payloads may carry the value as a JSON string or a number.
//   - readiness watch_score: "value" is an object whose "sleepHRV" is the
//     nightly rMSSD in ms, the value the Zepp app shows; 0/255 mean "not
//     measured". Confirmed against the structure of a real (redacted)
//     response read offline; testdata/zepp/real_shape keeps that structure
//     with synthetic values. hrv_sdnn came back empty there and HybridCharge
//     (Charge/insight_data) carries no HRV, so neither is an HRV source.
//
// Zero, negative, non-finite and out-of-range readings are failed reads,
// never values.
const (
	zeppHRVMin = 1.0
	// 255 (0xFF) is Zepp's single-byte "not measured" sentinel, so the
	// valid range stops at 254.
	zeppHRVMax           = 254.0
	zeppHRVSourceSamples = "samples"
	zeppHRVSourceValue   = "value"
	zeppHRVSourceSleep   = "readiness.sleepHRV"
)

// ValidZeppHRV reports whether v is a usable HRV reading in ms.
func ValidZeppHRV(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= zeppHRVMin && v <= zeppHRVMax
}

type zeppHRVItem struct {
	Timestamp json.RawMessage `json:"timestamp"`
	Value     json.RawMessage `json:"value"`
}

type zeppHRVValue struct {
	StartTime json.RawMessage   `json:"startTime"`
	Samples   []json.RawMessage `json:"samples"`
	SleepHRV  json.RawMessage   `json:"sleepHRV"`
	Timestamp json.RawMessage   `json:"timestamp"`
	// Flat keys some payloads use instead of samples.
	HRV   json.RawMessage `json:"hrv"`
	RMSSD json.RawMessage `json:"rmssd"`
	SDNN  json.RawMessage `json:"sdnn"`
}

// rawNumber parses a JSON number or a numeric JSON string; ok=false otherwise.
func rawNumber(raw json.RawMessage) (float64, bool) {
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f, true
	}
	var str string
	if json.Unmarshal(raw, &str) != nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(str), 64)

	return f, err == nil
}

// unwrapJSONString returns the inner JSON when raw is a string holding an
// object (Zepp sometimes nests JSON-in-string).
func unwrapJSONString(raw json.RawMessage) json.RawMessage {
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" && s[0] == '{' {
		return json.RawMessage(s)
	}

	return raw
}

func sampleKeys(metric string) []string {
	if metric == "sdnn" {
		return []string{"sdnn", "hrv", "value"}
	}

	return []string{"hrv", "rmssd", "value"}
}

// DecodeHRV decodes a raw HRVRMSSD / hrv_sdnn V2 events response. Each item
// yields at most one event: the mean of its valid per-minute samples (or its
// valid scalar value), stamped at the sample window start. Items without a
// valid reading are skipped.
func DecodeHRV(raw []byte, metric string) ([]ZeppHRVEvent, error) {
	if len(raw) == 0 {
		return []ZeppHRVEvent{}, nil
	}
	var payload struct {
		Items []zeppHRVItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode hrv: %w", err)
	}

	out := make([]ZeppHRVEvent, 0, len(payload.Items))
	for _, item := range payload.Items {
		ts, _ := rawNumber(item.Timestamp)
		if ev, ok := decodeHRVItem(int64(ts), unwrapJSONString(item.Value), metric); ok {
			out = append(out, ev)
		}
	}

	return out, nil
}

func decodeHRVItem(itemTS int64, value json.RawMessage, metric string) (ZeppHRVEvent, bool) {
	ev := ZeppHRVEvent{Timestamp: itemTS, Metric: metric}
	if v, ok := rawNumber(value); ok {
		ev.Value, ev.Source = v, zeppHRVSourceValue

		return finishHRV(ev)
	}

	var obj zeppHRVValue
	if err := json.Unmarshal(value, &obj); err != nil {
		return ev, false
	}
	if start, ok := rawNumber(obj.StartTime); ok && start > 0 {
		ev.Timestamp = int64(start)
	}
	if len(obj.Samples) > 0 {
		sum, n := 0.0, 0
		for _, s := range obj.Samples {
			var m map[string]json.RawMessage
			if json.Unmarshal(s, &m) != nil {
				continue
			}
			for _, k := range sampleKeys(metric) {
				if v, ok := rawNumber(m[k]); ok {
					if ValidZeppHRV(v) {
						sum += v
						n++
					}

					break
				}
			}
		}
		if n == 0 {
			return ev, false
		}
		ev.Value = math.Round(sum/float64(n)*10) / 10 //nolint:mnd // one decimal
		ev.Samples, ev.Source = n, zeppHRVSourceSamples

		return finishHRV(ev)
	}
	for _, f := range []json.RawMessage{obj.RMSSD, obj.SDNN, obj.HRV} {
		if v, ok := rawNumber(f); ok {
			ev.Value, ev.Source = v, zeppHRVSourceValue

			return finishHRV(ev)
		}
	}

	return ev, false
}

func finishHRV(ev ZeppHRVEvent) (ZeppHRVEvent, bool) {
	if ev.Timestamp <= 0 || !ValidZeppHRV(ev.Value) {
		return ev, false
	}
	ev.Date = timestampDateUTC(ev.Timestamp)

	return ev, true
}

// DecodeSleepHRV extracts readiness watch_score value.sleepHRV (Zepp's
// nightly rMSSD) as HRV events. 0 and the 255 sentinel are skipped. The
// event is stamped at value.timestamp (local midnight of the scored day)
// when present, else the item timestamp (the morning update).
func DecodeSleepHRV(raw []byte) ([]ZeppHRVEvent, error) {
	if len(raw) == 0 {
		return []ZeppHRVEvent{}, nil
	}
	var payload struct {
		Items []zeppHRVItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode readiness sleepHRV: %w", err)
	}
	out := make([]ZeppHRVEvent, 0, len(payload.Items))
	for _, item := range payload.Items {
		var obj zeppHRVValue
		if json.Unmarshal(unwrapJSONString(item.Value), &obj) != nil {
			continue
		}
		v, ok := rawNumber(obj.SleepHRV)
		if !ok || !ValidZeppHRV(v) {
			continue
		}
		ts, _ := rawNumber(obj.Timestamp)
		if ts <= 0 {
			ts, _ = rawNumber(item.Timestamp)
		}
		ev := ZeppHRVEvent{Timestamp: int64(ts), Metric: "rmssd", Value: v, Source: zeppHRVSourceSleep}
		if ev, ok := finishHRV(ev); ok {
			out = append(out, ev)
		}
	}

	return out, nil
}
