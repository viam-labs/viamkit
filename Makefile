# Makefile for viamkit — a pure-Go toolkit for building Viam modules.
#
# `make check` runs the full suite that CI runs. `make hooks` wires up
# the repo's git pre-commit hook (run once after cloning).

GOLANGCI_LINT_VERSION := v2.11.4
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: all check build test lint fmt tidy hooks

# Default target: the full check suite.
all: check

# check runs build + lint + test — identical to what CI runs.
check: build lint test

# build compiles every package.
build:
	go build ./...

# test runs the suite with the race detector enabled.
test:
	go test -race ./...

# lint reports golangci-lint findings without modifying files.
lint:
	$(GOLANGCI_LINT) run ./...

# fmt applies gofmt + goimports formatting in place.
fmt:
	$(GOLANGCI_LINT) fmt ./...

# tidy prunes and syncs go.mod / go.sum.
tidy:
	go mod tidy

# hooks installs the repo's git hooks. Run once after cloning.
hooks:
	git config core.hooksPath .githooks
	@echo "git hooks installed (core.hooksPath = .githooks)"
