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
	@echo "Mutation testing requires go-mutesting: go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest"
	@# go-mutesting --exec "$(go test -exec)" ./...

ci: fmt lint vet test-cover test-race build

install: build
	go install ./cmd/icu/

clean:
	rm -rf bin/
	go clean -cache -testcache
