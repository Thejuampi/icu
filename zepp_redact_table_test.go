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
	kept := []struct{ key, value string }{
		{"date", "2026-10-29"},
		{"timestamp", "2026-10-29T06:30:00Z"},
		{"startTime", "1761710400000"},
		{"time", "2026-10-29 06:30"},
		{"eventType", "readiness"},
		{"subType", "watch_score"},
		{"type", "HRVRMSSD"},
		{"unit", "ms"},
		{"status", "ok"},
		{"value", "57"},
		{"hrv", "38"},
		{"sleepHRV", "37.5"},
		{"skinTempCalibrated", "-12"},
		{"score", "0.85"},
	}
	for _, c := range kept {
		if got := redactOne(t, c.key, c.value); got != `"`+c.value+`"` {
			t.Errorf("%q=%q: safe string lost: %s", c.key, c.value, got)
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

	raw := map[string]any{
		"value":  inner,
		"double": string(double),
		"arr":    arr,
		"a<b&c>": 5,
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
	for _, keep := range []string{`\"hrv\":41`, `\"sdnn\":52`, `,7]`, `"a<b&c>":5`} {
		if !strings.Contains(s, keep) {
			t.Fatalf("missing %s in %s", keep, s)
		}
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	dbl, _ := back["double"].(string)
	var d string
	if err := json.Unmarshal([]byte(dbl), &d); err != nil || !strings.Contains(d, `"hrv":41`) {
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

// Free text under safe keys must have the key's shape; all example values
// are obviously fake.
func TestRedactShapeRestrictedSafeKeys(t *testing.T) {
	t.Parallel()

	cases := []struct{ key, value string }{
		{"status", "Testy Mc Testface"},
		{"result", "+00 00 0000-0000"},
		{"unit", "0000000000000"},
		{"subType", "Fake Street 0000 Nowhere"},
		{"type", "a<b"},
		{"type", strings.Repeat("x", 33)},
		{"eventType", "x1"},
		{"date", "yesterday"},
		{"date", "2026-10-29 at the park"},
		{"timestamp", "12345678901234"},
		{"value", "1234567"},
		{"value", "55555551234"},
		{"hrv", "38 ms"},
		{"status", "192.0.2.10"},
		{"value", "203.0.113.7"},
		{"unit", "2001:db8::1"},
	}
	for _, c := range cases {
		if got := redactOne(t, c.key, c.value); got != `"[redacted]"` {
			t.Errorf("%q=%q kept: %s", c.key, c.value, got)
		}
	}
}

func TestRedactArraysHaveNoStringAllowlist(t *testing.T) {
	t.Parallel()

	if got := redactOne(t, "samples", []any{"Testy", "192.0.2.10", "57"}); got != `["[redacted]","[redacted]","[redacted]"]` {
		t.Fatalf("strings in array kept: %s", got)
	}
	if got := redactOne(t, "samples", []any{41, 38.5, -2}); got != `[41,38.5,-2]` {
		t.Fatalf("numeric array changed: %s", got)
	}
}

func TestRedactLongIntegers(t *testing.T) {
	t.Parallel()

	for _, k := range []string{"note", "insight", "samples", "hrv", "value"} {
		if got := redactOne(t, k, int64(123456789)); got != `"[redacted]"` {
			t.Errorf("%q: 9-digit int kept: %s", k, got)
		}
	}
	if got := redactOne(t, "insight", 12345678); got != "12345678" {
		t.Errorf("8-digit int lost: %s", got)
	}
	if got := redactOne(t, "insight", 12345678.5); got != "12345678.5" {
		t.Errorf("8-digit decimal lost: %s", got)
	}
	for _, k := range []string{"timestamp", "startTime", "date"} {
		if got := redactOne(t, k, int64(1761710400000)); got != "1761710400000" {
			t.Errorf("%q: epoch lost: %s", k, got)
		}
		if got := redactOne(t, k, int64(12345678901234)); got != `"[redacted]"` {
			t.Errorf("%q: 14-digit kept: %s", k, got)
		}
	}
}

func TestRedactNewFragments(t *testing.T) {
	t.Parallel()

	for _, k := range []string{
		"birthDate", "dob", "zipCode", "postcode", "city", "countryCode", "region", "geoHash",
		"coords", "position", "ownerName", "friendList", "memberSince", "wearer", "hostName", "msisdn", "ip",
		"clientIp", "ip_addr", "ipv4", "userIdentifier", "USER_IDENTIFIER",
	} {
		if got := redactOne(t, k, 7); got != `"[redacted]"` {
			t.Errorf("%q kept: %s", k, got)
		}
	}
	// "ip" is exact, not a fragment/suffix: these stay.
	for _, k := range []string{"tip", "trip", "chip", "skip"} {
		if icu.IsZeppSecretKey(k) {
			t.Errorf("%q should not be a secret key", k)
		}
	}
}

// Floats and exponents count the digits of their integer part.
func TestRedactLongFloatsAndExponents(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"123456789.0", "1.23456789e8", "1E9", "-123456789.5", "12345678901234567890123", "1e400"} {
		out, err := icu.RedactZeppJSON([]byte(`{"insight":` + raw + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != `{"insight":"[redacted]"}` {
			t.Errorf("%s kept: %s", raw, out)
		}
	}
	for _, raw := range []string{"99999999.9", "1.5e3", "-12.25", "0"} {
		out, err := icu.RedactZeppJSON([]byte(`{"insight":` + raw + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != `{"insight":`+raw+`}` {
			t.Errorf("%s lost: %s", raw, out)
		}
	}
	// A float epoch under a time key is not an integer epoch.
	if out, _ := icu.RedactZeppJSON([]byte(`{"timestamp":1761710400000.5}`)); string(out) != `{"timestamp":"[redacted]"}` {
		t.Errorf("float epoch kept: %s", out)
	}
	if out, _ := icu.RedactZeppJSON([]byte(`{"timestamp":1.7617104e12}`)); string(out) != `{"timestamp":1.7617104e12}` {
		t.Errorf("exponent epoch lost: %s", out)
	}
}

func TestRedactTimeKeysEpochRangeAndClock(t *testing.T) {
	t.Parallel()

	keep := []struct {
		key string
		val any
	}{
		{"timestamp", "1761710400"},
		{"timestamp", "1761710400000"},
		{"startTime", int64(1761710400)},
		{"startTime", int64(1761710400000)},
		{"time", "07:12"},
		{"time", "23:59:59"},
		{"endTime", "00:00"},
		{"date", "2026-10-29"},
	}
	for _, c := range keep {
		want, err := json.Marshal(c.val) //nolint:errchkjson // string or int64 only
		if err != nil {
			t.Fatal(err)
		}
		if got := redactOne(t, c.key, c.val); got != string(want) {
			t.Errorf("%s=%v lost: %s", c.key, c.val, got)
		}
	}
	drop := []struct {
		key string
		val any
	}{
		{"timestamp", "123456789"},
		{"timestamp", "12345678901"},
		{"timestamp", "4102444800001"},
		{"timestamp", "0946684799"},
		{"startTime", int64(9999999999)},
		{"startTime", int64(123456789012)},
		{"startTime", int64(4102444800001)},
		{"time", "24:00"},
		{"time", "7:12"},
		{"time", "07:60"},
		{"date", "12345"},
	}
	for _, c := range drop {
		if got := redactOne(t, c.key, c.val); got != `"[redacted]"` {
			t.Errorf("%s=%v kept: %s", c.key, c.val, got)
		}
	}
}

func TestRedactEnumShapes(t *testing.T) {
	t.Parallel()

	for _, v := range []string{
		"readiness", "watch_score", "real_data", "insightData", "HRVRMSSD", "UPPER_SNAKE",
		"ms", "Charge", "Emotion", "DailyHealth", "skinTemp", "hrv_sdnn", strings.Repeat("x", 20), "SUCCESS",
	} {
		if got := redactOne(t, "type", v); got != `"`+v+`"` {
			t.Errorf("enum %q lost: %s", v, got)
		}
	}
	for _, v := range []string{
		"Testy", "Fakename", "Nowhere", "JUAN", "TESTY", "Fake_Name", "FakeName", "fake__name", "_x", "x_",
		strings.Repeat("x", 21), strings.Repeat("x", 33), "very_long_enum_name_with_words", "insightDataLongerThanTwenty",
	} {
		if got := redactOne(t, "status", v); got != `"[redacted]"` {
			t.Errorf("%q kept: %s", v, got)
		}
	}
}

// #40 needs samples[].s (ms offset) and samples[].hrv as numbers.
func TestRedactKeepsHRVSampleOffsets(t *testing.T) {
	t.Parallel()

	in := `{"value":{"startTime":1761710400000,"deviceId":"SYN-DEV","samples":[{"hrv":41,"s":0},{"hrv":0,"s":60000},` +
		`{"hrv":255,"s":36000000},{"sdnn":52,"s":"120000"}]}}`
	out, err := icu.RedactZeppJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"value":{"startTime":1761710400000,"deviceId":"[redacted]","samples":[{"hrv":41,"s":0},{"hrv":0,"s":60000},` +
		`{"hrv":255,"s":36000000},{"sdnn":52,"s":"120000"}]}}`
	if string(out) != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
}

// Pins the owner fragment (ownerName alone is also caught by the name suffix).
func TestRedactOwnerFragment(t *testing.T) {
	t.Parallel()

	for _, k := range []string{"owner", "ownerAge", "deviceOwner"} {
		if got := redactOne(t, k, 7); got != `"[redacted]"` {
			t.Errorf("%q kept: %s", k, got)
		}
	}
}
