package icu_test

import (
	"encoding/json"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// Synthetic payload: every identifier below is fake.
const redactFixture = `{"items":[{"timestamp":1761710400000,"eventType":"Charge","subType":"insight_data","userId":"SYN-USER-1",` +
	`"value":{"wakeHRV":38,"DeviceID":"SYN-DEV-1","nested":[{"SN":"SYN-SN-1","s":0,"hrv":41}],` +
	`"inner":"{\"user_id\":\"SYN-USER-2\",\"apptoken\":\"SYN-TOK-1\",\"x\":1.50}"},` +
	`"APP_TOKEN":"SYN-TOK-2","loginToken":"SYN-TOK-3","refresh-token":"SYN-TOK-4","eventId":"SYN-EV-1",` +
	`"Email":"syn@example.com","macAddress":"SYN-MAC","valid":true}],"next":-1}`

func TestRedactZeppJSONRemovesIdentifiersAtAnyDepth(t *testing.T) {
	t.Parallel()

	out, err := icu.RedactZeppJSON([]byte(redactFixture))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	for _, secret := range []string{
		"SYN-USER-1", "SYN-USER-2", "SYN-DEV-1", "SYN-SN-1", "SYN-TOK-1", "SYN-TOK-2",
		"SYN-TOK-3", "SYN-TOK-4", "SYN-EV-1", "syn@example.com", "SYN-MAC",
	} {
		if strings.Contains(s, secret) {
			t.Fatalf("%s leaked: %s", secret, s)
		}
	}
	// Payload values, key order and number formatting are untouched.
	for _, keep := range []string{
		`"wakeHRV":38`, `"hrv":41`, `\"x\":1.50`, `"valid":true`, `"next":-1`,
		`"timestamp":1761710400000`, `"subType":"insight_data"`,
	} {
		if !strings.Contains(s, keep) {
			t.Fatalf("missing %s in %s", keep, s)
		}
	}
	if strings.Index(s, `"timestamp"`) > strings.Index(s, `"userId"`) {
		t.Fatalf("key order changed: %s", s)
	}
	if !json.Valid(out) {
		t.Fatalf("not valid JSON: %s", s)
	}
}

func TestIsZeppSecretKeyCaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, k := range []string{
		"userId", "USERID", "user_id", "deviceId", "device-id", "sn", "SN", "token", "appToken",
		"APPTOKEN", "login_token", "Authorization", "id", "Id", "eventId", "serialNumber",
	} {
		if !icu.IsZeppSecretKey(k) {
			t.Errorf("%q should be redacted", k)
		}
	}
	for _, k := range []string{"value", "hrv", "samples", "timestamp", "valid", "sleepHRV", "score", "subType", "s"} {
		if icu.IsZeppSecretKey(k) {
			t.Errorf("%q should be kept", k)
		}
	}
}

func TestRedactZeppJSONRejectsNonJSON(t *testing.T) {
	t.Parallel()

	if _, err := icu.RedactZeppJSON([]byte("userId=SYN")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := icu.RedactZeppJSON([]byte(`{"a":1} {"userId":"SYN"}`)); err == nil {
		t.Fatal("expected trailing-data error")
	}
}

func TestDecodeV2EventsKeepsObjectPayload(t *testing.T) {
	t.Parallel()

	raw := `{"items":[{"timestamp":1761710400000,"eventType":"readiness","subType":"watch_score",` +
		`"value":{"sleepHRV":38,"skinTempCalibrated":-12}},` +
		`{"timestamp":1761710400000,"eventType":"Charge","subType":"insight_data","value":"{\"wakeHRV\":44}"},` +
		`{"timestamp":1761710400000,"eventType":"x","value":"57"}]}`

	events, err := icu.DecodeV2Events([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	v0, _ := events[0].Extra["value"].(map[string]any)
	if v0["sleepHRV"] != float64(38) {
		t.Fatalf("readiness payload dropped: %+v", events[0])
	}
	v1, _ := events[1].Extra["value"].(map[string]any)
	if v1["wakeHRV"] != float64(44) {
		t.Fatalf("json-in-string payload dropped: %+v", events[1])
	}
	if events[2].Value != 57 || events[2].Extra["value"] != nil {
		t.Fatalf("numeric string: %+v", events[2])
	}

	ready, err := icu.DecodeReadiness([]byte(raw))
	if err != nil || ready[0].Extra["value"] == nil {
		t.Fatalf("readiness extra: %+v %v", ready, err)
	}
}
