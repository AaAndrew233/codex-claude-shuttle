GO ?= go
PNPM ?= pnpm
WAILS ?= $(shell $(GO) env GOPATH)/bin/wails

.PHONY: doctor test check build dev

doctor:
	$(WAILS) doctor

test:
	$(GO) test ./...
	$(PNPM) --dir frontend test

check:
	$(GO) vet ./...
	$(PNPM) --dir frontend check

build:
	$(WAILS) build

dev:
	$(WAILS) dev
