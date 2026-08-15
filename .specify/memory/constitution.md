<!--
SYNC IMPACT REPORT
==================
Version change: (unfilled template) → 1.0.0
Bump rationale: MAJOR/initial. The file was still the unmodified Spec Kit
placeholder — no prior governance existed to amend. This is first ratification,
so 1.0.0 rather than a bump from 0.x.

Principles defined (all new — nothing renamed or removed):
  I.   Packs Declare, the Workspace Supplies
  II.  packs/ Is Read-Only
  III. The Workspace Serves, Packs Render
  IV.  Derived Artifacts Regenerate, Never Drift
  V.   Configuration Over Code
  VI.  Boundaries Are Structural, Not Procedural
  VII. Local-First, Least Exposure

Sections added:
  - Workspace–Pack Interface Contract (operationalises I–III)
  - Development Workflow & Quality Gates
  - Governance

Provenance: Principles I, II, IV, V, VI, VII codify decisions already accepted
in docs/adr/ (0001, 0002, 0003, 0005, 0006, 0007, 0012, 0018, 0019). They are
written down here, not invented here. Principle III is NEW governance from the
2026-08-15 amendment request and is not yet implemented — see Unimplemented
Governance below.

Templates requiring updates:
  ✅ .specify/templates/plan-template.md — Constitution Check gates filled
     (was the placeholder "[Gates determined based on constitution file]")
  ✅ .specify/templates/spec-template.md — pack-boundary + UI-declaration
     prompts added to Requirements and Assumptions guidance
  ✅ .specify/templates/tasks-template.md — principle-driven task categories
     added to the Foundational phase guidance
  ✅ .claude/skills/speckit-*/SKILL.md — reviewed; no agent-specific or
     outdated constitution references found, no changes needed

Unimplemented governance (Principle III):
  The workspace does not yet serve pack-generated UI or a landing page.
  `auto serve` (ADR 0012) renders one built-in dashboard and has no concept of
  a pack declaring a UI artefact. Principle III is therefore ASPIRATIONAL until
  the feature exists, and is marked as such in-line. Tracked as a deferred
  intent requiring /speckit-specify — see the constitution amendment summary.
  No ADR covers it yet; one MUST be written when it is built.

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
  persistent data it produces or reads. The sample carries declarations and
  placeholders only — it is committed, so it MUST contain no real values.
- The workspace MUST supply those needs at call time: merge environment
  (pack defaults < `config/<pack>/config.yaml` < ambient shell), symlink each
  declared `files:` entry from `config/<pack>/`, and symlink each declared
  `data_files:` entry from `data/<pack>/`, all into the job's workdir at the
  path the pack expects.
- A pack MUST NOT hardcode an absolute path, resolve a workspace-root-relative
  path, read another pack's directory directly, or otherwise discover its
  environment by inspection. Everything it touches arrives through a declared
  name in its own workdir.
- A pack MUST NOT require modification to be configured. Injection happens
  from the outside at call time, so the same pack serves every user unforked.
- Adding a new secret or data file MUST be a declaration plus a workspace
  value — never a code change in the pack.

*Rationale:* this is what makes a pack shareable at all. A pack that reads
`~/…` or `../../data/` is welded to one machine and one person. ADR 0007
established the mechanism for secrets, ADR 0019 extended it to produced data;
this principle states the rule both are instances of. Cross-pack reads have a
narrow exception — see the Interface Contract.

### II. `packs/` Is Read-Only

`packs/` holds what a pack *is*. It never holds what a pack *has done*.

- A pack directory MUST contain only code, schemas, contracts, sample
  configuration, and documentation.
- A pack directory MUST NOT contain secrets, produced output, caches,
  registries, ledgers, or any machine-specific state — **including the pack's
  own**. There is no carve-out for a pack writing inside itself.
- Every secret MUST live in `config/<pack>/`. Every produced or persisted data
  file MUST live in `data/<pack>/`. Both reach the pack through a symlink the
  workspace creates.
- A job process MUST be prevented from writing outside `config/` and `data/`
  (plus toolchain caches and ephemeral temp), not merely discouraged. Where
  the platform cannot enforce this, the run MUST say so loudly rather than
  claim a guarantee it is not providing.
- `auto doctor` MUST verify that every declared `files:` and `data_files:`
  entry is a symlink into the workspace, never a real file in the pack.

*Rationale:* the credentials-drift bug (ADR 0018) and the pack-resident
registry problem (ADR 0019) were the same failure twice — a pack accumulating
state where its code lives, silently, until something went stale or nearly got
committed. ADR 0018 Amendment 1 tried a per-pack write carve-out and was
reverted by Amendment 2 precisely because "read-only except its own directory"
is not a boundary anyone can reason about. The rule is unconditional so that it
is checkable.

### III. The Workspace Serves, Packs Render

*(Aspirational — governance is binding on new work; the serving mechanism does
not exist yet. See Unimplemented Governance in the sync report.)*

A pack produces UI as artefacts on disk. It does not serve them, and it does
not own how they are reached.

- A pack that has a user interface MUST produce it as one or more static
  artefacts written through its declared `data_files:` into `data/<pack>/`,
  like any other output.
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
  regenerated from `packs.yaml`, manifests, `config/` and Makefiles — never
  hand-edited and never cached into a file that can fall behind.
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
boundary, then visibility checks, then the sandbox.

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
| `packs/<pack>/` | code, schemas, samples, docs | yes, in the pack's own repo |
| `config/<pack>/` | secret values, env overrides | never |
| `data/<pack>/` | data the pack produces or persists, incl. UI artefacts | per-file, declared |
| `docs/adr/` | the decisions behind all of the above | yes |

**What a pack declares** — `config.sample.yaml`: `env:`, `files:`,
`data_files:`. Manifest (`jobs/<id>/manifest.yaml`): `exec`, `workdir`,
`language`, `visibility`, `runs_on`, `schedule`, `runtime.env`, `data.reads`,
`data.writes`, and — for Principle III — its UI artefacts with a title and
path.

**What the workspace guarantees** before a job's process starts: merged
environment exported; every `files:` entry symlinked from `config/<pack>/`;
every `data_files:` entry symlinked from `data/<pack>/`, parent directories
created; the write sandbox active or a loud unconfined warning emitted.

**Declared-name semantics:** the declared name is the path relative to the
pack's own workdir — what the pack opens. The target under `config/<pack>/` or
`data/<pack>/` is flattened to the basename. A pack's internal nesting is its
own business and MUST NOT be mirrored into the workspace directories.

**Cross-pack reads** are permitted and MUST be declared in the manifest's
`data.reads` as a relative path. Exactly one pack MAY write any given file;
every other pack reads it. A shared file MUST have a published, versioned
schema, and its owning pack MUST make writes atomic so a reader never observes
a partial state.

**Prohibited without exception:** a real secret or data file inside
`packs/`; a pack writing outside `config/` and `data/`; a pack binding a port;
a hand-maintained list of anything discoverable from manifests; a second code
path for a second instance of a supported thing.

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

**Version**: 1.0.0 | **Ratified**: 2026-08-15 | **Last Amended**: 2026-08-15
