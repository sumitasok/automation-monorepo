# Makefile for the wallet automation pack (Go app, pure stdlib).
#
# Two ways to run:
#   * Direct  (make sync / dry-run) -> plain `go run . sync`, fast dev loop.
#     Reads WALLET_* env from your shell and ./accounts.json.
#   * Via auto (make auto-sync) -> workspace `./auto run wallet-sync`, which
#     injects env (WALLET_*, AUTO_* vars) + config/wallet/accounts.json per
#     ADR-0007. This is the path the scheduler uses.
#
# Pass extra flags with ARGS, e.g.:  make sync ARGS="--since 2026-07-01 --limit 20"
# Point at a different source file with CSV, e.g.:  make sync CSV=../gmail/transactions.20260627.csv

GO   ?= go
BIN  ?= bin/wallet
PKG  := .
AUTO := ../../auto
CSV  ?= ../gmail/transactions.csv
ARGS ?=

.DEFAULT_GOAL := help

## ---- build -------------------------------------------------------------

.PHONY: build
build: ## compile the app to bin/wallet
	$(GO) build -o $(BIN) $(PKG)

## ---- run (direct: go run .) --------------------------------------------

.PHONY: sync
sync: ## sync transactions.csv -> Wallet (needs WALLET_API_TOKEN + accounts.json); override source with CSV=path
	$(GO) run $(PKG) sync --csv $(CSV) $(ARGS)

.PHONY: dry-run
dry-run: ## parse + map + report only; no API calls, no token needed; override source with CSV=path
	$(GO) run $(PKG) sync --dry-run --csv $(CSV) $(ARGS)

## ---- run (via auto: env-injected, scheduler path) ----------------------

.PHONY: auto-sync
auto-sync: ## run wallet-sync through the workspace auto CLI; override source with CSV=path
	$(AUTO) run wallet-sync -- --csv $(CSV) $(ARGS)

.PHONY: auto-dry-run
auto-dry-run: ## dry-run wallet-sync through auto (no token needed; accounts.json still required); override source with CSV=path
	$(AUTO) run wallet-sync -- --dry-run --csv $(CSV) $(ARGS)

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

.PHONY: check
check: fmt vet test ## fmt + vet + test

## ---- housekeeping ------------------------------------------------------

.PHONY: config
config: ## show pack config/secret status (via auto)
	$(AUTO) config wallet

.PHONY: clean
clean: ## remove build artifacts
	rm -rf bin/

.PHONY: help
help: ## list targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
