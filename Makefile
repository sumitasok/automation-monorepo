# Makefile — convenience wrapper around the `auto` CLI for this workspace.
#
# `auto` itself needs no build step (framework/tools/auto is a plain Python
# script run directly — edit it and the next `./auto` call picks it up, see
# README). This Makefile exists purely so the common `./auto run <job>` /
# `./auto config <pack>` calls have short, memorable names.
#
# Pass extra job flags with ARGS, e.g.:
#   make wallet-sync ARGS="--since 2026-07-01 --limit 20"
#   make run JOB=gmail-extract ARGS="--backfill"
# Jobs that read a source CSV (wallet-sync, expenses-update-event) accept
# CSV=path to point at a different file. The path is resolved from the PACK's
# own directory (auto run cd's there first), not the workspace root — e.g.
# from packs/wallet/, reach a dated backup in data/gmail/ via ../../data/gmail/:
#   make wallet-sync-dry CSV=../../data/gmail/transactions.20260627.csv
#
# The per-job targets below mirror CATALOG.md at the time of writing. `make
# jobs` always shows the live, authoritative list (run it after `auto new` /
# `auto catalog` add something this file doesn't know about yet) — `make run
# JOB=<id>` works for ANY job, catalogued here or not.

AUTO := ./auto
ARGS ?=
Q    ?=
MSG  ?=
NAME ?=
# Default matches each job's own --csv default (resolved from the pack's
# workdir, e.g. packs/wallet/ -> ../gmail/transactions.csv).
CSV  ?= ../gmail/transactions.csv

.DEFAULT_GOAL := help

## ---- workspace introspection --------------------------------------------

.PHONY: packs
packs: ## list mounted packs
	$(AUTO) packs

.PHONY: jobs
jobs: ## list every job you can see (pack + visibility shown) — live source of truth
	$(AUTO) list

.PHONY: search
search: ## search jobs: make search Q=backup
	$(AUTO) search $(Q)

.PHONY: catalog
catalog: ## regenerate CATALOG.md
	$(AUTO) catalog

.PHONY: doctor
doctor: ## validate manifests + check for visibility leaks
	$(AUTO) doctor

.PHONY: serve
serve: ## start local dashboard: packs, config status, jobs, command help — http://127.0.0.1:$(or $(PORT),4321) (Ctrl+C to stop)
	$(AUTO) serve $(if $(PORT),--port $(PORT),)

.PHONY: schedule-sync
schedule-sync: ## install/refresh OS schedules for enabled jobs
	$(AUTO) schedule sync

.PHONY: schedule-dry
schedule-dry: ## preview what `schedule sync` would install
	$(AUTO) schedule sync --dry-run

.PHONY: log
log: ## append a worklog entry: make log MSG="did a thing"
	$(AUTO) log "$(MSG)"

.PHONY: new
new: ## scaffold a new job into a pack (interactive)
	$(AUTO) new

.PHONY: run
run: ## run any job by id: make run JOB=gmail-extract ARGS="--backfill"
	$(AUTO) run $(JOB) $(if $(ARGS),-- $(ARGS),)

.PHONY: orchestrate
orchestrate: ## run a pipeline from orchestrator/: make orchestrate NAME=gmail-wallet-sync (omit NAME to list)
	$(AUTO) orchestrate $(NAME)

## ---- gmail pack ----------------------------------------------------------

.PHONY: gmail-extract
gmail-extract: ## extract transactions -> data/gmail/transactions.csv
	$(AUTO) run gmail-extract $(if $(ARGS),-- $(ARGS),)

.PHONY: gmail-discover
gmail-discover: ## discover senders -> email_catalog.csv + filters/staging/
	$(AUTO) run gmail-discover $(if $(ARGS),-- $(ARGS),)

.PHONY: gmail-recategorize
gmail-recategorize: ## re-tag rows in email_catalog.csv from categorizer rules
	$(AUTO) run gmail-recategorize $(if $(ARGS),-- $(ARGS),)

.PHONY: gmail-categorize
gmail-categorize: ## AI-assign Category/SubCategory/Labels to transactions.csv (needs DEEPSEEK_API_KEY)
	$(AUTO) run gmail-categorize $(if $(ARGS),-- $(ARGS),)

.PHONY: gmail-categorize-dry
gmail-categorize-dry: ## preview AI categorisation; nothing written
	$(AUTO) run gmail-categorize -- --dry-run $(ARGS)

.PHONY: config-gmail
config-gmail: ## show gmail pack config/secret status
	$(AUTO) config gmail

## ---- wallet pack -----------------------------------------------------------

.PHONY: wallet-sync
wallet-sync: ## REAL RUN: push every not-yet-synced transactions.csv row into Wallet, day-by-day, deduped by MessageID in state.json, tagged with WALLET_LABEL — needs WALLET_API_TOKEN + accounts.json set (config-wallet to check); override source with CSV=path; run wallet-sync-dry first
	$(AUTO) run wallet-sync -- --csv $(CSV) $(ARGS)

.PHONY: wallet-sync-dry
wallet-sync-dry: ## preview what would sync; no token, no API calls; override source with CSV=path
	$(AUTO) run wallet-sync -- --dry-run --csv $(CSV) $(ARGS)

.PHONY: config-wallet
config-wallet: ## show wallet pack config/secret status
	$(AUTO) config wallet

## ---- expenses pack ---------------------------------------------------------

.PHONY: expenses-update-event
expenses-update-event: ## match/create AI events for transactions.csv rows (needs DEEPSEEK_API_KEY)
	$(AUTO) run expenses-update-event $(if $(ARGS),-- $(ARGS),)

.PHONY: expenses-update-event-dry
expenses-update-event-dry: ## preview matches/new events; nothing written
	$(AUTO) run expenses-update-event -- --dry-run $(ARGS)

.PHONY: expense-eventify
expense-eventify: ## send transactions to AI assistant and enrich CSV with event descriptions
	$(AUTO) run expenses-update-event -- --write-csv $(ARGS)

.PHONY: expense-eventify-dry
expense-eventify-dry: ## preview AI event enrichment; nothing written
	$(AUTO) run expenses-update-event -- --write-csv --dry-run $(ARGS)

.PHONY: expense-bulk-assign
expense-bulk-assign: ## import manual event assignments from CSV (MessageID,EventID); override source with ASSIGNMENTS=path
	$(AUTO) run expenses-bulk-assign -- --assignments $(ASSIGNMENTS) $(ARGS)

.PHONY: expense-bulk-assign-dry
expense-bulk-assign-dry: ## preview manual assignments; nothing written; use ASSIGNMENTS=path
	$(AUTO) run expenses-bulk-assign -- --assignments $(ASSIGNMENTS) --dry-run $(ARGS)

.PHONY: expense-fill-similar
expense-fill-similar: ## find unassigned transactions similar to manually-assigned ones via AI (needs DEEPSEEK_API_KEY)
	$(AUTO) run expenses-fill-similar $(if $(ARGS),-- $(ARGS),)

.PHONY: expense-fill-similar-dry
expense-fill-similar-dry: ## preview AI similarity matching; nothing written
	$(AUTO) run expenses-fill-similar -- --dry-run $(ARGS)

.PHONY: config-expenses
config-expenses: ## show expenses pack config/secret status
	$(AUTO) config expenses

## ---- telegram pack ---------------------------------------------------------

.PHONY: telegram-summary
telegram-summary: ## generate the telegram daily digest
	$(AUTO) run telegram-summary $(if $(ARGS),-- $(ARGS),)

.PHONY: config-telegram
config-telegram: ## show telegram pack config/secret status
	$(AUTO) config telegram

## ---- shared pack -----------------------------------------------------------

.PHONY: hello-report
hello-report: ## run the daily hello report demo job
	$(AUTO) run hello-report $(if $(ARGS),-- $(ARGS),)

.PHONY: appdemo
appdemo: ## run the app-backed job demo
	$(AUTO) run appdemo $(if $(ARGS),-- $(ARGS),)

## ---- help --------------------------------------------------------------

.PHONY: help
help: ## list targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}'
