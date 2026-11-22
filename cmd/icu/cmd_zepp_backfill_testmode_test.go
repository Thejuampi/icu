package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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

func TestZeppBackfillPreviewOffline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private ride.fit")
	// FIT header, timestamp-only record, virtual cycling session, and CRC.
	const syntheticFIT = "0c200000390000002e464954400000140001fd048600eb030000410000120006fd048602048605010006010008048609048601f0030000e8030000023a70170000c4090000df75"
	fit, err := hex.DecodeString(syntheticFIT)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, fit, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ICU_CONFIG_DIR", path) // Not a directory: configuration is unavailable.
	previous := newCommandClient
	t.Cleanup(func() { newCommandClient = previous })
	newCommandClient = func(_, _, _ string) *icu.Client {
		t.Fatal("offline preview constructed an Intervals.icu client")
		return nil
	}

	var stdout, stderr bytes.Buffer
	args := []string{"icu", "zepp", "backfill-test", "--preview", "--fit", path}
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("offline preview failed: %s", stderr.String())
	}

	var preview icu.ZeppRideFITPreview
	if err := json.Unmarshal(stdout.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Detail) == 0 || len(preview.Summary) == 0 ||
		preview.FITRecords != 1 || preview.AlignedRecords != 1 || preview.GPSRecords != 0 ||
		strings.Contains(stdout.String(), path) || strings.Contains(stdout.String(), "631066600") ||
		strings.Contains(stdout.String(), "undo_verified") {
		t.Fatal("missing preview sections or unexpected sensitive/write output")
	}
}

func TestZeppBackfillPreviewRejectsUnsafeInvocation(t *testing.T) {
	badFIT := filepath.Join(t.TempDir(), "not-a-ride.fit")
	if err := os.WriteFile(badFIT, []byte("not a FIT"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		flags []string
		want  string
	}{
		{[]string{"--preview", "--real-write-test", "--fit", "a.fit"}, "mutually exclusive"},
		{[]string{"--preview", "--real-write-test=false", "--fit", "a.fit"}, "mutually exclusive"},
		{[]string{"--preview=false", "--fit", "a.fit"}, "--preview must be enabled"},
		{[]string{"--preview"}, "exactly one local FIT"},
		{[]string{"--preview", "--fit", "a.fit", "b.fit"}, "exactly one local FIT"},
		{[]string{"--preview", "--fit", t.TempDir()}, "readable regular file"},
		{[]string{"--preview", "--fit", badFIT}, "invalid FIT header"},
		{[]string{"--preview", "--fit", filepath.Join(t.TempDir(), "missing", "a.fit")}, "could not open the FIT file directory"},
	} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"icu", "zepp", "backfill-test"}, tc.flags...)
		if code := run(args, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), tc.want) || stdout.Len() != 0 {
			t.Fatalf("expected %q and no output, got code %d, stdout %q, stderr %q", tc.want, code, &stdout, &stderr)
		}
	}
}
