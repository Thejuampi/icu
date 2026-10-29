package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestDocumentedFlagsPassUnknownFlagCheck(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../docs/cli-reference.md")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile("Invocation: `icu ([^`]*)`")
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		words := strings.Fields(m[1])
		var res, act string
		res = words[0]
		for _, w := range words[1:] {
			if !strings.HasPrefix(w, "<") && !strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "[") && !strings.HasPrefix(w, "(") {
				act = w
				break
			}
		}
		cmd, ok := registry.Lookup(res, act)
		if !ok {
			t.Logf("no command for doc invocation %s %s", res, act)
			continue
		}
		for name := range usageFlags(m[1]) {
			if cmd.Schema != nil {
				found := false
				for _, f := range cmd.Schema.Flags {
					if f.Name == name {
						found = true
					}
				}
				if !found && !isGlobalFlag(name) {
					t.Errorf("%s %s: documented --%s not in schema", res, act, name)
				}
				continue
			}
			if err := validateUsageFlags(cmd, map[string]string{name: ""}); err != nil {
				t.Errorf("%s %s: documented --%s rejected as unknown", res, act, name)
			}
		}
	}
}
