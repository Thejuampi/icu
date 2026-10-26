package icu

import (
	"os"
	"path/filepath"
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
