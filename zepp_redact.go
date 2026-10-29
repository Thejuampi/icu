package icu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ZeppRedacted replaces every value whose key names an identifier or secret.
const ZeppRedacted = "[redacted]"

// zeppSecretKeys are exact key names (lower-cased, with "_" and "-" removed)
// that identify the user, the device or a session.
func zeppSecretKeys() map[string]bool {
	return map[string]bool{
		"userid": true, "uid": true, "huamiid": true, "deviceid": true, "sn": true,
		"serialnumber": true, "mac": true, "macaddress": true, "imei": true, "email": true,
		"phone": true, "phonenumber": true, "id": true, "openid": true, "unionid": true,
		"thirdpartyid": true, "authorization": true, "cookie": true, "code": true, "name": true,
		"nickname": true, "username": true,
	}
}

// zeppSecretFragments redact any key containing them.
func zeppSecretFragments() []string {
	return []string{"token", "secret", "password", "session", "credential", "apikey"}
}

// IsZeppSecretKey reports whether a JSON key holds an identifier or secret.
// Matching is case-insensitive and ignores "_" and "-"; any key ending in
// "id" as its own word (userId, device_id, eventID) counts as an identifier.
func IsZeppSecretKey(key string) bool {
	lower := strings.ToLower(key)
	norm := strings.NewReplacer("_", "", "-", "").Replace(lower)
	if zeppSecretKeys()[norm] {
		return true
	}

	for _, f := range zeppSecretFragments() {
		if strings.Contains(norm, f) {
			return true
		}
	}

	return strings.HasSuffix(key, "Id") || strings.HasSuffix(key, "ID") ||
		strings.HasSuffix(lower, "_id") || strings.HasSuffix(lower, "-id")
}

// RedactZeppJSON returns raw with key order, numbers and structure kept, but
// every value under an identifier/secret key (at any depth) replaced by
// ZeppRedacted. A string value that itself holds a JSON object or array is
// redacted recursively too and kept as a string.
func RedactZeppJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var out bytes.Buffer
	if err := redactValue(dec, &out); err != nil {
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

func redactValue(dec *json.Decoder, out *bytes.Buffer) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("read value: %w", err)
	}

	switch t := tok.(type) {
	case json.Delim:
		return redactContainer(dec, out, t)
	case string:
		trimmed := strings.TrimSpace(t)
		if trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
			inner, err := RedactZeppJSON([]byte(trimmed))
			if err == nil {
				writeJSONString(out, string(inner))

				return nil
			}
		}
		writeJSONString(out, t)
	case json.Number:
		out.WriteString(t.String())
	case bool:
		fmt.Fprintf(out, "%t", t)
	case nil:
		out.WriteString("null")
	}

	return nil
}

func redactContainer(dec *json.Decoder, out *bytes.Buffer, open json.Delim) error {
	out.WriteRune(rune(open))
	first := true
	for dec.More() {
		if !first {
			out.WriteByte(',')
		}
		first = false

		if open == '{' {
			keyTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("read key: %w", err)
			}
			key, _ := keyTok.(string)
			writeJSONString(out, key)
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

		if err := redactValue(dec, out); err != nil {
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
