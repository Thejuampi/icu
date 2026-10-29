package icu_test

import (
	"os"
	"testing"

	"github.com/Thejuampi/icu/internal/testnet"
)

func TestMain(m *testing.M) {
	// No test may reach a non-loopback host (real API, Open-Meteo, Zepp).
	testnet.Install()

	dir, err := os.MkdirTemp("", "icu-test-config-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	mustSetTestEnv(dir)

	os.Exit(m.Run())
}

func mustSetTestEnv(dir string) {
	if err := os.Setenv("ICU_CONFIG_DIR", dir); err != nil {
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
