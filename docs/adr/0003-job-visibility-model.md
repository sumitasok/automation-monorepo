# ADR 0003 — Job visibility model (private / shared / public)

**Status:** accepted — 2026-07-04

## Context

Within the pack architecture (ADR 0002) we need a per-job signal of how far a job
may travel, and an enforced rule that prevents private jobs from leaking.

## Decision

Every job has a `visibility`: one of `private`, `shared`, `public`. If omitted,
it inherits the pack's `default_visibility` (from `pack.yaml`). Meanings:

- **private** — never leaves the owner's workspace. Only valid inside a pack
  whose `default_visibility` is `private`.
- **shared** — travels to everyone with access to the shared pack's repo.
- **public** — safe to publish openly (e.g. a community pack).

**Enforced invariant:** a `private` job may not live in a non-private pack.
`auto doctor` fails on violation, and this check runs in CI, so a private job
can never be committed into the shared pack. The repo boundary (ADR 0002) is the
primary guard; this is the second.

## Alternatives considered

- *Visibility as folder convention only* (e.g. `jobs/private/…`): rejected —
  easy to misfile, no enforcement.
- *No per-job field, rely purely on which repo the job sits in*: rejected — the
  explicit field enables `auto share`, `auto list --visibility`, and a
  machine-checkable leak test.

## Consequences

- `auto list --visibility`, `auto share`, and the shareable per-pack catalog are
  all driven by this field.
- Promoting a private job to shared is a deliberate two-step act: move its folder
  into the shared pack **and** change `visibility` — doctor blocks a half-done
  move.
