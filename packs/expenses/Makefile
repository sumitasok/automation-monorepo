# Makefile for the expenses automation pack (Go app, pure stdlib).
#
# Two ways to run:
#   * Direct  (make update-event / dry-run) -> plain `go run . update-event`,
#     fast dev loop. Reads DEEPSEEK_* env from your shell and config/events.json.
#   * Via auto (make auto-update-event) -> workspace `./auto run expenses-update-event`,
#     which injects env (DEEPSEEK_*, AUTO_* vars) per ADR-0007. This is the path
#     the scheduler uses.
#
# Pass extra flags with ARGS, e.g.:  make update-event ARGS="--limit 20 --threshold 0.7"

GO   ?= go
BIN  ?= bin/expenses
PKG  := .
AUTO := ../../auto
ARGS ?=

.DEFAULT_GOAL := help

## ---- build -------------------------------------------------------------

.PHONY: build
build: ## compile the app to bin/expenses
	$(GO) build -o $(BIN) $(PKG)

## ---- run (direct: go run .) --------------------------------------------

.PHONY: update-event
update-event: ## match/create events for unassigned transactions.csv rows (needs DEEPSEEK_API_KEY)
	$(GO) run $(PKG) update-event $(ARGS)

.PHONY: dry-run
dry-run: ## preview matches/new events; nothing written to events.json or state.json
	$(GO) run $(PKG) update-event --dry-run $(ARGS)

## ---- run (via auto: env-injected, scheduler path) ----------------------

.PHONY: auto-update-event
auto-update-event: ## run expenses-update-event through the workspace auto CLI
	$(AUTO) run expenses-update-event $(if $(ARGS),-- $(ARGS),)

.PHONY: auto-dry-run
auto-dry-run: ## dry-run expenses-update-event through auto (needs DEEPSEEK_API_KEY in config/expenses)
	$(AUTO) run expenses-update-event -- --dry-run $(ARGS)

## ---- quality -----------------------------------------------------------

.PHONY: test
test: ## run unit tests
	$(GO) test ./...

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## gofmt all sources
	$(GO) fmt ./...

.PHONY: tidy
tidy: ## sync go.mod/go.sum
	$(GO) mod tidy

.PHONY: check
check: fmt vet test ## fmt + vet + test

## ---- housekeeping ------------------------------------------------------

.PHONY: config
config: ## show pack config/secret status (via auto)
	$(AUTO) config expenses

.PHONY: events
events: ## print the current event registry
	@cat config/events.json

.PHONY: clean
clean: ## remove build artifacts
	rm -rf bin/

.PHONY: help
help: ## list targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
