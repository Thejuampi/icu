package main

import (
	"os"
	"testing"

	"github.com/Thejuampi/icu/internal/testnet"
)

func TestMain(m *testing.M) {
	// No test may reach a non-loopback host.
	testnet.Install()

	os.Exit(m.Run())
}
