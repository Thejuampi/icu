package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "icu-cli-test-config-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	mustSetCommandTestEnv(dir)

	os.Exit(m.Run())
}

func mustSetCommandTestEnv(dir string) {
	if err := os.Setenv("ICU_CONFIG_DIR", dir); err != nil {
		panic(err)
	}
	// Never let a CLI test reach the real API by accident.
	if err := os.Setenv("ICU_BASE_URL", "http://127.0.0.1:1"); err != nil {
		panic(err)
	}
	if err := os.Setenv("ICU_TEST_ISOLATED_CONFIG", "1"); err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", dir); err != nil {
		panic(err)
	}
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		panic(err)
	}
}
