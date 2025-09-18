SHELL := /bin/bash
PROJECT := santa-sleigh
PACKAGE := github.com/kallsyms/santa-sleigh
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PACKAGE)/internal/daemon.version=$(VERSION)
DIST_DIR := dist

GO ?= go

.PHONY: all build clean build-linux build-macos test

all: build

build:
	mkdir -p $(DIST_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(PROJECT) ./cmd/$(PROJECT)

build-linux:
	mkdir -p $(DIST_DIR)/linux
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux/$(PROJECT) ./cmd/$(PROJECT)

build-macos:
	mkdir -p $(DIST_DIR)/darwin
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/darwin/$(PROJECT) ./cmd/$(PROJECT)
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/darwin/$(PROJECT)-amd64 ./cmd/$(PROJECT)

clean:
	rm -rf $(DIST_DIR)

lint:
	$(GO) fmt ./...

test:
	$(GO) test ./...
