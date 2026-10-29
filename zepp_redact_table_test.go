package icu_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// redactOne redacts {key: value} and returns the output value as raw JSON.
func redactOne(t *testing.T, key string, value any) string {
	t.Helper()
	in, err := json.Marshal(map[string]any{key: value})
	if err != nil {
		t.Fatal(err)
	}
	out, err := icu.RedactZeppJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("invalid output %s: %v", out, err)
	}
	for _, v := range m {
		return string(v)
	}
	t.Fatalf("no value in %s", out)

	return ""
}

// Every key here must be redacted even with a numeric value (numbers are
// only kept under non-identifier keys). One row per rule so that removing a
// list entry or a suffix rule fails a named case.
func TestRedactDeniedKeysTable(t *testing.T) {
	t.Parallel()

	keys := []string{
		// identifiers
		"uid", "UID", "userId", "user_id", "USER-ID", "deviceId", "deviceIds", "id", "ids", "Id", "uuid", "guid",
		"iccid", "imsi", "imei", "IMEI", "serial", "serialNo", "serialNumber", "sn", "SN", "macAddr", "mac",
		"macAddress", "fooID", "foo_id", "foo-id", "fooid", "fooIds", "huamiId", "openid", "code",
		// personal data
		"birthday", "gender", "firstName", "lastName", "displayName", "fullname", "nick", "nickname", "alias",
		"mobile", "tel", "phone", "email", "email_addr", "EMAIL", "name", "userName",
		// location
		"lat", "lon", "latitude", "longitude", "gps", "location", "address", "ip", "ssid", "bssid", "timezone",
		// secrets
		"token", "appToken", "login_token", "Refresh-Token", "access", "refresh", "auth", "Authorization",
		"signature", "sign", "key", "apikey", "api_key", "secret", "password", "session", "sessionId",
		"credential", "cookie",
		// devices and links
		"deviceName", "avatar", "avatarUrl", "photo", "url", "account", "accountName",
	}
	for _, k := range keys {
		if got := redactOne(t, k, 123456); got != `"[redacted]"` {
			t.Errorf("%q: number kept: %s", k, got)
		}
		if got := redactOne(t, k, "x"); got != `"[redacted]"` {
			t.Errorf("%q: string kept: %s", k, got)
		}
		if got := redactOne(t, k, map[string]any{"hrv": 1}); got != `"[redacted]"` {
			t.Errorf("%q: object kept: %s", k, got)
		}
	}
}

// The HRV, readiness and skin-temp payload keys must survive.
func TestRedactKeepsPayloadKeys(t *testing.T) {
	t.Parallel()

	for _, k := range []string{
		"sleepHRV", "hrv", "sdnn", "rmssd", "samples", "s", "value", "startTime", "timestamp",
		"date", "score", "skinTemp", "skinTempCalibrated", "skinTempBaseLine", "skinTempScore", "valid", "paid",
		"hybrid",
	} {
		if icu.IsZeppSecretKey(k) {
			t.Errorf("%q must not be a secret key", k)
		}
		if got := redactOne(t, k, 38); got != "38" {
			t.Errorf("%q: number lost: %s", k, got)
		}
	}
	for _, k := range []string{"date", "timestamp", "startTime", "eventType", "subType", "type", "value", "skinTempInsight"} {
		if got := redactOne(t, k, "2026-10-29"); got != `"2026-10-29"` {
			t.Errorf("%q: safe string lost: %s", k, got)
		}
	}
	if got := redactOne(t, "valid", true); got != "true" {
		t.Errorf("bool lost: %s", got)
	}
}

// Strings under keys outside the allowlist are always redacted.
func TestRedactUnknownStringKeys(t *testing.T) {
	t.Parallel()

	for _, k := range []string{"note", "message", "insight", "label", "desc"} {
		if got := redactOne(t, k, "hello"); got != `"[redacted]"` {
			t.Errorf("%q: %s", k, got)
		}
	}
}

func TestRedactHostileValues(t *testing.T) {
	t.Parallel()

	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJTWU4ifQ.c2lnbmF0dXJl"
	b64 := base64.StdEncoding.EncodeToString([]byte(`{"userId":"SYN-B64-USER","hrv":40}`))
	cases := map[string]struct{ key, value string }{
		"email under safe key":       {"type", "syn@example.com"},
		"jwt in note":                {"note", jwt},
		"jwt under safe key":         {"value", jwt},
		"userId in a URL":            {"value", "https://example.com/u?userId=SYN-URL-USER"},
		"url without query":          {"type", "ftp://example.com/x"},
		"mac":                        {"type", "AA:BB:CC:DD:EE:FF"},
		"mac dashes":                 {"value", "aa-bb-cc-dd-ee-ff"},
		"uuid":                       {"value", "123e4567-e89b-12d3-a456-426614174000"},
		"uuid no dashes":             {"value", "123e4567e89b12d3a456426614174000"},
		"base64 json":                {"value", b64},
		"long hex":                   {"type", "a1b2c3d4e5f6a7b8"},
		"looks like json, invalid":   {"value", `{"userId":"SYN-BAD"`},
		"array-ish invalid":          {"value", `[1,"SYN-BAD"`},
		"quoted-ish invalid":         {"value", `"SYN-BAD`},
		"email in unknown key":       {"insight", "mail syn@example.com now"},
		"embedded email in safe key": {"value", "a syn@example.com"},
	}
	for name, c := range cases {
		if got := redactOne(t, c.key, c.value); got != `"[redacted]"` {
			t.Errorf("%s: %s", name, got)
		}
	}
}

func TestRedactEmbeddedJSON(t *testing.T) {
	t.Parallel()

	inner := `{"userId":"SYN-IN-USER","hrv":41,"note":"syn@example.com"}`
	double, err := json.Marshal(inner) // a JSON string holding JSON
	if err != nil {
		t.Fatal(err)
	}
	arr := `[{"deviceId":"SYN-IN-DEV","sdnn":52},7]`

	raw := map[string]string{
		"value":  inner,
		"double": string(double),
		"arr":    arr,
		"type":   "a<b&c>d",
	}
	in, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := icu.RedactZeppJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, leak := range []string{"SYN-IN-USER", "SYN-IN-DEV", "syn@example.com"} {
		if strings.Contains(s, leak) {
			t.Fatalf("%s leaked: %s", leak, s)
		}
	}
	// JSON object in a string, double-encoded JSON and a JSON array in a
	// string are unwrapped: their safe numbers survive.
	for _, keep := range []string{`\"hrv\":41`, `\"sdnn\":52`, `,7]`, `a<b&c>d`} {
		if !strings.Contains(s, keep) {
			t.Fatalf("missing %s in %s", keep, s)
		}
	}
	var back map[string]string
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	var d string
	if err := json.Unmarshal([]byte(back["double"]), &d); err != nil || !strings.Contains(d, `"hrv":41`) {
		t.Fatalf("double-encoded not kept as a JSON string: %q %v", back["double"], err)
	}
}

func TestRedactNumericAndHexKeys(t *testing.T) {
	t.Parallel()

	in := `{"items":{"123456789":{"hrv":40},"deadbeef01":{"hrv":41},"7":1,"beef":2}}`
	out, err := icu.RedactZeppJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, leak := range []string{"123456789", "deadbeef01", `"7"`} {
		if strings.Contains(s, leak) {
			t.Fatalf("%s key leaked: %s", leak, s)
		}
	}
	for _, keep := range []string{`"[redacted-key-1]":{"hrv":40}`, `"[redacted-key-2]":{"hrv":41}`, `"[redacted-key-3]":1`, `"beef":2`} {
		if !strings.Contains(s, keep) {
			t.Fatalf("missing %s in %s", keep, s)
		}
	}
}

func TestRedactDepthLimit(t *testing.T) {
	t.Parallel()

	deep := strings.Repeat("[", 200) + strings.Repeat("]", 200)
	if _, err := icu.RedactZeppJSON([]byte(deep)); err == nil {
		t.Fatal("expected depth error")
	}
}
