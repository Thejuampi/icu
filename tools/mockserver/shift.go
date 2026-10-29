package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// shiftMarker is written at the root of a fixture directory once its dates
// have been shifted, so re-running -anonymize never shifts them again.
const shiftMarker = ".dates-shifted"

const shiftMarkerText = "dates shifted by a random whole-week offset (value not recorded)\n"

// Random shift bounds: whole weeks back, so weekdays (and every weekday name
// in free text) stay correct.
const (
	minShiftWeeks = 3
	maxShiftWeeks = 12
	daysPerWeek   = 7
	// maxJitterBasisPoints bounds the distance/moving-time jitter (±4%).
	maxJitterBasisPoints = 400
	minJitterBasisPoints = 100
	basisPoints          = 10_000
	signBit              = 1 << 31
)

// dateInText matches a calendar day anywhere inside a string, including
// ids ("2026-10-26"), timestamps and free-text names and descriptions.
var dateInText = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// activityID matches intervals.icu activity ids ("i900000001").
var activityID = regexp.MustCompile(`^i\d+$`)

// jitteredKeys are scaled by one per-activity factor, so speed and the
// elapsed >= moving invariant are preserved.
func jitteredKeys() []string {
	return []string{"distance", "icu_distance", "moving_time", "elapsed_time"}
}

// dateShift moves every date in a fixture set by one shared offset and
// jitters per-activity distance and moving time. Unlike anonymize it is not
// idempotent by itself; the directory marker makes -anonymize safe to re-run.
type dateShift struct {
	days int
	salt uint32
}

func randomDateShift() dateShift {
	var buf [8]byte

	_, _ = rand.Read(buf[:])

	weeks := minShiftWeeks + int(binary.LittleEndian.Uint32(buf[:4])%uint32(maxShiftWeeks-minShiftWeeks+1))

	return dateShift{days: -weeks * daysPerWeek, salt: binary.LittleEndian.Uint32(buf[4:])}
}

func (s dateShift) text(value string) string {
	matches := dateInText.FindAllStringIndex(value, -1)
	if matches == nil {
		return value
	}

	var out strings.Builder

	last := 0

	for _, match := range matches {
		start, end := match[0], match[1]
		// Skip digit runs that merely contain a date-like pattern.
		if (start > 0 && isDigit(value[start-1])) || (end < len(value) && isDigit(value[end])) {
			continue
		}

		parsed, err := time.Parse(time.DateOnly, value[start:end])
		if err != nil {
			continue
		}

		out.WriteString(value[last:start])
		out.WriteString(parsed.AddDate(0, 0, s.days).Format(time.DateOnly))
		last = end
	}

	out.WriteString(value[last:])

	return out.String()
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// factor returns a deterministic per-activity jitter of 1-4% either way.
func (s dateShift) factor(id string) float64 {
	hash := fnv.New32a()
	_, _ = fmt.Fprintf(hash, "%d:%s", s.salt, id)
	sum := hash.Sum32()

	bp := minJitterBasisPoints + float64(sum%(maxJitterBasisPoints-minJitterBasisPoints+1))
	if sum&signBit != 0 {
		bp = -bp
	}

	return 1 + bp/basisPoints
}

func (s dateShift) apply(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, inner := range typed {
			// Redacted placeholders (date of birth) are fixed values.
			if _, redacted := redactedKey(key); redacted {
				out[key] = inner

				continue
			}

			out[s.text(key)] = s.apply(inner)
		}

		if id, ok := out["id"].(string); ok && activityID.MatchString(id) {
			s.jitter(out, id)
		}

		return out
	case []any:
		for i, inner := range typed {
			typed[i] = s.apply(inner)
		}

		return typed
	case string:
		return s.text(typed)
	default:
		return value
	}
}

func (s dateShift) jitter(obj map[string]any, id string) {
	factor := s.factor(id)

	for _, key := range jitteredKeys() {
		number, ok := obj[key].(json.Number)
		if !ok {
			continue
		}

		if whole, err := number.Int64(); err == nil {
			obj[key] = json.Number(strconv.FormatInt(int64(math.Round(float64(whole)*factor)), 10))

			continue
		}

		if value, err := number.Float64(); err == nil {
			obj[key] = json.Number(strconv.FormatFloat(math.Round(value*factor*100)/100, 'f', -1, 64))
		}
	}
}

// shiftJSON applies the shift to one JSON document; invalid JSON is returned
// unchanged.
func (s dateShift) shiftJSON(body []byte) []byte {
	var value any
	if decodeJSON(body, &value) != nil {
		return body
	}

	out, err := json.Marshal(s.apply(value))
	if err != nil {
		return body
	}

	return out
}

// dirShifted reports whether the fixture directory already carries the
// shift marker.
func dirShifted(root *os.Root) bool {
	_, err := root.Stat(shiftMarker)

	return err == nil
}
