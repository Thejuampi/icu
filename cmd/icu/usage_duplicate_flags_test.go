package main

import (
	"sort"
	"testing"
)

// Every command's Usage (including appended undocumented flags) must list
// each flag once; a duplicate means two sources document the same flag.
func TestUsageListsEachFlagOnce(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	for resource, actions := range registry.commands {
		for action, command := range actions {
			counts := map[string]int{}
			for _, match := range usageFlagPattern.FindAllStringSubmatch(command.Usage, -1) {
				counts[match[1]]++
			}
			var dups []string
			for name, n := range counts {
				if n > 1 {
					dups = append(dups, name)
				}
			}
			sort.Strings(dups)
			if len(dups) > 0 {
				t.Errorf("%s %s usage lists %v more than once: %q", resource, action, dups, command.Usage)
			}
		}
	}
}
