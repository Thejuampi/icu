package icu_test

import (
	"os"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestSaveConfigWritesAuditLog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ICU_CONFIG_DIR", dir)

	err := icu.SaveConfigWithAction(&icu.Config{AthleteID: "i123"}, "config_set")
	if err != nil {
		t.Fatalf("SaveConfigWithAction: %v", err)
	}

	raw, readErr := os.ReadFile(icu.ConfigAuditPath())
	if readErr != nil {
		t.Fatalf("ReadFile(audit): %v", readErr)
	}
	got := string(raw)
	if !strings.Contains(got, `"action":"config_set"`) || !strings.Contains(got, `"changedFields":["athleteId"]`) {
		t.Fatalf("audit log = %q, want config_set audit entry for athleteId", got)
	}
}

func TestConfigPathPanicsWithoutIsolatedTestEnv(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", "")
	t.Setenv("ICU_TEST_ISOLATED_CONFIG", "")

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic for unsafe config path access during tests")
		}
	}()

	_ = icu.ConfigPath()
}

func TestSaveConfigAuditsZeppAuthMutationInIsolatedTestEnv(t *testing.T) {
	t.Setenv("ICU_CONFIG_DIR", t.TempDir())
	t.Setenv("ICU_TEST_ISOLATED_CONFIG", "1")

	err := icu.SaveConfigWithAction(&icu.Config{ZeppLoginToken: "secret"}, "zepp_login")
	if err != nil {
		t.Fatalf("SaveConfigWithAction: %v", err)
	}

	raw, readErr := os.ReadFile(icu.ConfigAuditPath())
	if readErr != nil {
		t.Fatalf("ReadFile(audit): %v", readErr)
	}
	got := string(raw)
	if !strings.Contains(got, `"action":"zepp_login"`) || !strings.Contains(got, `"secretMutations":["zeppLoginToken:set"]`) {
		t.Fatalf("audit log = %q, want Zepp auth mutation audit entry", got)
	}
}

func TestMemoryConfigStoreRoundTripAndAudit(t *testing.T) {
	t.Parallel()

	store := icu.NewMemoryConfigStore()
	if store.ConfigPath() == "" || store.AuditPath() == "" {
		t.Fatal("memory store paths must be non-empty")
	}

	restore := icu.SetConfigStoreForTesting(store)
	t.Cleanup(restore)

	if err := icu.SaveConfigWithAction(&icu.Config{AthleteID: "i999"}, "memory_set"); err != nil {
		t.Fatalf("SaveConfigWithAction: %v", err)
	}
	cfg, err := icu.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.AthleteID != "i999" {
		t.Fatalf("AthleteID = %q, want i999", cfg.AthleteID)
	}

	entries := store.AuditEntries()
	if len(entries) == 0 {
		t.Fatal("expected audit entries after save")
	}
	if entries[0].Action != "memory_set" {
		t.Fatalf("action = %q, want memory_set", entries[0].Action)
	}

	if err := store.Save(nil, "nil"); err == nil {
		t.Fatal("Save(nil) should error")
	}
}
