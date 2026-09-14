GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || cat VERSION)

.PHONY: setup ui build run check test e2e

setup:
	$(GO) mod download
	npm --prefix web ci

ui:
	npm --prefix web run build

build: ui
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/textdock ./cmd/textdock

run: build
	./bin/textdock

check:
	test -z "$$($(GO) fmt ./...)"
	$(GO) vet ./...
	npm --prefix web run check
	./web/node_modules/.bin/tsc --noEmit --strict --target ES2022 --module NodeNext --moduleResolution NodeNext --lib ES2022,DOM packages/sdk/test/types.ts

test:
	$(GO) test -race -count=1 ./...
	npm --prefix packages/sdk test

e2e: build
	npm --prefix web run test:e2e
