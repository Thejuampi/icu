package main

import (
	"fmt"
	"io"
	"os"
)

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
		fmt.Sscanf(v, "%f", &f)
		return f
	}
	return defaultVal
}

func BoolFlag(args map[string]string, name string) bool {
	v, ok := args[name]
	return ok && (v == "true" || v == "1" || v == "")
}

func StringFlag(args map[string]string, name, defaultVal string) string {
	if v, ok := args[name]; ok && v != "" {
		return v
	}
	return defaultVal
}

func IntFlag(args map[string]string, name string, defaultVal int) int {
	// simplified: values are strings in the flag map
	if v, ok := args[name]; ok && v != "" {
		var n int
		for _, c := range v {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				return defaultVal
			}
		}
		return n
	}
	return defaultVal
}
