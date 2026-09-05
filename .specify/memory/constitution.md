<!--
SYNC IMPACT REPORT
==================
Version change: 1.0.0 → 2.0.0

Bump rationale: MAJOR. Principles I and II are redefined at the directory-
location level: the Single Source of Truth for pack config, produced data,
and learned rules moves from in-repo `config/<pack>/` + `data/<pack>/`
(git-ignored, ADR 0007 / 0019) to a fully external `${CONFIG_PATH}`
(default `~/automation-monorepo-config/`), addressed by `<domain>/<source>`
for any pack that is a domain (per spec 008 / ARCHITECTURE.md, verified
against the actual `ConfigLoader`/`RulesLoader` implementations in
`packs/shared/lib/`), falling back to flat `<pack>` addressing for packs
that are not domains. Every currently-compliant pack (`gmail`, `wallet`,
`telegram`, `expenses` per `packs.yaml`) stores its config/data in-repo
today and becomes NON-COMPLIANT the moment this lands — that is the MAJOR
trigger by the constitution's own versioning policy, even though no
principle is removed or renamed. (The proposing request suggested MINOR;
the policy is followed here instead, since redefinition-with-breakage is
explicitly the MAJOR criterion, not the MINOR one.)

Principles modified:
  I.   Packs Declare, the Workspace Supplies — config/data/rules root moved
       external; addressing becomes `<domain>/<source>` for domain packs
       (flat `<pack>` otherwise); `rules:` added as a third declarable
       category alongside `files:` / `data_files:`.
  II.  packs/ Is Read-Only — updated to point at `${CONFIG_PATH}/...`;
       clarified that the *workspace repository itself* (not only packs/)
       must not hold runtime config, data, or rules — the previous in-repo
       `config/` and `data/` directories are deprecated by this amendment.

Sections added:
  - "Configuration Root & Addressing" note under Principle I (defines
    `CONFIG_PATH`, its default, domain-vs-flat addressing, and that it is
    never versioned in this repo).
  - "Migration (v2.0.0)" under the Interface Contract, naming the packs
    affected and the ADR still required to record this decision.

Sections removed: none.

Interface Contract table: `config/<pack>/` and `data/<pack>/` rows repointed
to `${CONFIG_PATH}/...` with domain/source addressing; `rules/<domain>/
<source>/` row added.

Templates requiring updates:
  ✅ .specify/templates/plan-template.md — Constitution Check gates I/II
     re-worded to the external, domain/source-addressed path
  ✅ .specify/templates/spec-template.md — constitution prompts re-worded
  ✅ .specify/templates/tasks-template.md — foundational-phase bullets
     re-worded
  ✅ .claude/skills/speckit-*/ — checked, no hardcoded `config/<pack>` or
     `data/<pack>` path references found, no changes needed
  ⚠ README.md — still documents the in-repo, flat-pack model in several
     places (`auto config init`, ADR 0007/0019 citations); needs a fuller
     pass tied to the `auto` CLI's actual migration, tracked as a Next
     Action
  ✅ CLAUDE.md (project) — already states the external-config model;
     no change needed, this amendment brings governance into line with it
  ✅ FRAMEWORK_RULES.md — already CONFIG_PATH-based; no change needed

Follow-up TODOs (deferred, not performed by this amendment):
  - An ADR is required per the Development Workflow gate before this
    decision is relied upon. None exists yet; the next free number is
    0023 (last is 0022-wallet-fetch-incremental.md). Recommend
    "0023-external-config-root.md", superseding/amending ADR 0007 and
    ADR 0019's location choice (their symlink *mechanism* is kept).
  - `specs/008-restructure-architecture/` is referenced by
    `.specify/spec-map.json` and by the phase notes as this effort's
    feature directory but does not exist on disk — the spec lifecycle
    (Development Workflow) was bypassed in favour of `notes/` +
    `ARCHITECTURE.md` + `PHASE5-*.md`. Flagged, not fixed here.
  - README.md's config/data narrative needs rewriting once the `auto`
    CLI itself is updated to inject from `${CONFIG_PATH}` with
    domain/source addressing — implementation work, out of scope for
    this governance-only amendment.
  - Migrating the four existing flat packs' in-repo config/data to
    `~/automation-monorepo-config/` (as flat `<pack>` addressing, since
    they are not domains) is implementation work, out of scope here —
    tracked as a Next Action.

Deferred placeholders: none. No bracketed tokens remain.
-->

# Automation Workspace Constitution

The workspace is a host. Packs are guests. Almost everything in this document
follows from taking that seriously: the host owns the machine — paths,
secrets, data, ports, scheduling, what gets served — and the guest owns a
capability and nothing else. A pack that reaches around the host to touch the
machine directly is the defect this constitution exists to prevent, because
every drift, leak and stale-state bug in this repo's history has had that
shape.

## Core Principles

### I. Packs Declare, the Workspace Supplies

A pack states what it needs. The workspace decides where that comes from and
puts it in place before the pack runs. A pack never resolves this for itself.

- A pack MUST declare its needs in `config.sample.yaml` at its root: `env:`
  for environment variables, `files:` for secret files, `data_files:` for
  persistent data it produces or reads, and `rules_files:` for learned or
  AI-generated rules it reads or updates. The sample carries declarations and
  placeholders only — it is committed, so it MUST contain no real values.
- A pack that groups multiple data sources under one problem space (e.g.
  `expense-domain` grouping `gmail`, `wallet`, `telegram`) MUST declare
  itself a **domain** in its manifest and MUST declare each **source** by
  name. A domain's own aggregation logic (the "engine") is addressed as if
  it were a source named `engine`. A pack that is not a domain is addressed
  flat, by its own pack name.
- The workspace MUST supply those needs at call time, merging environment
  (pack defaults < the relevant `config.yaml` below < ambient shell) and
  symlinking each declared entry into the job's workdir at the path the pack
  expects, from:
  - a **domain pack**: `${CONFIG_PATH}/config/<domain>/domain.yaml` for
    domain-level config, `${CONFIG_PATH}/config/<domain>/<source>.yaml` for
    each source's `files:`, `${CONFIG_PATH}/data/<domain>/<source>/` for
    each source's `data_files:` (`<source>` = `engine` for the domain's own
    outputs), and `${CONFIG_PATH}/rules/<domain>/<source>/` for each
    source's `rules_files:`;
  - a **flat pack**: `${CONFIG_PATH}/config/<pack>/` for `files:`,
    `${CONFIG_PATH}/data/<pack>/` for `data_files:`, and
    `${CONFIG_PATH}/rules/<pack>/` for `rules_files:`.
- A pack MUST NOT hardcode an absolute path, resolve a workspace-root-relative
  path, read another pack's or another source's directory directly, resolve
  `~` itself, or otherwise discover its environment by inspection. Everything
  it touches arrives through a declared name in its own workdir — including
  the location of `${CONFIG_PATH}` itself, and its own domain/source or pack
  name, which a pack never computes.
- A pack MUST NOT require modification to be configured. Injection happens
  from the outside at call time, so the same pack serves every user unforked.
- Adding a new secret, data file, or rule file — or a new source within an
  existing domain — MUST be a declaration plus a workspace value, never a
  code change in the pack or the domain engine.

**Configuration Root & Addressing.** `${CONFIG_PATH}` names one external
directory — default `~/automation-monorepo-config/`, overridable via the
`CONFIG_PATH` environment variable or an equivalent `--config-path` startup
parameter — holding three subtrees: `config/`, `data/`, `rules/`. Within
each subtree, a domain pack is addressed `<domain>/<source>` (source config
is a `.yaml` file; source data and rules are directories) and a flat pack is
addressed `<pack>` directly — see the Interface Contract for the concrete
layout. This directory, in full, is the Single Source of Truth for
everything a pack needs to run and everything a pack produces. It MUST NOT
be versioned in this repository, MUST NOT be nested inside this repository's
working tree, and its contents MUST NOT be committed here even accidentally
(see Principle II).

*Rationale:* this is what makes a pack shareable at all. A pack that reads
`~/…` or `../../data/` is welded to one machine and one person. ADR 0007
established the mechanism for secrets and ADR 0019 extended it to produced
data; this principle states the rule both are instances of. The external
root additionally keeps the workspace repository itself free of
machine-local and personal state, so the repository can be cloned, forked,
or inspected without ever touching real credentials or financial data.
Cross-pack reads have a narrow exception — see the Interface Contract.

### II. `packs/` Is Read-Only

`packs/` holds what a pack *is*. It never holds what a pack *has done*. The
workspace repository as a whole is subject to the same rule: this repo holds
code, contracts, and documentation, never runtime state.

- A pack directory MUST contain only code, schemas, contracts, sample
  configuration, and documentation.
- A pack directory MUST NOT contain secrets, produced output, caches,
  registries, ledgers, learned rules, or any machine-specific state —
  **including the pack's own**. There is no carve-out for a pack writing
  inside itself.
- Every secret MUST live under `${CONFIG_PATH}/config/` — at `<domain>/
  <source>.yaml` for a domain pack's source, `<domain>/domain.yaml` for a
  domain pack itself, or `<pack>/` for a flat pack. Every produced or
  persisted data file MUST live under `${CONFIG_PATH}/data/` at the matching
  `<domain>/<source>/` or `<pack>/` location. Every learned or AI-generated
  rule MUST live under `${CONFIG_PATH}/rules/` at the matching `<domain>/
  <source>/` or `<pack>/` location. All three reach the pack through a
  symlink the workspace creates into the job's workdir, never a real file in
  `packs/` and never a file committed to this repository.
- A job process MUST be prevented from writing outside `${CONFIG_PATH}` (plus
  toolchain caches and ephemeral temp), not merely discouraged. Where the
  platform cannot enforce this, the run MUST say so loudly rather than claim
  a guarantee it is not providing.
- `auto doctor` MUST verify that every declared `files:`, `data_files:`, and
  `rules_files:` entry is a symlink into `${CONFIG_PATH}` at the correct
  domain/source or flat-pack location, never a real file in the pack or a
  real file inside this repository.

*Rationale:* the credentials-drift bug (ADR 0018) and the pack-resident
registry problem (ADR 0019) were the same failure twice — a pack accumulating
state where its code lives, silently, until something went stale or nearly got
committed. ADR 0018 Amendment 1 tried a per-pack write carve-out and was
reverted by Amendment 2 precisely because "read-only except its own directory"
is not a boundary anyone can reason about. This amendment (v2.0.0) applies the
same reasoning one level up: an in-repo-but-git-ignored `config/` or `data/`
directory is the identical shape of risk — one accidental `.gitignore` edit
or `git add -f` away from leaking secrets or personal financial data into
history. Moving the root fully outside the repository removes that failure
mode structurally rather than trusting an ignore rule to hold. The rule is
unconditional so that it is checkable.

### III. The Workspace Serves, Packs Render

*(Aspirational — governance is binding on new work; the serving mechanism does
not exist yet. See Unimplemented Governance in the v1.0.0 sync report.)*

A pack produces UI as artefacts on disk. It does not serve them, and it does
not own how they are reached.

- A pack that has a user interface MUST produce it as one or more static
  artefacts written through its declared `data_files:` into
  `${CONFIG_PATH}/data/<domain>/<source>/` (or `${CONFIG_PATH}/data/<pack>/`
  for a flat pack), like any other output.
- A pack MUST NOT run a server, bind a port, register a route, or depend on
  the URL it will be served from. A pack-generated page MUST render correctly
  when opened directly from disk, with no server involved.
- The workspace MUST be the only thing that serves. It serves each pack's UI
  artefacts and MUST provide a landing page enumerating every UI every mounted
  pack produces, so there is one place to find all of them.
- The landing page MUST be discovered from pack declarations, never
  hand-maintained: a pack that declares a UI appears without the workspace
  being edited, and a pack that is unmounted disappears.
- A pack MUST declare its UI artefacts — at minimum a human-readable title and
  the artefact path — in its manifest, so the workspace can enumerate them
  without executing the pack or parsing its output.
- Serving MUST inherit Principle VII: bound to localhost, no authentication,
  and no wider exposure without a decision that adds authentication first.

*Rationale:* several packs now generate pages, and without this each one
invents its own answer to "where does it live and how do I open it" — the same
sprawl `auto serve` (ADR 0012) was created to collapse for jobs and config.
Keeping serving in the host also keeps packs portable: an artefact-producing
pack works when mounted anywhere, whereas a port-binding one carries a
deployment assumption with it.

### IV. Derived Artifacts Regenerate, Never Drift

Anything computable from source of truth MUST be computed, on demand, from
that source.

- Catalogs, schedules, dashboards, landing pages and job listings MUST be
  regenerated from `packs.yaml`, manifests, `${CONFIG_PATH}/config/` and
  Makefiles — never hand-edited and never cached into a file that can fall
  behind.
- Two views of the same fact MUST call the same loader. A second
  implementation that reads the same source separately is a defect, not an
  optimisation.
- Registration steps are forbidden where discovery is possible. Mounting a
  pack MUST be sufficient for its jobs, config and UI to appear.

*Rationale:* ADR 0001 and ADR 0012 both chose regeneration over storage, and
the reason holds generally — a derived file that can drift eventually will,
and the drift is silent by construction.

### V. Configuration Over Code

Knowledge that varies by institution, instrument, jurisdiction, format or
account belongs in data, not in a branch.

- Support for a new instance of a thing the system already handles — another
  broker, another data source, another category, another rule — MUST be
  addable as data. It MUST NOT require a new code path.
- There MUST be exactly one implementation of any shared computation.
  Duplicating a program to serve a second instance is prohibited; if two
  instances cannot share a description, the description is wrong and MUST be
  generalised.
- Where variants are described by data, there MUST be exactly one contract all
  variants are described under.
- Input that matches no configured rule MUST be surfaced, never silently
  dropped. Silence turns a missing rule into invisible data loss.

*Rationale:* the manifest-driven design (ADR 0001) is this principle applied to
jobs. Where it has not been applied, the cost is visible: a second broker was
added to the finances analysis by copying the program, producing two divergent
copies of the same tax arithmetic. The rule exists to make that non-viable.

### VI. Boundaries Are Structural, Not Procedural

A boundary that depends on remembering is not a boundary.

- Privacy boundaries MUST be enforced by repository access control, not by
  ignore rules or export tooling.
- Every job MUST carry a visibility (`private` | `shared` | `public`),
  defaulting to its pack's; `auto doctor` MUST fail when a private job appears
  in a shareable pack.
- Runtime containment MUST be enforced by the sandbox, and contract compliance
  by `auto doctor`. Documentation alone is never the enforcement mechanism.
- Every enforcement mechanism MUST be self-testable on a machine before it is
  relied upon there, and MUST report honestly when it is not in effect.

*Rationale:* ADR 0002 chose separate repos over one-repo-with-flags for exactly
this reason — with flags, one bad `.gitignore` line leaks private automations
and there is no second line of defence. Defence in depth throughout: the repo
boundary, then visibility checks, then the sandbox. This amendment (v2.0.0)
extends the same defence-in-depth logic to config/data location: an ignore
rule was the second line of defence for secrets before v2.0.0; an external
root removes the need to rely on that line at all.

### VII. Local-First, Least Exposure

This workspace holds personal financial data, live account sessions and OAuth
credentials. The default is that none of it leaves the machine.

- Services MUST bind to localhost. Widening exposure requires adding
  authentication first, as its own decision, never as a default or a
  convenience flag.
- Secret *values* MUST never be rendered in any output. Set-versus-missing
  status is the most any introspection surface may show.
- Anything leaving the machine MUST be an explicit, configured act — never a
  default, a fallback, or a side effect of another operation.
- Where output is shared, what it may reveal MUST be governed by data, and
  withholding MUST remove the value from the artefact rather than hide it in
  the presentation.

*Rationale:* ADR 0012 scoped the dashboard to localhost because it surfaces
which secrets exist and the full job layout. The same reasoning governs
everything else this workspace can emit.

## Workspace–Pack Interface Contract

This section makes Principles I–III concrete. It is the checkable surface;
where it and prose disagree, this section governs.

**Directory meanings — no file may contradict these:**

| Path | Holds | Versioned |
|---|---|---|
| `packs/<pack>/` | code, schemas, samples, docs; a domain pack's sources live at `packs/<domain>/sources/<source>/` | yes, in the pack's own repo |
| `${CONFIG_PATH}/config/<domain>/domain.yaml` or `${CONFIG_PATH}/config/<pack>/` | domain-level secret values / env overrides, or a flat pack's | never in this repo (external directory; a personal dotfiles/config repo of the user's own is out of scope of this constitution) |
| `${CONFIG_PATH}/config/<domain>/<source>.yaml` | one source's secret values / env overrides within a domain | never in this repo |
| `${CONFIG_PATH}/data/<domain>/<source>/` or `${CONFIG_PATH}/data/<pack>/` | data a source (or `engine` for the domain itself) produces or persists, incl. UI artefacts; or a flat pack's | never in this repo |
| `${CONFIG_PATH}/rules/<domain>/<source>/` or `${CONFIG_PATH}/rules/<pack>/` | learned or AI-generated rules (YAML) for a source (or `engine`), applied without code change; or a flat pack's | never in this repo; the generating pack is authoritative for its own rules files |
| `docs/adr/` | the decisions behind all of the above | yes |

**What a pack declares** — `config.sample.yaml`: `env:`, `files:`,
`data_files:`, `rules_files:`, plus, for a domain pack, the list of
`sources:` it groups. Manifest (`jobs/<id>/manifest.yaml`): `exec`,
`workdir`, `language`, `visibility`, `runs_on`, `schedule`, `runtime.env`,
`data.reads`, `data.writes`, and — for Principle III — its UI artefacts with
a title and path.

**What the workspace guarantees** before a job's process starts: merged
environment exported; every `files:` entry symlinked from the config path
above (domain/source or flat, as declared); every `data_files:` entry
symlinked from the matching `data/` path; every `rules_files:` entry
symlinked from the matching `rules/` path; parent directories created; the
write sandbox active or a loud unconfined warning emitted.

**Declared-name semantics:** the declared name is the path relative to the
pack's own workdir — what the pack opens. The target under `${CONFIG_PATH}`
is flattened to the basename within its `<domain>/<source>/` or `<pack>/`
directory. A pack's internal nesting is its own business and MUST NOT be
mirrored into the workspace directories.

**Cross-pack and cross-source reads** are permitted and MUST be declared in
the manifest's `data.reads` as a path relative to `${CONFIG_PATH}/data/`
(e.g. `expense-domain/gmail/transactions.csv`). Exactly one source or pack
MAY write any given file; every other reader consumes it. A shared file MUST
have a published, versioned schema, and its owning source/pack MUST make
writes atomic so a reader never observes a partial state.

**Prohibited without exception:** a real secret, data, or rules file inside
`packs/` or inside this repository anywhere; a pack or source writing outside
`${CONFIG_PATH}`; a pack binding a port; a hand-maintained list of anything
discoverable from manifests; a second code path for a second instance of a
supported thing; a pack or the workspace hardcoding the literal path
`~/automation-monorepo-config` instead of reading `${CONFIG_PATH}`; a
domain's source reaching into a sibling source's `<domain>/<other-source>/`
subtree instead of going through the domain engine or a declared
`data.reads`.

**Migration (v2.0.0).** Every pack currently registered in `packs.yaml` that
predates this amendment — `gmail`, `wallet`, `telegram`, `expenses` — is a
flat pack that stores its config and data in-repo (`config/<pack>/`,
`data/<pack>/`, git-ignored) and is NON-COMPLIANT with this contract until
migrated to `${CONFIG_PATH}/config/<pack>/` and `${CONFIG_PATH}/data/<pack>/`
(flat addressing — these are not domains). The restructured `expense-domain`
pack already declares itself a domain with `gmail`/`wallet`/`telegram`/
`bank-csv` sources and an `engine`, targeting the external, domain/source-
addressed root; it is compliant by design once its manifest paths are
verified against this section. An ADR recording this decision (superseding
the location choice in ADR 0007 and ADR 0019, whose symlink *mechanism* this
amendment keeps) MUST be written before the migration is relied upon — see
Development Workflow.

## Development Workflow & Quality Gates

**Decisions are recorded.** Any change to the workspace–pack contract, a
directory's meaning, an enforcement mechanism, or a cross-pack dependency MUST
have an ADR in `docs/adr/` before it is relied upon. ADRs are append-only:
superseding a decision means a new ADR or a dated amendment, never a silent
edit. When an ADR and this constitution conflict, the more recent governs and
the older MUST be amended to say so.

**Features follow the spec lifecycle.** `/speckit-specify` → `/speckit-plan` →
`/speckit-tasks` → `/speckit-implement`, each on a feature branch in its own
worktree, tracked in `.claude/spec-map.json`, with a merge request raised and
the worktree retained until it merges.

**Gates.** `/speckit-plan` MUST evaluate its Constitution Check against these
principles and MUST record any violation in Complexity Tracking with the
simpler alternative that was rejected and why. An unjustified violation blocks
the plan. `auto doctor` MUST pass before a change to pack layout, config or
data declarations is considered done. `auto sandbox-check` MUST pass on any
machine before that machine's containment guarantee is relied upon.

**Verification is honest.** A guarantee that could not be exercised on the
target platform MUST be reported as unverified rather than assumed. Untested
carve-outs MUST be labelled as such.

## Governance

This constitution supersedes ad-hoc practice. Where it and habit disagree, it
governs, and the habit is a defect to be fixed or an amendment to be proposed.

**Amendment.** Proposed via `/speckit-constitution`, with the affected
principles, the rationale, and a version bump. An amendment that changes what a
pack must do MUST state the migration for packs that already exist. Amendments
land through the same branch-and-merge-request flow as any feature.

**Versioning** is semantic:
- **MAJOR** — a principle is removed or redefined such that compliant work
  becomes non-compliant.
- **MINOR** — a principle or section is added, or guidance is materially
  expanded.
- **PATCH** — clarification, wording, or non-semantic refinement.

**Compliance review.** Every plan runs the Constitution Check. Every ADR states
which principles it implements or amends. When a principle is marked
aspirational, work that would depend on it MUST either implement it or record
the gap — it MUST NOT quietly proceed as if the mechanism existed.

**Precedence** on conflict: this constitution, then ADRs in `docs/adr/` newest
first, then the templates in `.specify/templates/`, then `README.md` and
`RUNBOOK.md`. Runtime guidance lives in `README.md` (usage) and `RUNBOOK.md`
(what was done and why); neither may contradict this document.

**Version**: 2.0.0 | **Ratified**: 2026-08-15 | **Last Amended**: 2026-09-05
