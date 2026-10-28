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

// flagScan is a whole-program view of cmd/icu and the icu package that
// answers "which flag names can this code read from a flags map".
//
// It resolves names held in string constants, follows helper parameters that
// carry a flag name (queryFromFlags, stringFlagAlias, BoolFlag, icu.Resolve*,
// resolveString, ...) to a fixpoint, and treats a read through a non-constant
// key (loops over name lists such as sportIntFlags or the weather fields) as
// reading every string literal in the composite literals the loop ranges over.
type flagScan struct {
	funcs      map[string]*ast.FuncDecl // "main.f" / "icu.f"
	consts     map[string]string        // "main.c" / "icu.c"
	nameParams map[string]map[int]bool  // param positions that carry a flag name
	reads      map[string]map[string]bool
	refs       map[string]map[string]bool
}

func isStringMapType(expr ast.Expr) bool {
	m, ok := expr.(*ast.MapType)
	if !ok {
		return false
	}
	k, okK := m.Key.(*ast.Ident)
	v, okV := m.Value.(*ast.Ident)

	return okK && okV && k.Name == "string" && v.Name == "string"
}

func parsePackage(t *testing.T, dir, pkg string, scan *flagScan) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					scan.funcs[pkg+"."+d.Name.Name] = d
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || d.Tok != token.CONST {
						continue
					}
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if lit, isLit := vs.Values[i].(*ast.BasicLit); isLit && lit.Kind == token.STRING {
								if v, uerr := strconv.Unquote(lit.Value); uerr == nil {
									scan.consts[pkg+"."+name.Name] = v
								}
							}
						}
					}
				}
			}
		}
	}
}

// funcCtx is the per-function state used while walking one body.
type funcCtx struct {
	pkg        string
	flagMaps   map[string]bool            // idents holding a flags map
	params     map[string]int             // string param name -> position
	rangeNames map[string]map[string]bool // range var -> literal names it takes
	rangeParam map[string]int             // range var over a variadic name param
	skip       map[ast.Node]bool          // callee idents called without a flags map
}

func (scan *flagScan) resolveString(pkg string, expr ast.Expr) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)

		return s, err == nil && s != ""
	case *ast.Ident:
		s, ok := scan.consts[pkg+"."+v.Name]

		return s, ok
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok && x.Name == "icu" {
			s, found := scan.consts["icu."+v.Sel.Name]

			return s, found
		}
	}

	return "", false
}

func (scan *flagScan) callee(pkg string, fun ast.Expr) string {
	switch v := fun.(type) {
	case *ast.Ident:
		if _, ok := scan.funcs[pkg+"."+v.Name]; ok {
			return pkg + "." + v.Name
		}
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok && x.Name == "icu" {
			if _, found := scan.funcs["icu."+v.Sel.Name]; found {
				return "icu." + v.Sel.Name
			}
		}
	}

	return ""
}

// literalStrings returns every string literal (or constant) element in the
// composite literals under node, and in package functions node calls that
// return a composite literal (e.g. sportIntFlags()).
func (scan *flagScan) literalStrings(pkg string, node ast.Node, depth int) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			ast.Inspect(v, func(m ast.Node) bool {
				// Only literals: struct fields that hold wire-name
				// constants (sportIntFlags) are not flag names.
				if lit, ok := m.(*ast.BasicLit); ok {
					if s, found := scan.resolveString(pkg, lit); found {
						out[s] = true
					}
				}

				return true
			})
		case *ast.CallExpr:
			if name := scan.callee(pkg, v.Fun); name != "" && depth < 2 {
				for s := range scan.literalStrings(pkgOf(name), scan.funcs[name].Body, depth+1) {
					out[s] = true
				}
			}
		}

		return true
	})

	return out
}

func pkgOf(qualified string) string {
	return qualified[:strings.Index(qualified, ".")]
}

// nameArg records a flag name passed in expr: a constant is a read, a name
// parameter makes this function's parameter a name position too, a range var
// over literals reads those literals.
func (scan *flagScan) nameArg(ctx *funcCtx, expr ast.Expr, reads map[string]bool, marks map[int]bool) {
	if s, ok := scan.resolveString(ctx.pkg, expr); ok {
		reads[s] = true

		return
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if pos, isParam := ctx.params[ident.Name]; isParam {
			marks[pos] = true

			return
		}
		if pos, isRange := ctx.rangeParam[ident.Name]; isRange {
			marks[pos] = true

			return
		}
		for s := range ctx.rangeNames[ident.Name] {
			reads[s] = true
		}

		return
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if x, isIdent := sel.X.(*ast.Ident); isIdent {
			for s := range ctx.rangeNames[x.Name] {
				reads[s] = true
			}
		}
	}
}

func (scan *flagScan) isFlagMap(ctx *funcCtx, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && (ident.Name == "flags" || ctx.flagMaps[ident.Name])
}

// walk scans node, returning flag reads, referenced functions, and the
// positions of params (from ctx.params) that carry flag names.
func (scan *flagScan) walk(ctx *funcCtx, node ast.Node) (map[string]bool, map[string]bool, map[int]bool) {
	reads, refs, marks := map[string]bool{}, map[string]bool{}, map[int]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			for _, field := range v.Type.Params.List {
				if isStringMapType(field.Type) {
					for _, name := range field.Names {
						ctx.flagMaps[name.Name] = true
					}
				}
			}
		case *ast.AssignStmt:
			// Aliases: f := flags.
			for i, rhs := range v.Rhs {
				if i < len(v.Lhs) && scan.isFlagMap(ctx, rhs) {
					if id, ok := v.Lhs[i].(*ast.Ident); ok {
						ctx.flagMaps[id.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			value, ok := v.Value.(*ast.Ident)
			if !ok {
				break
			}
			if x, isIdent := v.X.(*ast.Ident); isIdent {
				if pos, isParam := ctx.params[x.Name]; isParam {
					ctx.rangeParam[value.Name] = pos

					break
				}
			}
			ctx.rangeNames[value.Name] = scan.literalStrings(ctx.pkg, v.X, 0)
		case *ast.IndexExpr:
			if scan.isFlagMap(ctx, v.X) {
				scan.nameArg(ctx, v.Index, reads, marks)
			}
		case *ast.CallExpr:
			name := scan.callee(ctx.pkg, v.Fun)
			if name == "" {
				break
			}
			hasMap := false
			for _, arg := range v.Args {
				if scan.isFlagMap(ctx, arg) {
					hasMap = true
				}
			}
			if !hasMap {
				// A helper that takes a flags map but is handed nil or a
				// literal map reads nothing the user passed.
				if hasMapParam(scan.funcs[name]) {
					ctx.skip[v.Fun] = true
				}

				break
			}
			for pos := range scan.nameParams[name] {
				for i := pos; i < len(v.Args); i++ {
					scan.nameArg(ctx, v.Args[i], reads, marks)
					if !isVariadicAt(scan.funcs[name], pos) {
						break
					}
				}
			}
		case *ast.Ident:
			if ctx.skip[v] {
				break
			}
			if _, ok := scan.funcs[ctx.pkg+"."+v.Name]; ok {
				refs[ctx.pkg+"."+v.Name] = true
			}
		case *ast.SelectorExpr:
			if ctx.skip[v] {
				return false
			}
			if name := scan.callee(ctx.pkg, v); name != "" {
				refs[name] = true
			}
		}

		return true
	})

	return reads, refs, marks
}

func hasMapParam(fn *ast.FuncDecl) bool {
	for _, field := range fn.Type.Params.List {
		if isStringMapType(field.Type) {
			return true
		}
	}

	return false
}

func isVariadicAt(fn *ast.FuncDecl, pos int) bool {
	i := 0
	for _, field := range fn.Type.Params.List {
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		if pos < i+n {
			_, ok := field.Type.(*ast.Ellipsis)

			return ok
		}
		i += n
	}

	return false
}

func newFuncCtx(pkg string, fn *ast.FuncDecl) *funcCtx {
	ctx := &funcCtx{
		pkg: pkg, flagMaps: map[string]bool{}, params: map[string]int{},
		rangeNames: map[string]map[string]bool{}, rangeParam: map[string]int{},
		skip: map[ast.Node]bool{},
	}
	pos := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if isStringMapType(field.Type) {
				ctx.flagMaps[name.Name] = true
			}
			typ := field.Type
			if e, ok := typ.(*ast.Ellipsis); ok {
				typ = e.Elt
			}
			if id, ok := typ.(*ast.Ident); ok && id.Name == "string" {
				ctx.params[name.Name] = pos
			}
			pos++
		}
		if len(field.Names) == 0 {
			pos++
		}
	}

	return ctx
}

func buildFlagScan(t *testing.T) *flagScan {
	t.Helper()
	scan := &flagScan{
		funcs: map[string]*ast.FuncDecl{}, consts: map[string]string{},
		nameParams: map[string]map[int]bool{}, reads: map[string]map[string]bool{}, refs: map[string]map[string]bool{},
	}
	parsePackage(t, ".", "main", scan)
	parsePackage(t, filepath.Join("..", ".."), "icu", scan)
	for changed := true; changed; {
		changed = false
		for name, fn := range scan.funcs {
			if fn.Body == nil {
				continue
			}
			reads, refs, marks := scan.walk(newFuncCtx(pkgOf(name), fn), fn.Body)
			scan.reads[name], scan.refs[name] = reads, refs
			if scan.nameParams[name] == nil {
				scan.nameParams[name] = map[int]bool{}
			}
			for pos := range marks {
				if !scan.nameParams[name][pos] {
					scan.nameParams[name][pos] = true
					changed = true
				}
			}
		}
	}

	return scan
}

func (scan *flagScan) closure(reads, refs map[string]bool) map[string]bool {
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
		if seen[name] || name == "main.registerAllCommands" {
			continue
		}
		seen[name] = true
		for k := range scan.reads[name] {
			out[k] = true
		}
		for k := range scan.refs[name] {
			queue = append(queue, k)
		}
	}

	return out
}

// commandReads returns the flags each registered command can read, keyed by
// "resource action".
func commandReads(t *testing.T) map[string]map[string]bool {
	t.Helper()
	scan := buildFlagScan(t)
	out := map[string]map[string]bool{}
	for name, fn := range scan.funcs {
		if pkgOf(name) != "main" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Register" {
				return true
			}
			resource, okR := scan.resolveString("main", call.Args[0])
			action, okA := scan.resolveString("main", call.Args[1])
			if !okR || !okA {
				return true
			}
			ctx := newFuncCtx("main", fn)
			reads, refs, _ := scan.walk(ctx, call.Args[2])
			out[resource+" "+action] = scan.closure(reads, refs)

			return true
		})
	}

	return out
}

// TestFlagsReadByCodeAreAccepted checks each flag the code of a registered
// command can read is accepted by validateCommandInput for that command.
func TestFlagsReadByCodeAreAccepted(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	registerAllCommands(registry)
	reads := commandReads(t)
	if len(reads) < 100 {
		t.Fatalf("scanned %d Register calls, want >= 100", len(reads))
	}
	for key, flags := range reads {
		parts := strings.SplitN(key, " ", 2)
		command, found := registry.Lookup(parts[0], parts[1])
		if !found {
			continue
		}
		var missing []string
		for flag := range flags {
			if flag == "_posargs_" || flag == "dry-run" && !strings.Contains(command.Usage, "--dry-run") {
				// Shared helpers check dry-run; commands without it reject it.
				continue
			}
			if err := validateCommandInput(command, nil, map[string]string{flag: "1"}); err != nil &&
				strings.Contains(err.Error(), "unknown flag") {
				missing = append(missing, "--"+flag)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("%s reads flags rejected by validation: %s", key, strings.Join(missing, " "))
		}
	}
}

// TestFlagScanSeesIndirectReads pins the indirect read patterns the scan has
// to follow, so a regression in the scanner cannot hide missing flags.
func TestFlagScanSeesIndirectReads(t *testing.T) {
	t.Parallel()
	reads := commandReads(t)
	want := map[string][]string{
		"zepp status":         {"zepp-login-token", "zepp-app-token", "zepp-user-id"}, // icu.Resolve* -> resolveString
		"events update":       {"desc", "description"},                                // stringFlagAlias
		"analysis plan":       {"limit"},                                              // queryFromFlags (variadic)
		"sports update":       {"ftp", "indoor-ftp", "lthr", "max-hr"},                // range sportIntFlags()
		"weather config":      {"lat", "lon"},
		"activity decoupling": {"decoupling-warmup-min"},
	}
	for key, flags := range want {
		for _, flag := range flags {
			if !reads[key][flag] {
				t.Errorf("scan missed %s --%s", key, flag)
			}
		}
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
