package main

import (
	"fmt"
	"io"
	"os"
)

const strTrue = "true"

func osStdout() io.Writer { return os.Stdout }

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
	if v, ok := flags[name]; ok && v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
			return defaultVal
		}

		return f
	}

	return defaultVal
}

func BoolFlag(args map[string]string, name string) bool {
	v, ok := args[name]

	return ok && (v == strTrue || v == "1" || v == "")
}

func IntFlag(args map[string]string, name string, defaultVal int) int {
	if v, ok := args[name]; ok && v != "" {
		var val int

		const base = 10

		for _, c := range v {
			if c >= '0' && c <= '9' {
				val = val*base + int(c-'0')
			} else {
				return defaultVal
			}
		}

		return val
	}

	return defaultVal
}
