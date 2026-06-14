package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync"

	icu "github.com/Thejuampi/icu"
)

const strTrue = "true"

var (
	stdoutOverride   io.Writer  //nolint:gochecknoglobals // test output redirection
	stdoutOverrideMu sync.Mutex //nolint:gochecknoglobals // protects stdoutOverride
)

func osStdout() io.Writer {
	stdoutOverrideMu.Lock()
	ov := stdoutOverride
	stdoutOverrideMu.Unlock()

	if ov != nil {
		return ov
	}

	return os.Stdout
}

func setStdoutForTest(w io.Writer) {
	stdoutOverrideMu.Lock()
	stdoutOverride = w
	stdoutOverrideMu.Unlock()
}

func osGetenv(key string) string { return os.Getenv(key) }

func wrapCommandError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("command failed: %w", err)
}

func writeJSON(v any) error {
	return wrapCommandError(icu.WriteJSON(osStdout(), v))
}

func writeOutput(data []byte) error {
	if _, err := osStdout().Write(data); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func queryFromFlags(flags map[string]string, keys ...string) map[string]string {
	q := map[string]string{}

	for _, k := range keys {
		if v, ok := flags[k]; ok && v != "" {
			q[k] = v
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
