package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestZeppBackfillTestDisabledUnlessExplicit(t *testing.T) {
	for _, flags := range []map[string]string{
		nil,
		{zeppBackfillRealWriteFlag: "false"},
		{zeppBackfillRealWriteFlag: "other"},
	} {
		err := runZeppBackfillTest(nil, flags, nil, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "writes are disabled") {
			t.Fatalf("expected opt-in rejection, got %v", err)
		}
	}
}

func TestZeppBackfillTestRequiresOneFile(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"icu", "zepp", "backfill-test", "--real-write-test"}
	if code := run(args, io.Discard, &stderr); code == 0 || !strings.Contains(stderr.String(), "exactly one local FIT") {
		t.Fatalf("expected exactly one FIT error, got %q", stderr.String())
	}
}

func TestZeppBackfillTestRejectsEnvironmentTokens(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("ZEPP_LOGIN_TOKEN", "synthetic-login")
	t.Setenv("ZEPP_APP_TOKEN", "synthetic-app")
	t.Setenv("ZEPP_USER_ID", "123")
	flags := map[string]string{zeppBackfillRealWriteFlag: "", zeppBackfillFITFlag: "fixture.fit"}

	err := runZeppBackfillTest(nil, flags, nil, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no usable saved") {
		t.Fatalf("expected saved configuration requirement, got %v", err)
	}
}

func TestZeppBackfillTestRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ICU_CONFIG_DIR", dir)
	cfg := &icu.Config{ZeppLoginToken: "synthetic-login", ZeppAppToken: "synthetic-app", ZeppUserID: "123"}
	if err := icu.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	flags := map[string]string{zeppBackfillRealWriteFlag: "true", zeppBackfillFITFlag: dir}

	err := runZeppBackfillTest(nil, flags, nil, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "readable regular file") {
		t.Fatalf("expected regular file requirement, got %v", err)
	}
}
