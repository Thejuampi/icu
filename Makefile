.PHONY: build test test-cover test-race lint lint-fix fmt vet mutation ci clean install

BIN := bin/icu
ifeq ($(OS),Windows_NT)
	BIN := bin/icu.exe
endif

build:
	go build -o $(BIN) ./cmd/icu/

test:
	go test ./... -count=1

test-cover:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out
	@echo "---"
	@go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'

test-race:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

fmt:
	gofumpt -w . ./cmd/icu/
	goimports -w . ./cmd/icu/

vet:
	go vet ./...

mutation:
	@echo "Running mutation testing on functional core (auth.go, urls.go, output.go)"
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$git = Get-Command git -ErrorAction SilentlyContinue; if (-not $$git) { throw 'git is required for mutation testing on Windows' }; $$gitRoot = Split-Path (Split-Path $$git.Source -Parent) -Parent; $$diffPath = Join-Path $$gitRoot 'usr\\bin'; if (Test-Path $$diffPath) { $$env:PATH = \"$$diffPath;$$env:PATH\" }; go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest; go-mutesting auth.go urls.go output.go"
else
	@go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest
	@go-mutesting auth.go urls.go output.go
endif

ci: fmt lint vet test-cover test-race build

install: build
	go install ./cmd/icu/

clean:
	rm -rf bin/
	go clean -cache -testcache
