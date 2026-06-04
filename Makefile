BINARY := vid-summary-cli
ALIAS  := video-summary
PKG    := ./cmd/vid-summary-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)
GOBIN  := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN  := $(shell go env GOPATH)/bin
endif

.PHONY: build install clean dist tidy test

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)
	ln -sf $(BINARY) bin/$(ALIAS)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)
	ln -sf $(BINARY) $(GOBIN)/$(ALIAS)

tidy:
	go mod tidy

test:
	go test ./...

clean:
	rm -rf bin dist

dist:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 $(PKG)
	GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64  $(PKG)
	GOOS=linux  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64  $(PKG)
