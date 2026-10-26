package icu

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigWriteFileAtomic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(path)
	if len(entries) != 1 || string(data) != "new" {
		t.Fatalf("entries %v data %q", entries, data)
	}
	if err := os.MkdirAll(filepath.Join(dir, "busy", "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(filepath.Join(dir, "busy"), []byte("x"), 0o600); err == nil {
		t.Fatal("rename over a directory succeeded")
	}
	if err := writeFileAtomic(filepath.Join(dir, "missing", "c.json"), []byte("x"), 0o600); err == nil {
		t.Fatal("write into a missing dir succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("leftovers: %v", entries)
	}
}

// TestFileConfigStoreSaveIsAtomic fails if Save goes back to os.WriteFile:
// an in-place write keeps the inode, so a hard link to config.json would see
// the new bytes, while writeFileAtomic renames a fresh file over the name.
func TestFileConfigStoreSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	store := NewFileConfigStore()
	store.getenv = func(key string) string {
		if key == configDirEnvVar {
			return dir
		}

		return os.Getenv(key)
	}
	if reflect.ValueOf(store.writeFile).Pointer() != reflect.ValueOf(writeFileAtomic).Pointer() {
		t.Fatal("FileConfigStore must write config.json with writeFileAtomic")
	}
	if err := store.Save(&Config{}, "first"); err != nil {
		t.Fatal(err)
	}
	path := store.ConfigPath()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}
	if err := store.Save(&Config{AthleteID: "i42"}, "second"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(alias); string(after) != string(before) {
		t.Fatalf("config.json was rewritten in place (not atomic): alias now %q", after)
	}
	if now, _ := os.ReadFile(path); !strings.Contains(string(now), "i42") {
		t.Fatalf("config not saved: %q", now)
	}
}
