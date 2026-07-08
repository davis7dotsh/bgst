.PHONY: fmt fmt-check lint test check build cross-compile install release

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || true)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	@test -z "$$(gofmt -l ./cmd ./internal)" || (gofmt -l ./cmd ./internal && exit 1)

lint:
	go vet ./...

test:
	go test ./...

check: fmt-check lint test

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/bgst ./cmd/bgst

install:
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/bgst

cross-compile:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/bgst-linux-amd64 ./cmd/bgst
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/bgst-linux-arm64 ./cmd/bgst
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/bgst-darwin-amd64 ./cmd/bgst
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/bgst-darwin-arm64 ./cmd/bgst
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/bgst-windows-amd64.exe ./cmd/bgst
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/bgst-windows-arm64.exe ./cmd/bgst

release:
	@test -n "$(VERSION)" && test "$(VERSION)" != "dev" || (echo "VERSION is required, for example: make release VERSION=v0.1.0" && exit 1)
	depot ci dispatch --org kfmrjsn0w2 --repo davis7dotsh/bgst --workflow release.yml --ref main --input version=$(VERSION)
