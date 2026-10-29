package main

import (
	"sort"
	"testing"
)

// schemaDuplicateFlags returns the flag names a schema lists more than once.
func schemaDuplicateFlags(schema *CommandSchema) []string {
	if schema == nil {
		return nil
	}
	counts := map[string]int{}
	for _, flag := range schema.Flags {
		counts[flag.Name]++
	}
	var dups []string
	for name, n := range counts {
		if n > 1 {
			dups = append(dups, name)
		}
	}
	sort.Strings(dups)

	return dups
}

// Every command's Schema must list each flag once; a duplicate means two
// definitions of the same flag (sibling of the Usage duplicate check).
func TestSchemaListsEachFlagOnce(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	for resource, actions := range registry.commands {
		for action, command := range actions {
			if dups := schemaDuplicateFlags(command.Schema); len(dups) > 0 {
				t.Errorf("%s %s schema lists %v more than once", resource, action, dups)
			}
		}
	}
	if dups := schemaDuplicateFlags(&CommandSchema{Flags: []CommandFlag{{Name: "a"}, {Name: "b"}, {Name: "a"}}}); len(dups) != 1 || dups[0] != "a" {
		t.Fatalf("detector missed a duplicate: %v", dups)
	}
}
