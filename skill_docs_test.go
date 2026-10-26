package icu

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Both copies of the intervals-icu agent skill must carry the verify-after-
// write rule and must not show `--desc "...\n..."` examples, which the CLI
// now rejects.
func TestIntervalsSkillRequiresVerifyAfterWrite(t *testing.T) {
	t.Parallel()
	literalDesc := regexp.MustCompile(`--desc "[^"]*\\n`)
	for _, path := range []string{".agents/skills/intervals-icu/SKILL.md", ".claude/skills/intervals-icu/SKILL.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		if !strings.Contains(text, "After every write") || !strings.Contains(text, "re-read what you wrote") {
			t.Errorf("%s: missing verify-after-write rule", path)
		}
		if literalDesc.MatchString(text) {
			t.Errorf("%s: has a --desc example with a literal \\n", path)
		}
	}
}
