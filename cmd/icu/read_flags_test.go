package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// readFlagIndex maps every package-level function to the flags it reads
// directly and the package-level functions it references.
type readFlagIndex struct {
	reads map[string]map[string]bool
	refs  map[string]map[string]bool
}

func flagLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil || value == "" || strings.ContainsAny(value, " -") && strings.HasPrefix(value, "-") {
		return "", false
	}

	return value, true
}

func isFlagsIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == "flags"
}

// collectReads walks node and records flags["x"] and f(flags, "x", ...) reads
// plus references to package-level functions.
func collectReads(node ast.Node, funcs map[string]bool, reads, refs map[string]bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IndexExpr:
			if isFlagsIdent(v.X) {
				if name, ok := flagLiteral(v.Index); ok {
					reads[name] = true
				}
			}
		case *ast.CallExpr:
			if len(v.Args) >= 2 && isFlagsIdent(v.Args[0]) {
				// Only the flag name: later literals are defaults.
				if name, ok := flagLiteral(v.Args[1]); ok {
					reads[name] = true
				}
			}
		case *ast.Ident:
			if funcs[v.Name] {
				refs[v.Name] = true
			}
		}

		return true
	})
}

func buildReadFlagIndex(t *testing.T) (readFlagIndex, []*ast.File) {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	funcs := map[string]bool{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		files = append(files, file)
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				funcs[fn.Name.Name] = true
			}
		}
	}
	index := readFlagIndex{reads: map[string]map[string]bool{}, refs: map[string]map[string]bool{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			reads, refs := map[string]bool{}, map[string]bool{}
			collectReads(fn.Body, funcs, reads, refs)
			index.reads[fn.Name.Name] = reads
			index.refs[fn.Name.Name] = refs
		}
	}

	return index, files
}

func (index readFlagIndex) closure(reads, refs map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k := range reads {
		out[k] = true
	}
	seen := map[string]bool{}
	queue := make([]string, 0, len(refs))
	for k := range refs {
		queue = append(queue, k)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] || name == "registerAllCommands" {
			continue
		}
		seen[name] = true
		for k := range index.reads[name] {
			out[k] = true
		}
		for k := range index.refs[name] {
			queue = append(queue, k)
		}
	}

	return out
}

// TestFlagsReadByCodeAreAccepted scans every registry.Register call, follows
// the package functions it reaches, and checks each flag literal the code
// reads is accepted by validateCommandInput for that command.
func TestFlagsReadByCodeAreAccepted(t *testing.T) {
	t.Parallel()
	index, files := buildReadFlagIndex(t)
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	funcs := map[string]bool{}
	for name := range index.reads {
		funcs[name] = true
	}
	checked := 0
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Register" {
				return true
			}
			resource, okR := flagLiteral(call.Args[0])
			action, okA := flagLiteral(call.Args[1])
			if !okR || !okA {
				return true
			}
			command, found := registry.Lookup(resource, action)
			if !found {
				return true
			}
			reads, refs := map[string]bool{}, map[string]bool{}
			collectReads(call.Args[2], funcs, reads, refs)
			var missing []string
			for flag := range index.closure(reads, refs) {
				if flag == "_posargs_" {
					continue
				}
				if flag == "dry-run" && !strings.Contains(command.Usage, "--dry-run") {
					// Shared helpers check dry-run; commands without it reject it.
					continue
				}
				if err := validateCommandInput(command, nil, map[string]string{flag: "1"}); err != nil && strings.Contains(err.Error(), "unknown flag") {
					missing = append(missing, "--"+flag)
				}
			}
			checked++
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf("%s %s reads flags rejected by validation: %s", resource, action, strings.Join(missing, " "))
			}

			return true
		})
	}
	if checked < 100 {
		t.Fatalf("checked %d Register calls, want >= 100", checked)
	}
}

func TestAnalysisCoachingAcceptsOutputJSON(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	command, ok := registry.Lookup("analysis", "coaching")
	if !ok {
		t.Fatal("analysis coaching not registered")
	}
	if err := validateCommandInput(command, nil, map[string]string{"output": "json"}); err != nil {
		t.Fatalf("analysis coaching --output json rejected: %v", err)
	}
}

// TestAnalysisTimezoneAndLimitFlagsAccepted pins the flags #25's timezone
// work reads, so TestFlagsReadByCodeAreAccepted keeps passing after it lands.
func TestAnalysisTimezoneAndLimitFlagsAccepted(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	cases := map[string][]string{
		"coaching":   {"timezone"},
		"wellness":   {"timezone"},
		"adaptation": {"timezone"},
		"plan":       {"timezone", "limit"},
		"cycling":    {"timezone"},
		"micro":      {"limit"},
		"microcycle": {"limit"},
	}
	for action, flagNames := range cases {
		command, ok := registry.Lookup("analysis", action)
		if !ok {
			t.Fatalf("analysis %s not registered", action)
		}
		for _, flag := range flagNames {
			value := "1"
			if flag == "timezone" {
				value = "America/New_York"
			}
			if err := validateCommandInput(command, nil, map[string]string{flag: value}); err != nil {
				t.Errorf("analysis %s --%s rejected: %v", action, flag, err)
			}
		}
	}
}
