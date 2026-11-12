BINARY   := nexus-cli
DIST     := ./dist
MODULE   := github.com/EugenMayer/nexus-cli
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

export CGO_ENABLED := 0

.PHONY: all build dist test vet fmt fmt-check publish-module clean

all: vet test build

## build: build for the current platform into ./dist/nexus-cli
build:
	@mkdir -p $(DIST)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) .

## dist: cross-compile all release platforms into ./dist and write checksums
dist:
	@mkdir -p $(DIST)
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		out=$(DIST)/$(BINARY)-$$os-$$arch$$ext; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o $$out .; \
	done
	cd $(DIST) && sha256sum $(BINARY)-* > checksums.txt

test:
	CGO_ENABLED=1 go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

fmt-check:
	@test -z "$$(gofmt -s -l .)" || (echo "not gofmt -s clean:"; gofmt -s -l .; exit 1)

## publish-module: make the tagged version known to the Go module proxy (pkg.go.dev, go get)
publish-module:
	GOPROXY=https://proxy.golang.org GOFLAGS=-mod=mod go list -m $(MODULE)@$(VERSION)

clean:
	rm -rf $(DIST)
