# Case study: fitting a real application into the architecture

*A teaching walkthrough. It takes one real automation — a Gmail-processing Go
app — and shows exactly how a full, multi-directory application becomes part of
the framework without being torn apart.*

## The starting point

`sa.automation.gmail` is not a script. It's a proper application:

```
sa.automation.gmail/            ← its own git repo (github.com/…/sa.automation.gmail)
├── main.go   go.mod  go.sum    ← a Go program
├── discover/  parser/  auth/  store/  config/  aiassist/   ← packages
├── filters/                    ← YAML config (per-sender rules) + filters/staging/
├── output/                     ← generated per-email markdown  (gitignored)
├── email_catalog.csv           ← produced data                (gitignored)
├── credentials.json token.json ← secrets                       (gitignored)
├── README.md  RUNBOOK.md  CLAUDE.md  specs/  notes/
```

It exposes three runnable things:

- default: extract transactions from matched emails (`--filters-dir ./filters`)
- `discover`: scan the inbox, build a catalog, propose new filters
- `recategorize`: re-run categorization rules over the catalog

## The worry — and the answer

> *"A single application is spread across many directories. Doesn't that break a
> monorepo of one-folder jobs?"*

No — because **the application is a pack, not a job.** "Monorepo" here does not
mean one giant flat tree of tiny scripts. It means **one workspace that composes
many independently-versioned repos (packs) as submodules.** An app keeps its own
repo, its own directory structure, its own build and history. The framework only
asks it to expose two thin things:

1. a `pack.yaml` (identity + default visibility), and
2. one small `manifest.yaml` per runnable subcommand — an **app-backed job**
   that says *how to run* that command, not what the app contains.

So the three subcommands become three jobs: `gmail-extract`, `gmail-discover`,
`gmail-recategorize`. Nothing about the Go code moves or changes.

## How it plugs in (mechanics)

Add two kinds of file to the app repo (additive — no code touched):

`pack.yaml` at the app root:

```yaml
name: gmail
description: Gmail → transactions/catalog automation (Go app).
default_visibility: private     # this instance holds personal filters/data
maintainers: [sumit]
```

A manifest per subcommand, e.g. `jobs/gmail-extract/manifest.yaml`:

```yaml
id: gmail-extract
name: Gmail transaction extract
description: Extract transactions from matched emails into transactions.csv.
category: gmail
language: go
exec: "go run . --filters-dir ./filters"   # app-backed: a command, not a file
workdir: "."                               # run in the pack (app) root
visibility: private
runs_on: { os: [macos, linux], machines: [home-server] }
schedule: { cron: "0 7 * * *", timezone: Asia/Kolkata, enabled: false }
runtime: { timeout_seconds: 900, env: [ANTHROPIC_API_KEY] }
data: { reads: [filters/], writes: [transactions.csv] }
```

Then mount the app as a pack in the workspace and register it:

```bash
# in your workspace
git submodule add git@github.com:sumitasok/sa.automation.gmail.git packs/gmail
```

```yaml
# packs.yaml
  - name: gmail
    path: packs/gmail
    source: git@github.com:sumitasok/sa.automation.gmail.git
    default_visibility: private
    writable: true
```

Now it's a first-class part of the system:

```bash
./auto list                 # gmail-extract / discover / recategorize appear
./auto run gmail-discover   # runs `go run . discover` in the app root, logged + timed
./auto schedule sync        # installs gmail-extract's cron on home-server only
./auto catalog              # the app's subcommands show up in the index
```

The run wrapper gives a compiled Go app the same logging, timeout, and
run-history as a one-line bash script — for free.

## Two lessons this example teaches

**1. Code is shareable; inputs and outputs are not.** An app cleanly separates
the *program* (often fine to share or open-source) from the *private inputs*
(credentials, personal filters) and *produced data* (the transaction CSVs). The
pack/repo can be `shared` or `public` for the code, while each user supplies
their own secrets and config locally and keeps produced data out of git. This app
already does the hard part: `credentials.json`, `token.json`, `output/`,
`*.state`, and `email_catalog.csv` are all `.gitignore`d. Good hygiene.

**2. Spot the leak (ADR 0005 in the wild).** One file breaks the rule:
`transactions.csv` — personal financial data a job *produced* — is committed to
git. Per ADR 0005, produced data should be machine-local, not versioned.
The fix:

```bash
cd packs/gmail
git rm --cached transactions.csv transactions.20260627.csv
echo -e "transactions*.csv" >> .gitignore
git commit -m "stop versioning produced transaction data (ADR 0005)"
```

If you ever need those transactions on another machine, sync a diffable export or
move them to a hosted DB — don't carry the produced CSV in git.

## The takeaway for anyone adopting the framework

- Small script? → a **script job** (one folder, one entrypoint).
- Whole application? → a **pack** (its own repo, its own shape), with an
  **app-backed job** per subcommand (`exec` + `workdir`).
- Either way it lands in the same catalog, the same scheduler, the same run
  wrapper, and obeys the same visibility and data rules.

Nothing about "this is a big app across many directories" is a problem. That's
what packs are for.
