package icu

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestSkillCopiesInSync keeps the .claude skill pack identical to the
// canonical .agents pack; edit .agents and copy it over.
func TestSkillCopiesInSync(t *testing.T) {
	t.Parallel()
	const canonical, mirror = ".agents/skills", ".claude/skills"
	seen := map[string]bool{}
	err := filepath.WalkDir(canonical, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(canonical, path)
		seen[rel] = true
		want, _ := os.ReadFile(path)                            //nolint:gosec // repository file
		got, readErr := os.ReadFile(filepath.Join(mirror, rel)) //nolint:gosec // repository file
		if readErr != nil || !bytes.Equal(want, got) {
			t.Errorf("%s/%s differs from %s/%s; copy .agents/skills over .claude/skills", mirror, rel, canonical, rel)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = filepath.WalkDir(mirror, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if rel, _ := filepath.Rel(mirror, path); !seen[rel] {
				t.Errorf("%s has no canonical copy in %s", path, canonical)
			}
		}

		return nil
	})
}
