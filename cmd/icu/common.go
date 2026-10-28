package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	icu "github.com/Thejuampi/icu"
)

const strTrue = "true"

func osGetenv(key string) string { return os.Getenv(key) }

func wrapCommandError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("command failed: %w", err)
}

func writeJSON(stdout io.Writer, v any) error {
	return wrapCommandError(icu.WriteJSON(stdout, v))
}

func writeOutput(stdout io.Writer, data []byte) error {
	if _, err := stdout.Write(data); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func queryFromFlags(flags map[string]string, keys ...string) map[string]string {
	q := map[string]string{}

	for _, k := range keys {
		if v, ok := flags[k]; ok && v != "" {
			q[strings.ReplaceAll(k, "-", "_")] = v
		}
	}

	return q
}

func floatFlagVal(flags map[string]string, name string, defaultVal float64) float64 {
	v, ok := flags[name]
	if !ok || v == "" {
		return defaultVal
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}

	if math.IsNaN(f) || math.IsInf(f, 0) {
		return defaultVal
	}

	return f
}

func BoolFlag(args map[string]string, name string) bool {
	v, ok := args[name]

	return ok && (v == strTrue || v == "1" || v == "")
}

func BoolPtrFlag(args map[string]string, name string) *bool {
	v, ok := args[name]
	if !ok {
		return nil
	}

	enabled := v == strTrue || v == "1" || v == ""

	return &enabled
}

func IntFlag(args map[string]string, name string, defaultVal int) int {
	v, ok := args[name]
	if !ok || v == "" {
		return defaultVal
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}

	return n
}

// sessionEnvironmentFromFlags reads --session-environment (D-10c's
// per-invocation escape hatch). Validated at this CLI boundary, not inside
// ResolveFTP: the resolver treats any non-"indoor" string as outdoor, so a
// typo here would silently resolve as a confident "outdoor" override instead
// of failing loudly.
func sessionEnvironmentFromFlags(flags map[string]string) (string, error) {
	value := icu.StringFlag(flags, "session-environment", "")
	if value != "" && value != icu.SessionEnvironmentIndoor && value != icu.SessionEnvironmentOutdoor {
		return "", fmt.Errorf("session-environment must be %q or %q, got %q",
			icu.SessionEnvironmentIndoor, icu.SessionEnvironmentOutdoor, value)
	}

	return value, nil
}

func stringFlagAlias(flags map[string]string, primary, fallback string) string {
	if v, ok := flags[primary]; ok && v != "" {
		return v
	}

	if v, ok := flags[fallback]; ok && v != "" {
		return v
	}

	return ""
}
