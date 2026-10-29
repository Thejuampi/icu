package icu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
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
//     known-safe field AND the string has that field's shape: ISO date/time
//     or epoch for time keys, known enums or strict code case (<= 20 chars) for enum keys, a short
//     number for metric keys. Strings inside arrays are never allowlisted.
//     Numbers, booleans and null are kept (they are the payload), except an
//     integer of 9+ digits, unless it is an epoch under a time-like key.
//  3. Value scrubbing: an allowlisted string is still replaced when it looks
//     like an email, MAC, JWT, UUID, URL, IPv4/IPv6, a base64/hex blob, or JSON that
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
		// "ip" is exact (plus the ipaddr/ipv4/ipv6 fragments and these common
		// names), not a suffix or fragment: that would hit tip, zip, chip, trip.
		"clientip", "remoteip", "lastip", "loginip", "userip", "deviceip",
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
		"birth", "dob", "zip", "postcode", "city", "country", "region", "geo", "coord", "position", "owner", "friend",
		"member", "wearer", "host", "msisdn", "ipaddr", "ipv4", "ipv6",
		// Device identity: deviceSource, deviceType, deviceModel, ... carry
		// numeric device/model ids (a 7-digit deviceSource leaked through).
		"device", "firmware", "hardware", "hwver", "fwver", "productid", "productver", "macaddress", "bluetooth",
		"vendor", "brand", "model", "imsi", "meid", "esn", "sourceid",
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

// zeppSafeClass is the expected shape of a string under a safe key.
type zeppSafeClass int

const (
	zeppClassNone   zeppSafeClass = iota
	zeppClassTime                 // ISO date/time or a numeric epoch (<= 13 digits)
	zeppClassEnum                 // known enum or strict code case, <= 20 chars
	zeppClassMetric               // up to 6 digits, optional sign and decimals
	zeppClassOffset               // sample offset in ms: up to 8 digits
)

// zeppSafeStringKeys maps the keys whose string values may be kept to the
// shape they must have; anything else under them is redacted. They carry the
// dates, event enums and metric values --raw exists for.
func zeppSafeStringKeys() map[string]zeppSafeClass {
	return map[string]zeppSafeClass{
		"date": zeppClassTime, "day": zeppClassTime, "time": zeppClassTime, "timestamp": zeppClassTime,
		"starttime": zeppClassTime, "endtime": zeppClassTime,
		"eventtype": zeppClassEnum, "subtype": zeppClassEnum, "type": zeppClassEnum, "unit": zeppClassEnum,
		"status": zeppClassEnum, "result": zeppClassEnum,
		"value": zeppClassMetric, "hrv": zeppClassMetric, "sdnn": zeppClassMetric, "rmssd": zeppClassMetric,
		"sleephrv": zeppClassMetric, "score": zeppClassMetric, "samples": zeppClassMetric, "s": zeppClassOffset,
	}
}

func zeppKeyClass(key string) zeppSafeClass {
	if key == "" || IsZeppSecretKey(key) {
		return zeppClassNone
	}
	norm := normalizeZeppKey(key)
	if c, ok := zeppSafeStringKeys()[norm]; ok {
		return c
	}
	if strings.HasPrefix(norm, "skintemp") {
		return zeppClassMetric
	}

	return zeppClassNone
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

// IsZeppSafeString reports whether str under key is kept: the key must be
// on the safe list, str must have the key's expected shape, and it must not
// match a sensitive value pattern.
func IsZeppSafeString(key, str string) bool {
	var ok bool
	switch zeppKeyClass(key) {
	case zeppClassTime:
		ok = zeppISOTimeRe.MatchString(str) || zeppClockRe.MatchString(str) || zeppEpochString(str)
	case zeppClassEnum:
		ok = zeppEnumString(str)
	case zeppClassOffset:
		ok = zeppOffsetRe.MatchString(str)
	case zeppClassMetric:
		ok = zeppMetricRe.MatchString(str)
	case zeppClassNone:
		ok = false
	}

	return ok && !zeppValueLooksSensitive(str)
}

// zeppIsTimeKey reports whether key holds dates or epochs.
func zeppIsTimeKey(key string) bool {
	return zeppKeyClass(key) == zeppClassTime
}

// zeppNumberIsSensitive reports whether a number under a non-denied key
// looks like an identifier: an integer of 9+ digits, unless it is a
// plausible epoch (<= 13 digits) under a time-like key.
func zeppNumberIsSensitive(key, num string) bool {
	f, ok := new(big.Float).SetString(num)
	if !ok {
		return true // not a number we understand: do not print it
	}
	abs := new(big.Float).Abs(f)
	if abs.Cmp(big.NewFloat(zeppMaxPlainInt)) < 0 {
		return false // integer part has at most 8 digits
	}
	if !zeppIsTimeKey(key) || !abs.IsInt() {
		return true
	}
	n, _ := abs.Int64()

	return !zeppPlausibleEpoch(n)
}

// zeppMaxPlainInt is the smallest value whose integer part has 9 digits.
const zeppMaxPlainInt = 1e8

// Epoch bounds: 2000-01-01 to 2100-01-01 UTC, in seconds and milliseconds.
const (
	zeppEpochMinSec = 946684800
	zeppEpochMaxSec = 4102444800
	zeppMsPerSec    = 1000
)

// zeppPlausibleEpoch reports whether n is a 10-digit epoch in seconds or a
// 13-digit epoch in milliseconds between 2000 and 2100.
func zeppPlausibleEpoch(n int64) bool {
	return (n >= zeppEpochMinSec && n <= zeppEpochMaxSec) ||
		(n >= zeppEpochMinSec*zeppMsPerSec && n <= zeppEpochMaxSec*zeppMsPerSec)
}

// zeppEpochString reports whether str is a plausible 10- or 13-digit epoch.
func zeppEpochString(str string) bool {
	if !zeppEpochRe.MatchString(str) {
		return false
	}
	n, err := strconv.ParseInt(str, 10, 64)

	return err == nil && zeppPlausibleEpoch(n)
}

// zeppEnumMaxLen caps the length of a pattern-matched enum string.
const zeppEnumMaxLen = 20

// zeppKnownEnums are enum strings Zepp is known to send that the strict
// pattern alone would reject (single capitalised or all-caps words).
func zeppKnownEnums() map[string]bool {
	out := map[string]bool{"OK": true, "SUCCESS": true, "FAIL": true, "NONE": true, "UNKNOWN": true}
	for _, p := range v2EventPresets() {
		out[p.EventType] = true
		if p.SubType != "" {
			out[p.SubType] = true
		}
	}

	return out
}

// zeppEnumString reports whether str is an enum value: a known Zepp enum,
// or at most zeppEnumMaxLen chars of strict code case, i.e. snake_case
// ("watch_score"), lowerCamelCase with at least one hump ("insightData") or
// UPPER_SNAKE with at least one underscore ("REAL_DATA"). Single
// capitalised or all-caps words ("Testy", "JUAN") and anything longer look
// like names or free text and are redacted.
func zeppEnumString(str string) bool {
	if zeppKnownEnums()[str] {
		return true
	}

	return len(str) <= zeppEnumMaxLen && zeppStrictEnumRe.MatchString(str)
}

var (
	zeppEmailRe   = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	zeppMACRe     = regexp.MustCompile(`(?i)\b[0-9a-f]{2}([:-][0-9a-f]{2}){5}\b`)
	zeppJWTRe     = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.?[A-Za-z0-9_-]*`)
	zeppUUIDRe    = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{12}\b`)
	zeppURLRe     = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://`)
	zeppBlobRe    = regexp.MustCompile(`[A-Za-z0-9+/_-]{24,}={0,2}`)
	zeppHexRe     = regexp.MustCompile(`(?i)\b[0-9a-f]{12,}\b`)
	zeppIPv4Re    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	zeppIPv6Re    = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}\b|[0-9a-f]{0,4}::[0-9a-f:]*`)
	zeppISOTimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d{1,9})?)?(?:Z|[+-]\d{2}:?\d{2})?)?$`)
	zeppEpochRe   = regexp.MustCompile(`^(?:\d{10}|\d{13})$`)
	zeppClockRe   = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?$`)
	zeppOffsetRe  = regexp.MustCompile(`^\d{1,8}$`)
	// zeppStrictEnumRe: snake_case | lowerCamelCase (1+ hump) | UPPER_SNAKE (1+ "_").
	zeppStrictEnumRe = regexp.MustCompile(`^(?:[a-z]+(?:_[a-z0-9]+)*|[a-z]+(?:[A-Z][a-z0-9]*)+|[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+)$`)
	zeppMetricRe     = regexp.MustCompile(`^-?\d{1,6}(?:\.\d{1,6})?$`)
	zeppIDKeyRe      = regexp.MustCompile(`^(?:\d+|(?i)[0-9a-f]{8,})$`)
	zeppNumberRe     = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

// zeppValueLooksSensitive reports whether a string value matches a
// sensitive pattern (step 3).
func zeppValueLooksSensitive(val string) bool {
	if zeppNumberRe.MatchString(val) {
		return false // plain numbers (e.g. "57", timestamps) are payload
	}
	// A blob counts when it has a digit or a base64 symbol, so a long
	// letters-only enum (UPPER_SNAKE, camelCase) is left to the enum rule.
	for _, m := range zeppBlobRe.FindAllString(val, -1) {
		if strings.ContainsAny(m, "0123456789+/=") {
			return true
		}
	}
	for _, re := range []*regexp.Regexp{zeppEmailRe, zeppMACRe, zeppJWTRe, zeppUUIDRe, zeppURLRe, zeppHexRe, zeppIPv4Re, zeppIPv6Re} {
		if re.MatchString(val) {
			return true
		}
	}
	t := strings.TrimSpace(val)

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
	if err := redactValue(dec, &out, "", false, depth); err != nil {
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
	if !IsZeppSafeString(key, str) {
		return ZeppRedacted
	}

	return str
}

func redactValue(dec *json.Decoder, out *bytes.Buffer, key string, inArray bool, depth int) error {
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
		strKey := key
		if inArray {
			strKey = "" // strings in arrays get no allowlist
		}
		writeJSONString(out, redactString(t, strKey, depth))
	case json.Number:
		if zeppNumberIsSensitive(key, t.String()) {
			writeJSONString(out, ZeppRedacted)
		} else {
			out.WriteString(t.String())
		}
	case bool:
		out.WriteString(strconv.FormatBool(t))
	case nil:
		out.WriteString("null")
	}

	return nil
}

// redactContainer copies an object or array. Numbers in an array inherit the
// array's key (samples: [41, 38]); strings in an array never match the
// allowlist and are redacted.
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

		if err := redactValue(dec, out, key, open == '[', depth+1); err != nil {
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
