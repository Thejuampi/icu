package main

import (
	"go/ast"
	"go/build"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// flagScan is a whole-program, type-checked view of cmd/icu and the icu
// package that answers "which flag names can this code read from a flags map".
//
// Calls, constants and variables are linked by their go/types object, not by
// name, so a local func value that shadows a package helper, an alias of the
// flags map, or a builtin such as delete is never mistaken for a read.
//
// It resolves names held in string constants, follows helper parameters that
// carry a flag name (queryFromFlags, stringFlagAlias, BoolFlag, icu.Resolve*,
// resolveString, ...) to a fixpoint, and treats a read through a non-constant
// key (loops over name lists such as sportIntFlags or the weather fields) as
// reading every string literal in the composite literals the loop ranges over.
type flagScan struct {
	info       *types.Info
	funcs      map[*types.Func]*ast.FuncDecl // package-level funcs with bodies
	nameParams map[*types.Func]map[int]bool  // param positions that carry a flag name
	reads      map[*types.Func]map[string]bool
	refs       map[*types.Func]map[*types.Func]bool
	pkgs       map[string]*types.Package
}

func isStringMap(typ types.Type) bool {
	m, ok := typ.Underlying().(*types.Map)
	if !ok {
		return false
	}

	return isString(m.Key()) && isString(m.Elem())
}

func isString(typ types.Type) bool {
	b, ok := typ.Underlying().(*types.Basic)

	return ok && b.Kind() == types.String
}

// scanImporter type-checks the scanned packages from source, imports the
// standard library from GOROOT sources, and hands back an empty package for
// anything else (golang.org/x/term), whose uses never carry flag names.
type scanImporter struct {
	std  types.Importer
	pkgs map[string]*types.Package
}

func (imp *scanImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := imp.pkgs[path]; ok {
		return pkg, nil
	}
	if !strings.Contains(strings.Split(path, "/")[0], ".") {
		return imp.std.Import(path)
	}
	pkg := types.NewPackage(path, path[strings.LastIndex(path, "/")+1:])
	pkg.MarkComplete()
	imp.pkgs[path] = pkg

	return pkg, nil
}

type scanPackage struct{ path, dir string }

// loadFlagScan parses and type-checks pkgs (dependencies first) and builds
// the scan over their non-test files.
func loadFlagScan(t *testing.T, pkgs ...scanPackage) *flagScan {
	t.Helper()
	fset := token.NewFileSet()
	scan := &flagScan{
		info: &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{},
			Defs:  map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{},
		},
		funcs: map[*types.Func]*ast.FuncDecl{}, nameParams: map[*types.Func]map[int]bool{},
		reads: map[*types.Func]map[string]bool{}, refs: map[*types.Func]map[*types.Func]bool{},
		pkgs: map[string]*types.Package{},
	}
	imp := &scanImporter{
		std:  importer.ForCompiler(fset, "source", nil),
		pkgs: scan.pkgs,
	}
	for _, p := range pkgs {
		bp, err := build.ImportDir(p.dir, 0)
		if err != nil {
			t.Fatal(err)
		}
		files := make([]*ast.File, 0, len(bp.GoFiles))
		for _, name := range bp.GoFiles {
			file, parseErr := parser.ParseFile(fset, filepath.Join(p.dir, name), nil, 0)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			files = append(files, file)
		}
		var typeErrs []string
		conf := types.Config{Importer: imp, Error: func(err error) {
			// Only the stubbed third-party packages may fail to resolve.
			if !strings.Contains(err.Error(), "term.") {
				typeErrs = append(typeErrs, err.Error())
			}
		}}
		pkg, _ := conf.Check(p.path, fset, files, scan.info)
		if len(typeErrs) > 0 {
			t.Fatalf("type-check %s: %s", p.path, strings.Join(typeErrs, "; "))
		}
		scan.pkgs[p.path] = pkg
		for _, file := range files {
			for _, decl := range file.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
					if fn, isFunc := scan.info.Defs[fd.Name].(*types.Func); isFunc {
						scan.funcs[fn] = fd
					}
				}
			}
		}
	}
	scan.fixpoint()

	return scan
}

// funcCtx is the per-function state used while walking one body.
type funcCtx struct {
	flagMaps   map[types.Object]bool            // vars holding a flags map
	params     map[types.Object]int             // string param -> position
	rangeNames map[types.Object]map[string]bool // range var -> literal names it takes
	rangeParam map[types.Object]int             // range var over a variadic name param
	skip       map[ast.Node]bool                // callee exprs called without a flags map
}

// resolveString returns the value of a constant string expression: a literal,
// a named constant from any package, or a constant concatenation.
func (scan *flagScan) resolveString(expr ast.Expr) (string, bool) {
	tv, ok := scan.info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	s := constant.StringVal(tv.Value)

	return s, s != ""
}

func (scan *flagScan) objectOf(expr ast.Expr) types.Object {
	switch v := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return scan.info.ObjectOf(v)
	case *ast.SelectorExpr:
		return scan.info.ObjectOf(v.Sel)
	}

	return nil
}

// callee returns the scanned package-level function fun refers to. Builtins
// (delete, len, ...), methods, func values and unscanned packages yield nil.
func (scan *flagScan) callee(fun ast.Expr) *types.Func {
	fn, ok := scan.objectOf(fun).(*types.Func)
	if !ok {
		return nil
	}
	if _, scanned := scan.funcs[fn]; !scanned {
		return nil
	}

	return fn
}

// literalStrings returns every string literal (or constant) element in the
// composite literals under node, and in package functions node calls that
// return a composite literal (e.g. sportIntFlags()).
func (scan *flagScan) literalStrings(node ast.Node, depth int) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			ast.Inspect(v, func(m ast.Node) bool {
				// Only literals: struct fields that hold wire-name
				// constants (sportIntFlags) are not flag names.
				if lit, ok := m.(*ast.BasicLit); ok {
					if s, found := scan.resolveString(lit); found {
						out[s] = true
					}
				}

				return true
			})
		case *ast.CallExpr:
			if fn := scan.callee(v.Fun); fn != nil && depth < 2 {
				for s := range scan.literalStrings(scan.funcs[fn].Body, depth+1) {
					out[s] = true
				}
			}
		}

		return true
	})

	return out
}

// nameArg records a flag name passed in expr: a constant is a read, a name
// parameter makes this function's parameter a name position too, a range var
// over literals reads those literals.
func (scan *flagScan) nameArg(ctx *funcCtx, expr ast.Expr, reads map[string]bool, marks map[int]bool) {
	if s, ok := scan.resolveString(expr); ok {
		reads[s] = true

		return
	}
	expr = ast.Unparen(expr)
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		expr = sel.X
	}
	obj := scan.objectOf(expr)
	if obj == nil {
		return
	}
	if pos, isParam := ctx.params[obj]; isParam {
		marks[pos] = true

		return
	}
	if pos, isRange := ctx.rangeParam[obj]; isRange {
		marks[pos] = true

		return
	}
	for s := range ctx.rangeNames[obj] {
		reads[s] = true
	}
}

func (scan *flagScan) isFlagMap(ctx *funcCtx, expr ast.Expr) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)

	return ok && ctx.flagMaps[scan.info.ObjectOf(ident)]
}

func (scan *flagScan) addFlagMapParams(ctx *funcCtx, params *ast.FieldList) {
	for _, field := range params.List {
		for _, name := range field.Names {
			if obj := scan.info.Defs[name]; obj != nil && isStringMap(obj.Type()) {
				ctx.flagMaps[obj] = true
			}
		}
	}
}

// walk scans node, returning flag reads, referenced functions, and the
// positions of params (from ctx.params) that carry flag names.
func (scan *flagScan) walk(ctx *funcCtx, node ast.Node) (map[string]bool, map[*types.Func]bool, map[int]bool) {
	reads, refs, marks := map[string]bool{}, map[*types.Func]bool{}, map[int]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			scan.addFlagMapParams(ctx, v.Type.Params)
		case *ast.AssignStmt:
			// Aliases: f := flags.
			for i, rhs := range v.Rhs {
				if i < len(v.Lhs) && len(v.Lhs) == len(v.Rhs) && scan.isFlagMap(ctx, rhs) {
					if obj := scan.objectOf(v.Lhs[i]); obj != nil {
						ctx.flagMaps[obj] = true
					}
				}
			}
		case *ast.RangeStmt:
			value := scan.objectOf(v.Value)
			if value == nil {
				break
			}
			if pos, isParam := ctx.params[scan.objectOf(v.X)]; isParam {
				ctx.rangeParam[value] = pos

				break
			}
			ctx.rangeNames[value] = scan.literalStrings(v.X, 0)
		case *ast.IndexExpr:
			if scan.isFlagMap(ctx, v.X) {
				scan.nameArg(ctx, v.Index, reads, marks)
			}
		case *ast.CallExpr:
			scan.walkCall(ctx, v, reads, marks)
		case *ast.Ident:
			if ctx.skip[v] {
				break
			}
			if fn := scan.callee(v); fn != nil {
				refs[fn] = true
			}
		case *ast.SelectorExpr:
			if ctx.skip[v] {
				return false
			}
		}

		return true
	})

	return reads, refs, marks
}

func (scan *flagScan) walkCall(ctx *funcCtx, call *ast.CallExpr, reads map[string]bool, marks map[int]bool) {
	fn := scan.callee(call.Fun)
	if fn == nil {
		return
	}
	hasMap := false
	for _, arg := range call.Args {
		if scan.isFlagMap(ctx, arg) {
			hasMap = true
		}
	}
	if !hasMap {
		// A helper that takes a flags map but is handed nil or a
		// literal map reads nothing the user passed.
		if hasMapParam(fn) {
			ctx.skip[ast.Unparen(call.Fun)] = true
		}

		return
	}
	sig := fn.Signature()
	for pos := range scan.nameParams[fn] {
		for i := pos; i < len(call.Args); i++ {
			scan.nameArg(ctx, call.Args[i], reads, marks)
			if !sig.Variadic() || pos != sig.Params().Len()-1 {
				break
			}
		}
	}
}

func hasMapParam(fn *types.Func) bool {
	params := fn.Signature().Params()
	for i := range params.Len() {
		if isStringMap(params.At(i).Type()) {
			return true
		}
	}

	return false
}

func (scan *flagScan) newFuncCtx(fn *ast.FuncDecl) *funcCtx {
	ctx := &funcCtx{
		flagMaps: map[types.Object]bool{}, params: map[types.Object]int{},
		rangeNames: map[types.Object]map[string]bool{}, rangeParam: map[types.Object]int{},
		skip: map[ast.Node]bool{},
	}
	scan.addFlagMapParams(ctx, fn.Type.Params)
	pos := 0
	for _, field := range fn.Type.Params.List {
		_, variadic := field.Type.(*ast.Ellipsis)
		for _, name := range field.Names {
			obj := scan.info.Defs[name]
			typ := obj.Type()
			if s, ok := typ.(*types.Slice); ok && variadic {
				typ = s.Elem()
			}
			if isString(typ) {
				ctx.params[obj] = pos
			}
			pos++
		}
		if len(field.Names) == 0 {
			pos++
		}
	}

	return ctx
}

func (scan *flagScan) fixpoint() {
	for changed := true; changed; {
		changed = false
		for fn, decl := range scan.funcs {
			reads, refs, marks := scan.walk(scan.newFuncCtx(decl), decl.Body)
			scan.reads[fn], scan.refs[fn] = reads, refs
			if scan.nameParams[fn] == nil {
				scan.nameParams[fn] = map[int]bool{}
			}
			for pos := range marks {
				if !scan.nameParams[fn][pos] {
					scan.nameParams[fn][pos] = true
					changed = true
				}
			}
		}
	}
}

func (scan *flagScan) closure(reads map[string]bool, refs map[*types.Func]bool, stop *types.Func) map[string]bool {
	out := map[string]bool{}
	for k := range reads {
		out[k] = true
	}
	seen := map[*types.Func]bool{}
	queue := make([]*types.Func, 0, len(refs))
	for k := range refs {
		queue = append(queue, k)
	}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if seen[fn] || fn == stop {
			continue
		}
		seen[fn] = true
		for k := range scan.reads[fn] {
			out[k] = true
		}
		for k := range scan.refs[fn] {
			queue = append(queue, k)
		}
	}

	return out
}

const icuImportPath = "github.com/Thejuampi/icu"

// commandReads returns the flags each registered command can read, keyed by
// "resource action", from a type-checked scan of cmd/icu and the icu package.
func commandReads(t *testing.T) map[string]map[string]bool {
	t.Helper()
	scan := loadFlagScan(t,
		scanPackage{icuImportPath, filepath.Join("..", "..")},
		scanPackage{icuImportPath + "/cmd/icu", "."})

	return scan.registeredReads()
}

func (scan *flagScan) registeredReads() map[string]map[string]bool {
	mainPkg := scan.pkgs[icuImportPath+"/cmd/icu"]
	stop, _ := mainPkg.Scope().Lookup("registerAllCommands").(*types.Func)
	out := map[string]map[string]bool{}
	for fn, decl := range scan.funcs {
		if fn.Pkg() != mainPkg {
			continue
		}
		ast.Inspect(decl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			method, isFunc := scan.info.Uses[sel.Sel].(*types.Func)
			if !isFunc || method.Name() != "Register" || method.Pkg() != mainPkg {
				return true
			}
			resource, okR := scan.resolveString(call.Args[0])
			action, okA := scan.resolveString(call.Args[1])
			if !okR || !okA {
				return true
			}
			reads, refs, _ := scan.walk(scan.newFuncCtx(decl), call.Args[2])
			out[resource+" "+action] = scan.closure(reads, refs, stop)

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

// TestFlagScanLinksByObject pins that the scan ignores builtins and links
// calls by object: delete(flags, ...) is not a read, and a local func value
// that shadows a package helper's name does not inherit its name parameter.
func TestFlagScanLinksByObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := `package p

const keyName = "via-const"

func read(flags map[string]string, name string) string { return flags[name] }

func run(flags map[string]string) {
	delete(flags, "deleted")
	_ = read(flags, keyName)
	f := flags
	_ = f["via-alias"]
	{
		read := func(m map[string]string, s string) string { return "" }
		_ = read(flags, "shadowed")
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	scan := loadFlagScan(t, scanPackage{"example.com/p", dir})
	run, _ := scan.pkgs["example.com/p"].Scope().Lookup("run").(*types.Func)
	got := scan.closure(scan.reads[run], scan.refs[run], nil)
	for _, want := range []string{"via-const", "via-alias"} {
		if !got[want] {
			t.Errorf("scan missed %q: %v", want, got)
		}
	}
	for _, unwanted := range []string{"deleted", "shadowed"} {
		if got[unwanted] {
			t.Errorf("scan counted %q as a read: %v", unwanted, got)
		}
	}
}
