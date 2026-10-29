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
	realPrecision        = 1000
)

// dateInText matches a calendar day anywhere inside a string, including
// ids ("2026-10-26"), timestamps and free-text names and descriptions.
var dateInText = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// activityID matches intervals.icu activity ids ("i900000001").
var activityID = regexp.MustCompile(`^i\d+$`)

// Jitter channels: distance and moving time get independent factors;
// elapsed time follows moving time (so elapsed >= moving still holds) and
// speeds follow distance/moving.
const (
	channelDistance = "distance"
	channelMoving   = "moving"
)

// distanceKeys scale by the activity's distance factor. Elevation gain and
// loss ride on the same factor: on a fixed-gradient course climbing grows
// with distance, so a separate factor would make the implied gradient drift.
func distanceKeys() []string {
	return []string{"distance", "icu_distance", "total_elevation_gain", "total_elevation_loss"}
}

func movingKeys() []string {
	return []string{"moving_time", "elapsed_time", "icu_recording_time", "time"}
}

func speedKeys() []string { return []string{"average_speed", "max_speed", "pace", "gap"} }

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

// factor returns a deterministic per-activity, per-channel jitter of 1-4%
// either way; |factor-1| is never below 0.01.
func (s dateShift) factor(id, channel string) float64 {
	hash := fnv.New32a()
	_, _ = fmt.Fprintf(hash, "%d:%s:%s", s.salt, id, channel)
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
	distance, moving := s.factor(id, channelDistance), s.factor(id, channelMoving)

	scaleKeys(obj, distanceKeys(), distance)
	scaleKeys(obj, movingKeys(), moving)
	scaleKeys(obj, speedKeys(), distance/moving)

	// Time in zones follows moving time, so zone totals still add up.
	scaleKeys(obj, []string{"icu_hr_zone_times", "pace_zone_times", "gap_zone_times"}, moving)

	if zones, ok := obj["icu_zone_times"].([]any); ok {
		for _, raw := range zones {
			if zone, ok := raw.(map[string]any); ok {
				scaleKeys(zone, []string{"secs"}, moving)
			}
		}
	}
}

func scaleKeys(obj map[string]any, keys []string, factor float64) {
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			obj[key] = scaleNumber(value, factor)
		}
	}
}

// scaleNumber scales a JSON number (or array of numbers, as in streams),
// keeping integers integral and rounding reals to 3 decimals; nulls and
// other values are kept.
func scaleNumber(value any, factor float64) any {
	switch typed := value.(type) {
	case json.Number:
		if whole, err := typed.Int64(); err == nil {
			return json.Number(strconv.FormatInt(int64(math.Round(float64(whole)*factor)), 10))
		}

		if f, err := typed.Float64(); err == nil {
			return json.Number(strconv.FormatFloat(math.Round(f*factor*realPrecision)/realPrecision, 'f', -1, 64))
		}
	case []any:
		out := make([]any, len(typed))
		for i, inner := range typed {
			out[i] = scaleNumber(inner, factor)
		}

		return out
	}

	return value
}

// scaleReal is scaleNumber without integer rounding, for the time stream
// (rounding 0,1,2,... would leave it unchanged or make duplicates).
func scaleReal(value any, factor float64) any {
	switch typed := value.(type) {
	case json.Number:
		if f, err := typed.Float64(); err == nil {
			return json.Number(strconv.FormatFloat(math.Round(f*factor*realPrecision)/realPrecision, 'f', -1, 64))
		}
	case []any:
		out := make([]any, len(typed))
		for i, inner := range typed {
			out[i] = scaleReal(inner, factor)
		}

		return out
	}

	return value
}

// droppedStreams carry location-revealing data (altitude profile, weather).
func droppedStreams() map[string]bool { return map[string]bool{"altitude": true, "temp": true} }

// shiftStreams scales the time, distance and velocity streams of activity id
// by the same factors as the activity itself and drops altitude and temp.
// The time stream is scaled as a real (it is elapsed time, not an index).
func (s dateShift) shiftStreams(streams []any, id string) []any {
	distance, moving := s.factor(id, channelDistance), s.factor(id, channelMoving)
	out := make([]any, 0, len(streams))

	for _, raw := range streams {
		stream, ok := raw.(map[string]any)
		if !ok {
			out = append(out, raw)

			continue
		}

		streamType, _ := stream["type"].(string)
		if droppedStreams()[streamType] {
			continue
		}

		switch streamType {
		case "time":
			stream["data"] = scaleReal(stream["data"], moving)
		case "distance":
			stream["data"] = scaleReal(stream["data"], distance)
		case "velocity_smooth":
			stream["data"] = scaleReal(stream["data"], distance/moving)
		}

		out = append(out, stream)
	}

	return out
}

// streamsActivity returns the activity id of an ".../activity/<id>/streams.json"
// fixture path.
func streamsActivity(name string) (string, bool) {
	match := streamsPath.FindStringSubmatch(name)
	if match == nil {
		return "", false
	}

	return match[1], true
}

var streamsPath = regexp.MustCompile(`(?:^|/)activity/(i\d+)/streams\.json$`)

// shiftJSON applies the shift to one JSON document; invalid JSON is returned
// unchanged.
func (s dateShift) shiftJSON(body []byte) []byte {
	return s.shiftFile("", body)
}

// shiftFile is shiftJSON for a fixture at name, so streams files can be
// matched to their activity.
func (s dateShift) shiftFile(name string, body []byte) []byte {
	var value any
	if decodeJSON(body, &value) != nil {
		return body
	}

	shifted := s.apply(value)
	if id, ok := streamsActivity(name); ok {
		if streams, ok := shifted.([]any); ok {
			shifted = s.shiftStreams(streams, s.text(id))
		}
	}

	out, err := json.Marshal(shifted)
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
