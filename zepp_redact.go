package icu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// ZeppRedacted replaces every value whose key names an identifier or secret,
// and every string value not on the safe-string allowlist.
const ZeppRedacted = "[redacted]"

// zeppRedactMaxDepth bounds recursion (nesting plus embedded JSON levels).
const zeppRedactMaxDepth = 64

var errZeppRedactTooDeep = errors.New("json nested too deep")

// --raw redaction is defense in depth:
//
//  1. Denylist: a key that names an identifier, personal data, location,
//     secret or link (normalized: lower-case, "_", "-", "." and spaces
//     removed) has its value replaced whatever its type, objects included.
//  2. Allowlist: any other string is replaced too, unless its key is a
//     known-safe field (dates, timestamps, event type enums and the
//     HRV/readiness/skin-temp metric keys). Numbers, booleans and null are
//     kept: they are the payload we need, and an identifier in a number
//     sits under a denylisted key.
//  3. Value scrubbing: an allowlisted string is still replaced when it looks
//     like an email, MAC, JWT, UUID, URL, a base64/hex blob, or JSON that
//     does not parse.
//  4. Strings holding JSON (also double-encoded) are unwrapped and the same
//     rules apply inside; the result stays a string.
//  5. Object keys that are only digits, or 8+ hex chars, are replaced by
//     "[redacted-key-N]" (they are usually ids).

// zeppDeniedKeys are exact normalized key names that are always redacted.
func zeppDeniedKeys() map[string]bool {
	keys := []string{
		"id", "ids", "uid", "sn", "code", "name", "nick", "alias", "tel", "lat", "lon", "lng", "ip", "key", "sign",
		"access", "refresh", "gender", "url", "uri", "mac", "timezone", "tz", "fullname", "firstname", "lastname",
		"displayname", "username", "huamiid", "openid", "unionid", "imei", "imsi", "iccid", "serial", "serialno",
		"birthday", "mobile", "photo", "avatar", "account", "location", "address", "gps", "ssid", "bssid",
		"latitude", "longitude", "signature", "cookie", "auth", "authorization",
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}

	return out
}

// zeppDeniedFragments redact any normalized key containing them.
func zeppDeniedFragments() []string {
	return []string{
		"token", "secret", "passw", "session", "credential", "apikey", "email", "phone", "mobile", "imei", "imsi",
		"iccid", "uuid", "guid", "serial", "avatar", "address", "latitude", "longitude", "signature", "account",
		"nickname", "birthday", "cookie", "auth", "userid", "deviceid", "macaddr", "ssid", "photo", "gps",
		"location", "nick", "alias", "fullname", "firstname", "lastname", "displayname", "devicename",
	}
}

// zeppDeniedSuffixes redact a normalized key ending with them (fooid, fooIds,
// avatarUrl, appKey, ...), except for the words in zeppSuffixExceptions.
func zeppDeniedSuffixes() []string {
	return []string{"id", "ids", "uuid", "guid", "url", "uri", "key", "token", "addr", "address", "mac", "sn", "name"}
}

func zeppSuffixExceptions() map[string]bool {
	return map[string]bool{
		"valid": true, "invalid": true, "paid": true, "unpaid": true, "hybrid": true, "humid": true, "fluid": true,
		"void": true, "avoid": true, "rapid": true, "solid": true, "liquid": true, "lucid": true, "acid": true,
	}
}

// zeppSafeStringKeys are the keys whose string values are kept (after value
// scrubbing). They carry dates, event enums and the metrics --raw exists for.
func zeppSafeStringKeys() map[string]bool {
	keys := []string{
		"date", "day", "time", "timestamp", "starttime", "endtime", "eventtype", "subtype", "type", "unit",
		"value", "hrv", "sdnn", "rmssd", "sleephrv", "score", "samples", "s", "status", "result",
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}

	return out
}

func normalizeZeppKey(key string) string {
	return strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(strings.ToLower(key))
}

// IsZeppSecretKey reports whether a JSON key holds an identifier or secret
// (step 1 above). Matching is case-insensitive and ignores "_", "-", ".".
func IsZeppSecretKey(key string) bool {
	norm := normalizeZeppKey(key)
	if zeppDeniedKeys()[norm] {
		return true
	}
	for _, f := range zeppDeniedFragments() {
		if strings.Contains(norm, f) {
			return true
		}
	}
	if zeppSuffixExceptions()[norm] {
		return false
	}
	for _, suf := range zeppDeniedSuffixes() {
		if strings.HasSuffix(norm, suf) && len(norm) > len(suf) {
			return true
		}
	}

	return false
}

// IsZeppSafeStringKey reports whether a string under key is kept (step 2).
func IsZeppSafeStringKey(key string) bool {
	if IsZeppSecretKey(key) {
		return false
	}
	norm := normalizeZeppKey(key)

	return zeppSafeStringKeys()[norm] || strings.HasPrefix(norm, "skintemp")
}

var (
	zeppEmailRe  = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	zeppMACRe    = regexp.MustCompile(`(?i)\b[0-9a-f]{2}([:-][0-9a-f]{2}){5}\b`)
	zeppJWTRe    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.?[A-Za-z0-9_-]*`)
	zeppUUIDRe   = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{12}\b`)
	zeppURLRe    = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://`)
	zeppBlobRe   = regexp.MustCompile(`[A-Za-z0-9+/_-]{24,}={0,2}`)
	zeppHexRe    = regexp.MustCompile(`(?i)\b[0-9a-f]{12,}\b`)
	zeppIDKeyRe  = regexp.MustCompile(`^(?:\d+|(?i)[0-9a-f]{8,})$`)
	zeppNumberRe = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

// zeppValueLooksSensitive reports whether a string value matches a
// sensitive pattern (step 3).
func zeppValueLooksSensitive(s string) bool {
	if zeppNumberRe.MatchString(s) {
		return false // plain numbers (e.g. "57", timestamps) are payload
	}
	for _, re := range []*regexp.Regexp{zeppEmailRe, zeppMACRe, zeppJWTRe, zeppUUIDRe, zeppURLRe, zeppBlobRe, zeppHexRe} {
		if re.MatchString(s) {
			return true
		}
	}
	t := strings.TrimSpace(s)

	return t != "" && (t[0] == '{' || t[0] == '[' || t[0] == '"')
}

// RedactZeppJSON returns raw with structure, key order, numbers and booleans
// kept, redacted as described above.
func RedactZeppJSON(raw []byte) ([]byte, error) {
	return redactZeppJSONDepth(raw, 0)
}

func redactZeppJSONDepth(raw []byte, depth int) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var out bytes.Buffer
	if err := redactValue(dec, &out, "", depth); err != nil {
		return nil, fmt.Errorf("redact zepp json: %w", err)
	}

	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("redact zepp json: trailing data")
	}

	return out.Bytes(), nil
}

func writeJSONString(out *bytes.Buffer, s string) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil { // a Go string always encodes
		out.WriteString(`""`)

		return
	}
	out.Write(bytes.TrimRight(b.Bytes(), "\n"))
}

// redactString applies steps 2-4 to a string value found under key.
func redactString(str, key string, depth int) string {
	t := strings.TrimSpace(str)
	if t != "" && (t[0] == '{' || t[0] == '[' || t[0] == '"') {
		if depth < zeppRedactMaxDepth && json.Valid([]byte(t)) {
			if t[0] == '"' { // double-encoded: a JSON string inside a string
				var inner string
				if json.Unmarshal([]byte(t), &inner) == nil {
					var b bytes.Buffer
					writeJSONString(&b, redactString(inner, key, depth+1))

					return b.String()
				}
			} else if inner, err := redactZeppJSONDepth([]byte(t), depth+1); err == nil {
				return string(inner)
			}
		}

		return ZeppRedacted // looks like JSON but does not parse (or too deep)
	}
	if !IsZeppSafeStringKey(key) || zeppValueLooksSensitive(str) {
		return ZeppRedacted
	}

	return str
}

func redactValue(dec *json.Decoder, out *bytes.Buffer, key string, depth int) error {
	if depth > zeppRedactMaxDepth {
		return errZeppRedactTooDeep
	}
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("read value: %w", err)
	}

	switch t := tok.(type) {
	case json.Delim:
		return redactContainer(dec, out, t, key, depth)
	case string:
		writeJSONString(out, redactString(t, key, depth))
	case json.Number:
		out.WriteString(t.String())
	case bool:
		out.WriteString(strconv.FormatBool(t))
	case nil:
		out.WriteString("null")
	}

	return nil
}

// redactContainer copies an object or array. Array elements inherit the
// array's key, so ["a@b.c"] under "emails" or "note" is handled like a
// single value under that key.
func redactContainer(dec *json.Decoder, out *bytes.Buffer, open json.Delim, parentKey string, depth int) error {
	out.WriteRune(rune(open))
	first := true
	hidden := 0
	for dec.More() {
		if !first {
			out.WriteByte(',')
		}
		first = false

		key := parentKey
		if open == '{' {
			keyTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("read key: %w", err)
			}
			key, _ = keyTok.(string)
			if zeppIDKeyRe.MatchString(key) {
				hidden++
				writeJSONString(out, fmt.Sprintf("[redacted-key-%d]", hidden))
			} else {
				writeJSONString(out, key)
			}
			out.WriteByte(':')
			if IsZeppSecretKey(key) {
				var skip json.RawMessage
				if err := dec.Decode(&skip); err != nil {
					return fmt.Errorf("skip redacted value: %w", err)
				}
				writeJSONString(out, ZeppRedacted)

				continue
			}
		}

		if err := redactValue(dec, out, key, depth+1); err != nil {
			return err
		}
	}

	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("close container: %w", err)
	}

	if open == '{' {
		out.WriteByte('}')
	} else {
		out.WriteByte(']')
	}

	return nil
}
