# Feature Specification: Expense Classification Rules Engine

**Feature Branch**: `002-expense-rules-engine`

**Created**: 2026-07-23

**Status**: Draft

**Input**: User description: "i want to creeate a rules engine, like a 'uber drive around afternoon will be office to home travel, categorise this as work expenses', or 'A merhangt name haviung hungerbox is associated with food at work place, categorise it as work expenses', i want this to be the basis of how gmail-categorize and expenses-update-event takes decision about expenses when passed to AI."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Author a merchant-based classification rule (Priority: P1)

Sumit knows that any transaction from "HungerBox" is always workplace food, ordered through his employer's cafeteria vendor. Today, `gmail-categorize` sends every uncategorised transaction to the AI provider, which has to re-guess this every time and occasionally gets it wrong or inconsistent. Sumit wants to write this knowledge down once, as a rule, so that every current and future HungerBox transaction is classified the same, correct way without depending on the AI to infer it fresh each run.

**Why this priority**: This is the simplest, highest-frequency case (recurring merchant) and delivers the core value of the feature on its own: stop paying (in AI cost and inconsistency) for something that is actually a known, fixed fact.

**Independent Test**: Can be fully tested by adding one merchant-name rule, running `gmail-categorize` against a CSV containing an uncategorised HungerBox transaction, and confirming the row is classified consistently with the rule's stated outcome — repeatable across multiple runs without variation.

**Acceptance Scenarios**:

1. **Given** a rule stating transactions from merchant "HungerBox" are workplace food, **When** `gmail-categorize` processes an uncategorised transaction whose merchant matches "HungerBox", **Then** the transaction is classified with the outcome the rule specifies, and the CSV records that the classification came from that rule.
2. **Given** the same rule, **When** `gmail-categorize` is re-run later on a fresh backlog containing another HungerBox transaction, **Then** the new transaction receives the identical classification as the first, with no variation between runs.
3. **Given** no rule matches a transaction's merchant, **When** `gmail-categorize` processes it, **Then** the transaction is classified by the AI provider exactly as it is today (unchanged fallback behavior).

---

### User Story 2 - Author a time-and-pattern based travel rule (Priority: P2)

Sumit takes an Uber most weekday afternoons/evenings from his office back home, and separately sometimes takes Uber for other, non-commute purposes. He wants to describe the recurring pattern once — "Uber rides in the afternoon on a weekday are the office-to-home commute, and count as work travel" — so that matching rides are classified as work expense without the AI having to re-infer intent from sparse data (merchant name and amount alone) every time.

**Why this priority**: This demonstrates the rules engine handles more than a flat merchant lookup — it combines merchant with contextual conditions (time of day, day of week) — which is necessary for the feature to be more than a simple alias list. It builds on Story 1's mechanism.

**Independent Test**: Can be fully tested by adding one rule combining a merchant condition ("Uber") with a time-window condition (weekday afternoon/evening), running `gmail-categorize` against a mix of matching and non-matching Uber transactions, and confirming only the matching ones receive the work-travel classification while others fall through to the AI.

**Acceptance Scenarios**:

1. **Given** a rule for Uber rides on weekday afternoons/evenings classified as work travel, **When** an uncategorised Uber transaction timestamped on a weekday afternoon is processed, **Then** it is classified as the rule's stated work-travel outcome.
2. **Given** the same rule, **When** an uncategorised Uber transaction timestamped on a weekend, or outside the afternoon/evening window, is processed, **Then** the rule does NOT match and the transaction instead goes through the existing AI classification.

---

### User Story 3 - Rules inform the expense-event matcher, not just categorisation (Priority: P2)

Sumit runs `expenses-update-event` to cluster transactions into real-world events (trips, festivals, house moves). Routine, rule-classified transactions — his daily commute, his daily workplace lunch — are not "events"; today the AI sometimes still proposes spurious new events for these recurring routine transactions. Sumit wants the same rules that classify a transaction as routine work expense to also tell the event matcher "this is routine, do not treat it as event-worthy," so `expenses-update-event` stops inventing noise events out of his everyday commute and lunch purchases.

**Why this priority**: Extends the rules engine's value to the second consumer named by the feature request. It depends on rules already existing (Story 1/2) but is independently verifiable against the event job.

**Independent Test**: Can be fully tested by running `expenses-update-event` over a batch containing transactions matched by an existing rule and confirming they are excluded from new-event proposals, while transactions with no matching rule continue to go through the existing AI event-matching logic unchanged.

**Acceptance Scenarios**:

1. **Given** a rule that marks HungerBox transactions as routine (not event-worthy), **When** `expenses-update-event` processes a batch including a HungerBox transaction, **Then** that transaction is not proposed as, or added to, a new event.
2. **Given** a transaction with no matching rule, **When** `expenses-update-event` processes it, **Then** it is evaluated for event matching by the AI exactly as it is today.

---

### User Story 4 - Review why a transaction was classified the way it was (Priority: P3)

Weeks after rules are in use, Sumit looks at a classified transaction and wants to know whether a rule decided it (and which one) or whether the AI made the call, so he can trust or debug the outcome without re-reading code or logs.

**Why this priority**: Not required for the mechanism to function, but necessary for the feature to be trustworthy and maintainable over time — otherwise rules become an opaque black box indistinguishable from AI guesses.

**Independent Test**: Can be fully tested by classifying a batch with a mix of rule-matched and AI-matched transactions, then inspecting the output to confirm each row's decision source (and rule identity, where applicable) is visible without consulting source code.

**Acceptance Scenarios**:

1. **Given** a transaction classified by a specific rule, **When** Sumit inspects that transaction's record, **Then** he can see that a rule (identified by name) produced the classification.
2. **Given** a transaction classified by the AI (no rule matched), **When** Sumit inspects that transaction's record, **Then** he can see that the AI made the classification, not a rule.

---

### Edge Cases

- What happens when two or more rules match the same transaction with different, conflicting outcomes? The engine must resolve deterministically (see FR-005) rather than silently picking one at random or applying both.
- What happens when a rule specifies a category, subcategory, or label that no longer exists in the current taxonomy (e.g., the taxonomy was regenerated and a name changed)? The rule's outcome must be rejected/flagged rather than silently written as an invalid value, consistent with existing taxonomy validation behavior.
- What happens when a rule's merchant text doesn't exactly match the transaction's merchant field (e.g., "Uber" vs. "UBER *TRIP" vs. "Uber India")? Matching must tolerate reasonable real-world variation (case, partial/substring, common suffixes) rather than requiring byte-exact equality.
- What happens when a new rule is added or an existing rule is edited? Already-classified transactions are not silently retroactively changed; the change only affects future runs and any explicit re-categorisation run the user triggers (consistent with today's idempotent, only-touch-uncategorised behavior).
- What happens when a transaction matches a rule but the rule's condition data (e.g., transaction time) is missing or malformed? The rule must not match (fail closed) and the transaction must fall through to the existing AI classification rather than error out the whole run.
- What happens when no rules are defined at all? Behavior for both jobs is identical to today — 100% AI-driven — so adopting the feature is a strictly additive, no-regression change.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST let the user author reusable classification rules, each expressing: one or more matching conditions over a transaction, and an outcome to apply when all of a rule's conditions are met.
- **FR-002**: A rule's conditions MUST support, at minimum: merchant/counterparty name matching (case-insensitive, substring/partial), keyword matching against transaction description/subject text, a time-of-day window, and a day-of-week (or weekday/weekend) constraint. Conditions on a single rule combine with AND logic (all stated conditions must hold for the rule to match).
- **FR-003**: A rule's outcome MUST be expressible as: a category and subcategory, up to the existing label limit, and/or an "event-relevance" signal (routine/not event-worthy) — the same vocabulary already used by `gmail-categorize` and `expenses-update-event` today.
- **FR-004**: The system MUST evaluate every uncategorised transaction against the full set of enabled rules before it is handed to the AI provider, for both `gmail-categorize` and `expenses-update-event`.
- **FR-005**: When exactly one rule matches a transaction, the system MUST apply that rule's outcome directly, without requiring an AI call for that transaction, so a confirmed rule match is both cost-free (no AI usage) and deterministic (identical outcome on every run).
- **FR-006**: When two or more rules match the same transaction, the system MUST resolve the conflict deterministically using an explicit, user-visible precedence (e.g., declared rule order/priority — the first matching rule, in declared order, wins), and MUST NOT silently combine or randomly choose between conflicting outcomes.
- **FR-007**: When zero rules match a transaction, the system MUST fall back to today's AI-only classification path, unchanged.
- **FR-008**: The system MUST validate a rule's outcome against the current taxonomy/label vocabulary (for category/subcategory/labels) at the time the rule is applied, and MUST reject (not silently apply) an outcome referencing a category, subcategory, or label that does not exist, surfacing this to the user.
- **FR-009**: Rules MUST be stored in a human-readable, version-controlled file the user can add to or edit directly, without writing or changing program code.
- **FR-010**: For every classified transaction, the system MUST record whether the classification came from a rule (and which rule, by name) or from the AI provider, so the decision source is auditable after the fact.
- **FR-011**: Adding, editing, or removing a rule MUST NOT alter transactions already classified in a prior run; the effect applies only to transactions processed in subsequent runs (consistent with the existing idempotent, only-touch-uncategorised behavior of `gmail-categorize`), unless the user explicitly triggers a re-categorisation of already-classified rows.
- **FR-012**: The system MUST let a rule declare which decision(s) it applies to — categorisation, event-relevance, or both — so a rule authored for one purpose does not unintentionally affect the other job.
- **FR-013**: The system MUST let the user enable/disable an individual rule without deleting it, so a rule can be temporarily suspended (e.g., while under review) without losing its definition.

### Key Entities *(include if feature involves data)*

- **Rule**: A user-authored, named definition consisting of: an identifier/name, a human-readable description of intent (the "why", e.g. "office-to-home commute"), one or more matching conditions (merchant/counterparty, keyword, time-of-day window, day-of-week), a declared scope (categorisation and/or event-relevance), an outcome (category, subcategory, labels, and/or an event-relevance flag), a precedence/ordering position, and an enabled/disabled state.
- **Transaction** *(existing)*: The bank transaction record already produced by the gmail pack's extraction/categorisation pipeline (merchant, description/subject, amount, timestamp, and its resulting category/subcategory/labels).
- **Classification Decision** *(new, cross-cutting)*: The record of how a specific transaction ended up with its category/subcategory/labels or event-relevance outcome — which source produced it (a specific rule, by name, or the AI provider) — attached to or alongside the existing transaction record so it can be reviewed later.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can add a new rule and, on the very next run of `gmail-categorize` or `expenses-update-event`, see every matching transaction classified according to that rule — with no code change and no waiting on AI provider output for those transactions.
- **SC-002**: Transactions matched by the same rule receive an identical classification 100% of the time across repeated runs (full determinism for rule-covered transactions), compared to the AI-only path where repeat classification of similar transactions is not guaranteed to be identical.
- **SC-003**: For any classified transaction, a user can determine within a few seconds — without reading source code or raw logs — whether a rule or the AI produced the result, and which rule if applicable.
- **SC-004**: The number of transactions sent to the AI provider for classification decreases in proportion to how many transactions match a defined rule (i.e., every rule-covered transaction is one fewer AI request), directly reducing AI usage cost for recurring, known patterns.
- **SC-005**: Introducing the rules engine with zero rules defined produces classification results identical to today's AI-only behavior — confirming the feature is additive and does not regress existing behavior.

## Assumptions

- **Rules take deterministic precedence over the AI.** A confirmed rule match sets the outcome directly and skips the AI call for that transaction entirely, rather than merely being passed to the AI as extra hint context. This follows the user's own standing preference (documented in their global working instructions) to codify recurring, known decisions so they don't repeatedly cost an AI call — a known merchant/pattern should be resolved deterministically, not re-guessed.
- **Conflict resolution uses ordered precedence ("first match wins"), mirroring the existing per-bank filter files** already used elsewhere in the gmail pack (`packs/gmail/filters/*.yaml`), which the user is already familiar with as an ordered-rule pattern.
- **No GPS/route data is available.** Transactions only carry merchant/counterparty name, free-text description/subject, amount, and a timestamp (date and, where the source email/SMS provides it, time). Rules that describe a "direction" (e.g., "office to home") are expressed as time-of-day/day-of-week heuristics against the merchant, not as literal route or location matching, since no location data is captured today.
- **Rule outcomes reuse the existing taxonomy and label vocabulary** (`config/taxonomy.yaml` for gmail-categorize) and the existing event-registry concepts (for expenses-update-event) rather than introducing a new, parallel classification vocabulary (e.g., "work expense" is expressed via an existing category/subcategory/label combination such as Transportation/Business trips + a "Work" label, not a new top-level concept).
- **Rules are authored manually by the user**, not automatically inferred/learned from historical AI decisions or corrections; automatic rule suggestion from past corrections is a plausible future enhancement but out of scope here.
- **Retroactive re-application is opt-in**, reusing the existing recategorisation/bulk-assign mechanisms already present in the codebase (`gmail-recategorize`, `expenses-bulk-assign`) rather than this feature introducing a new automatic retroactive sweep.
- **Rule matching tolerates real-world text variation** (case-insensitivity, substring/partial merchant matches) rather than requiring exact string equality, since bank/merchant text formatting varies across statements.
