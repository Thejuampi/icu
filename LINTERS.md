# Linter Inventory

Generated from `golangci-lint linters` on 2026-05-27.

The repository configuration now enables the supported linters explicitly instead of using `enable-all`. That keeps the effective lint surface broad while avoiding deprecated-linter warnings from golangci-lint itself.

## Enabled Linters

- asasalint
- asciicheck
- bidichk
- bodyclose
- canonicalheader
- containedctx
- contextcheck
- copyloopvar
- cyclop
- decorder
- depguard
- dogsled
- dupl
- dupword
- durationcheck
- err113
- errcheck
- errchkjson
- errname
- errorlint
- exhaustive
- exhaustruct
- exptostd
- fatcontext
- forbidigo
- forcetypeassert
- funlen
- gci
- ginkgolinter
- gocheckcompilerdirectives
- gochecknoglobals
- gochecknoinits
- gochecksumtype
- gocognit
- goconst
- gocritic
- gocyclo
- godot
- godox
- gofmt
- gofumpt
- goheader
- goimports
- gomoddirectives
- gomodguard
- goprintffuncname
- gosec
- gosimple
- gosmopolitan
- govet
- grouper
- iface
- importas
- inamedparam
- ineffassign
- interfacebloat
- intrange
- ireturn
- lll
- loggercheck
- maintidx
- makezero
- mirror
- misspell
- mnd
- musttag
- nakedret
- nestif
- nilerr
- nilnesserr
- nilnil
- nlreturn
- noctx
- nolintlint
- nonamedreturns
- nosprintfhostport
- paralleltest
- perfsprint
- prealloc
- predeclared
- promlinter
- protogetter
- reassign
- recvcheck
- revive
- rowserrcheck
- sloglint
- spancheck
- sqlclosecheck
- staticcheck
- stylecheck
- tagalign
- tagliatelle
- testableexamples
- testifylint
- testpackage
- thelper
- tparallel
- unconvert
- unparam
- unused
- usestdlibvars
- usetesting
- varnamelen
- wastedassign
- whitespace
- wrapcheck
- wsl
- zerologlint

## Deprecated Linters Reported By Golangci-lint

- deadcode
- execinquery
- exhaustivestruct
- exportloopref
- golint
- gomnd
- ifshort
- interfacer
- maligned
- nosnakecase
- scopelint
- structcheck
- tenv
- varcheck

`tenv` is deprecated and replaced by `usetesting`, so it is intentionally omitted from the explicit enabled-linter list.

## Inline Suppressions

There are currently no code-level `//nolint` directives in the Go source tree.
