APP_NAME := ado
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build install clean release snapshot cross

## Build for current platform
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

## Install to /usr/local/bin
install: build
	sudo cp $(APP_NAME) /usr/local/bin/$(APP_NAME)

## Cross-compile for all platforms
cross:
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_linux_amd64/$(APP_NAME) .
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_linux_arm64/$(APP_NAME) .
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_darwin_amd64/$(APP_NAME) .
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_darwin_arm64/$(APP_NAME) .
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_windows_amd64/$(APP_NAME).exe .

## Release with goreleaser (requires git tag)
release:
	goreleaser release --clean

## Local snapshot build (no tag needed)
snapshot:
	goreleaser release --snapshot --clean

## Clean build artifacts
clean:
	rm -f $(APP_NAME)
	rm -rf dist/
