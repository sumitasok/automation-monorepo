# Feature Specification: `portfolio` pack — broker- and instrument-agnostic lot register and sell planner

**Feature Branch**: `feature/portfolio-tax-pack`

**Created**: 2026-08-15

**Last Updated**: 2026-08-15 (clarifications resolved — two-pack split, sharing model, retirement schedule)

**Status**: Ready for planning

**Input**: User description: "check the project ~/Claude/Projects/sa.finances and see how you can make the code in there into a pack in /Users/sumitasok/Claude/Projects/automation-monorepo/packs. Imagine this as a general purpose tool. source of data is right now schwab, but tomorrow it should be able to serve for IBKr and many more. data format should be generic (use current as generic) tomorrow this tool should be extendable for any ticker (right now it is AVGO). make the HTML such that it gets rendered from json which it either packs, or it should be possible to fetch the json from a location. Know that the pack repo should be only having data and json schemas. any data should go into data/<pack-name>. before coding lets start planning and specing."

**Clarifications resolved**: tax planning and tax calculation are two separate packs, delivered in that order (Q1); page sharing is local-first with hosted sharing as a deliberate, redacted opt-in (Q2); the vault-resident program is retained read-only for one filing cycle as a cross-check, then deleted (Q3).

## Overview

A body of working analysis currently lives outside the automation workspace, in
the `sa.finances` Obsidian vault (`_db/tax/`). It maintains a register of share
lots, replays broker exports against it to work out which shares were sold, and
answers the question that actually matters to the owner: *for each batch of
shares I hold, is selling it now expensive or cheap, and how far can the price
fall before waiting stops paying?* It publishes that as an interactive page.

The analysis is sound and hard-won, but it is welded to three accidents of its
origin: one broker (Schwab), one holding (AVGO), and one vault directory
layout. A second broker was added by **copying the program** rather than adding
a description of that broker, which is the clearest possible signal that the
generic seams are not yet real.

### Two packs, one register

The vault code is really two programs that happen to share a lot register, and
they answer different questions at different times:

| | **Planning** — before a sale | **Calculation** — after actual sales |
|---|---|---|
| Question | What should I sell, when, and what does waiting save me? | What do I owe and what do I file for this year? |
| Horizon | Forward | Backward |
| Driven by | Today's price, holding-period maturity, break-evens | The fiscal year's realised events |
| Vault origin | `planner.py`, `refresh_explorer.py`, `explorer.html` | `engine.py`, `report_md.py`, `loaders.py` |

These become **two separate packs**, delivered in sequence:

1. **`portfolio` — this feature.** Owns broker import, the lot register, the
   sell planner and the interactive page. Answers the planning question.
2. **`tax` — a separate, later feature.** Reads the register produced by
   `portfolio`, adds the fiscal year's own facts, and produces the tax return
   computation. Answers the calculation question. Specified separately.

The **lot register is the contract between them**, which is why its published
schema is a requirement of this feature rather than of the later one — the
second pack cannot be specified until the first has fixed the shape it reads.
The workspace already has this exact pattern: the `gmail` pack owns
`transactions.csv` and both `wallet` and `expenses` read it without writing it.

**This is a relocation-and-generalisation feature, not a new analysis.** The
arithmetic, the tax conventions and the interpretation the existing work
encodes are the requirement, not a starting point to be re-derived. Any change
in a published figure is a defect unless it is explicitly a correction.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run today's planning analysis from the workspace, with no data inside the pack (Priority: P1)

The owner runs the holdings analysis through the workspace's normal job
runner instead of by hand in a vault subdirectory. Every figure it produces
matches what the vault produced. The pack directory contains only behaviour,
contracts and samples; every lot, rate and generated page lives under the
workspace data directory for this pack.

**Why this priority**: Nothing else in this feature is worth anything if the
capability does not survive the move intact. This story alone replaces the
current manual workflow, and it establishes the pack/data boundary that every
later story — and the whole second pack — depends on.

**Independent Test**: Migrate the register, run the analysis, and compare the
resulting register and page figures against what the vault produces from the
same inputs. Separately, confirm no file the run reads or writes is a real file
inside the pack directory.

**Acceptance Scenarios**:

1. **Given** the current lot register and broker export, **When** the owner runs the analysis job from the workspace, **Then** the resulting register and every figure on the generated page equal what the vault program produces from the same inputs, exactly.
2. **Given** the pack is installed, **When** the workspace's configuration-drift check runs, **Then** it reports that every file the pack persists resolves to the pack's workspace data directory, and no real data file sits inside the pack directory.
3. **Given** a fresh checkout of the pack with no data present, **When** the owner runs the analysis job, **Then** it fails with a message naming exactly which data files are missing and where they are expected, rather than producing a partial or empty result.
4. **Given** a run completes, **When** the owner inspects what changed on disk, **Then** only files under the pack's workspace data directory were written.

---

### User Story 2 - Track a second instrument without touching behaviour (Priority: P2)

The owner holds something besides the one instrument the analysis was built
around. They add it by describing it as data — a position with its lots — and
the register, the planner and the page all handle it, including viewing one
instrument at a time or the whole portfolio.

**Why this priority**: The owner's actual portfolio already extends past the
single instrument, so this is the first real limitation encountered in daily
use. It is also the cheaper of the two generalisations, because the existing
planner already takes the instrument as a parameter.

**Independent Test**: Add a second instrument with a handful of lots to the
register, run the analysis, and confirm it appears fully rated and priced
alongside the first, with no behaviour changed anywhere.

**Acceptance Scenarios**:

1. **Given** a register containing only one instrument, **When** the owner adds a second instrument with its lots and market price, **Then** both appear in the output with correct ratings, break-evens and holding-period maturity, with no change to the pack's behaviour.
2. **Given** a register with several instruments, **When** the owner asks for one of them specifically, **Then** only that instrument's lots and totals are reported.
3. **Given** a register with several instruments, **When** the owner asks for the portfolio as a whole, **Then** totals aggregate across instruments and each instrument remains separately identifiable.
4. **Given** an instrument in the register with no market price recorded, **When** the analysis runs, **Then** that instrument is reported as unpriced and excluded from value-dependent totals, rather than silently valued at zero.
5. **Given** a broker export naming an instrument absent from the register, **When** it is imported, **Then** the owner is told about that instrument explicitly rather than having its rows dropped silently.

---

### User Story 3 - Support a new broker by describing it, never by copying the program (Priority: P2)

The owner's shares sit at more than one broker. They add a broker by writing
one description of that broker's export — what its columns mean and what its
transaction labels signify — and everything downstream works. The existing
second broker, which today is served by a duplicated copy of the program,
is migrated onto that same description and its duplicate retired.

**Why this priority**: This is the stated purpose of the generalisation, and
the duplicated broker path is an active liability — two copies of the same
arithmetic that can and will drift apart. It sits alongside Story 2 rather than
after it because the duplicate should not be allowed to age further.

**Independent Test**: Take the second broker's real export, describe it as a
broker profile, import it, and confirm the resulting register matches what the
duplicated program produces — then delete the duplicate and confirm nothing is
lost.

**Acceptance Scenarios**:

1. **Given** a broker whose export differs in column names, date format, number format and transaction labels, **When** the owner supplies a profile describing those differences, **Then** its export imports correctly with no change to the pack's behaviour.
2. **Given** a broker whose export is a sectioned statement rather than a flat transaction list, **When** it is described under the same profile contract, **Then** it imports correctly — the contract accommodates both shapes without a second code path.
3. **Given** the second broker currently served by a duplicated program, **When** it is expressed as a profile and imported, **Then** the resulting register matches what the duplicate produces, and the duplicate is removed.
4. **Given** an export row whose transaction label matches no rule in the profile, **When** it is imported, **Then** the row is reported as unrecognised and the owner is prompted to extend the profile, and it is never silently discarded.
5. **Given** a broker profile, **When** it is validated against the published profile contract, **Then** a profile missing a required part is rejected with a message naming the missing part.
6. **Given** two brokers holding the same instrument, **When** both are imported, **Then** lots stay attributed to the broker they came from and disposals only consume lots consistent with the owner's disposal convention.

---

### User Story 4 - Read the page from a data document, embedded or fetched (Priority: P2)

The owner opens the interactive page and it renders from a single data
document. That document may be carried inside the page, so the file works
offline and can be moved around as one artefact; or the page may be pointed at
a location it fetches the document from, so the numbers can be refreshed
without rebuilding the page.

**Why this priority**: The owner asked for it directly. It also decouples two
things that are currently fused — regenerating the analysis and regenerating
the page — which is what makes any future scheduled refresh possible.

**Independent Test**: Build the page with the document embedded and open it
with no network; then build it pointed at a location, serve an updated
document there, and confirm the page shows the updated numbers without being
rebuilt.

**Acceptance Scenarios**:

1. **Given** a page built with the document embedded, **When** it is opened with no network access, **Then** it renders completely from the embedded copy.
2. **Given** a page configured to fetch from a location, **When** it is opened and the location responds, **Then** it renders from the fetched document and shows when that document was generated.
3. **Given** a page configured to fetch, **When** the location is unreachable or returns something unusable, **Then** the page still renders from its embedded copy and states clearly that it is showing fallback data and how old that data is.
4. **Given** a page and a document, **When** the document's contract version is one the page does not support, **Then** the page says so plainly instead of rendering wrong or partial figures.
5. **Given** the analysis has been re-run, **When** only the document is regenerated and republished, **Then** an already-built page pointed at that location shows the new figures with no rebuild.

---

### User Story 5 - Share the page without sharing the numbers (Priority: P3)

The owner wants to show the analysis to someone else — an accountant who needs
the real figures, or a spouse or peer who only needs the shape of the decision.
They choose how much the shared copy reveals, and the page states plainly what
has been withheld.

**Why this priority**: Sharing is a deliberate opt-in over a local-first
default, so it is not needed for the tool to be useful. It ranks above the
schema work because the risk it manages — personal financial detail leaving the
machine — is the only irreversible mistake this feature can make.

**Independent Test**: Produce a shared copy under a restricted disclosure
profile and confirm that no withheld figure appears anywhere in the delivered
artefact — including in the data document behind the page, not merely hidden in
the display.

**Acceptance Scenarios**:

1. **Given** no sharing is configured, **When** the analysis runs, **Then** nothing is published anywhere and the document stays on the owner's machine.
2. **Given** the owner selects a disclosure profile that withholds absolute money figures, **When** the shared copy is produced, **Then** the withheld figures are absent from the data document itself, not merely hidden by the page.
3. **Given** a shared copy under a restricted profile, **When** it is opened, **Then** it renders the decision-shaped content that survives redaction — ratings, percentage cushions, relative comparisons — and states which categories were withheld.
4. **Given** a disclosure profile, **When** it is validated, **Then** a profile that would leak a figure it claims to withhold is rejected before anything is produced.
5. **Given** the owner shares with an accountant, **When** they select the full-disclosure profile, **Then** the shared copy carries every figure, and producing it requires an explicit confirmation naming what is about to leave the machine.

---

### User Story 6 - Every data file has a published, enforced shape (Priority: P3)

Each kind of file the pack consumes or produces — lot register, rate table,
broker profile, tax rule set, disclosure profile, page data document — has a
published schema in the pack. Files are checked against those schemas before
they are used, so a typo is caught at the point of editing rather than
surfacing as a wrong number.

**Why this priority**: The data is hand-edited and the consequences of an
undetected error are financial. It is a safety net over Stories 1–5 rather than
a capability in itself. It is also the point at which the register becomes a
publishable contract the future `tax` pack can be written against.

**Independent Test**: Introduce a realistic error into each kind of data file
in turn — a misspelled field, a missing required field, a date where a number
belongs — and confirm each is rejected with a message identifying the file, the
location and the problem.

**Acceptance Scenarios**:

1. **Given** a data file that violates its schema, **When** the analysis runs, **Then** it stops before computing anything and reports the file, the offending location and what was expected.
2. **Given** a valid data file, **When** the analysis runs, **Then** validation passes without the owner having to invoke it separately.
3. **Given** the owner wants to check their edits, **When** they run validation on its own, **Then** every data file is checked and every problem is reported together, not just the first.
4. **Given** a data file carrying a contract version the pack does not support, **When** it is loaded, **Then** the mismatch is reported explicitly rather than being interpreted under the wrong contract.
5. **Given** the register schema is published, **When** a separate consumer reads the register against that schema alone, **Then** it has everything it needs without reading any of this pack's behaviour.

---

### Edge Cases

**Import and reconciliation**

- The same export is imported twice, or two exports with overlapping date ranges are imported in sequence — already-seen rows must be recognised and skipped, and the register must be unchanged the second time.
- One export contains two genuinely identical transactions on the same day (same size, same price) — indistinguishable from a duplicated row by content alone. The safe default is to drop the repeat loudly, with an explicit way for the owner to say it was real.
- A disposal is recorded that consumes more shares than the register holds, or names an instrument the register does not know.
- A disposal lands days before a lot reaches its long-term holding threshold — the owner must be warned, because the cost is real and invisible after the fact.
- A disposal must be split across several lots, or must consume part of a lot and leave the remainder open.
- Shares arrive by transfer from another broker with no cost basis in the export — the lot exists but is unvalued and must be flagged, not assumed to have cost zero.

**Valuation and rates**

- A lot's acquisition date has no verified exchange rate — the figure must be usable but visibly flagged as estimated, and the flag must reach the page.
- An acquisition's cost basis in the broker's export is not the basis the tax treatment requires (compensation-funded shares, where the employer's valuation governs) — the imported value must be flagged for reconciliation rather than trusted.
- An instrument is denominated in a currency other than the one the existing analysis assumes.

**Corporate actions**

- A share split, consolidation, spin-off or merger changes quantities and per-share basis for every lot of an instrument. This is a known gap in the existing analysis and must at minimum be detected and refused loudly rather than silently producing wrong per-share figures.

**Page, sharing and disclosure**

- The data document is empty, or contains an instrument with no open lots.
- The configured fetch location is reachable but returns an error page instead of the document.
- The page is opened from the local filesystem where fetching may be restricted by the browser.
- The fetched document is older than the embedded one.
- A disclosure profile withholds a figure that another figure in the same document can be used to reconstruct.
- A shared copy is regenerated after the disclosure profile was tightened — the previously shared artefact still exists elsewhere and cannot be recalled.

**Cross-pack boundary**

- The future `tax` pack reads the register while an import is midway through writing it.
- The register's schema version advances while a consumer is still written against the previous one.

**Boundary**

- A run attempts to write somewhere inside the pack directory that was never declared — this must fail rather than succeed quietly.

## Requirements *(mandatory)*

### Functional Requirements

**Pack and data boundary**

- **FR-001**: The pack MUST be installable and runnable through the workspace's existing pack and job mechanism, discoverable the same way every other pack is.
- **FR-002**: The pack directory MUST contain only behaviour, published schemas, sample/reference configuration and documentation. It MUST NOT contain any personal data, any produced output, or any machine-specific state.
- **FR-003**: Every file the pack reads as personal data or writes as output MUST resolve under the workspace data directory for this pack, and MUST be declared so the workspace can verify it.
- **FR-004**: A run MUST NOT write anywhere inside the pack directory except through those declared paths; an undeclared write MUST fail rather than succeed.
- **FR-005**: The pack MUST NOT depend on the Obsidian vault's directory layout, note formats, or local database for any part of its normal operation.
- **FR-006**: The pack MUST state which of its data files are intended to be versioned and which are local-only, per file, in a way the workspace's existing conventions can act on.

**Scope and the shared register contract**

- **FR-007**: This pack MUST cover the forward-looking analysis only — broker import, the lot register, disposal reconciliation, the sell planner and the interactive page. The fiscal-year tax-return computation is a separate pack and is out of scope here.
- **FR-008**: This pack MUST own the lot register: it is the only pack that writes it.
- **FR-009**: The register MUST be published under a versioned schema sufficient for a separate consumer to read it correctly without reference to this pack's behaviour.
- **FR-010**: The register MUST carry everything the later tax-return pack needs from it — per-lot acquisition facts, per-disposal facts including the date, quantity, proceeds and the lot consumed, and the flags marking any unverified value — so that pack never needs to re-derive them from broker exports.
- **FR-011**: Register writes MUST be atomic from a reader's point of view: a concurrent reader MUST see either the complete previous state or the complete new one, never a partial write.

**Instrument independence**

- **FR-012**: The pack MUST support any number of instruments in one register, with no instrument identifier, name or characteristic embedded in its behaviour.
- **FR-013**: Adding, removing or renaming an instrument MUST require only data edits.
- **FR-014**: The owner MUST be able to scope any output to a single instrument or to the whole portfolio.
- **FR-015**: The pack MUST report per-instrument results separately even when aggregating, so no figure loses its attribution.
- **FR-016**: An instrument lacking a market price MUST be reported as unpriced and excluded from value-dependent totals, never treated as worthless.

**Broker independence**

- **FR-017**: All knowledge of a broker's export — how to locate its data, what its columns mean, its date and number formats, and what its transaction labels signify — MUST live in a broker profile expressed as data.
- **FR-018**: Adding support for a new broker MUST require only a new broker profile; it MUST NOT require any change to behaviour.
- **FR-019**: There MUST be exactly one broker profile contract that every broker is described under, accommodating both flat transaction lists and sectioned statements without a second code path.
- **FR-020**: The pack MUST NOT contain a separate program path, or a duplicate of any shared computation, for any individual broker.
- **FR-021**: The currently duplicated second-broker path MUST be expressed as a broker profile under the single contract, produce a matching register, and the duplicate MUST then be removed.
- **FR-022**: An export row whose transaction label matches no rule in its profile MUST be surfaced to the owner as unrecognised, and MUST NOT be silently ignored.
- **FR-023**: The pack MUST attribute each lot to the broker it came from and preserve that attribution through disposal and reporting.

**Import, register and reconciliation**

- **FR-024**: The pack MUST maintain a register of open lots and closed disposals, with each lot carrying at minimum its instrument, broker, acquisition date, quantity, cost basis, acquisition exchange rate and how it was funded.
- **FR-025**: The pack MUST reconcile a broker export against the register, creating lots for acquisitions and matching disposals to lots under the owner's stated disposal convention, splitting a lot where a disposal consumes part of it.
- **FR-026**: Import MUST be idempotent: re-importing rows already imported MUST leave the register unchanged, and this MUST hold across exports with overlapping date ranges.
- **FR-027**: Import MUST offer a preview mode that reports every lot it would create and every disposal it would match, and writes nothing.
- **FR-028**: Import MUST warn when a disposal occurred shortly before the affected lot would have reached its long-term holding threshold.
- **FR-029**: Values the pack cannot verify — an unresolved exchange rate, a cost basis the tax treatment does not accept from the broker, a transferred lot with no basis — MUST be recorded with a durable flag that reaches every output including the page and the published register.
- **FR-030**: The pack MUST detect a corporate action that would invalidate per-share figures and refuse to produce affected figures rather than reporting them as if nothing happened.

**Planning and rating knowledge**

- **FR-031**: Holding-period thresholds, tax rates, surcharges, disposal-matching conventions and lot-rating thresholds MUST all be expressed as data, not embedded in behaviour.
- **FR-032**: Adding or altering a lot rating, or its threshold, MUST require only a data edit.
- **FR-033**: Every rating MUST carry, alongside its result, an explanation stated in that specific lot's own numbers rather than in generic threshold terms.
- **FR-034**: The pack MUST preserve the existing meaning of its visual encoding, so that one colour carries one meaning consistently across every element of every output.
- **FR-035**: The pack MUST reproduce the vault program's current register and page figures exactly from the same inputs, and MUST be able to demonstrate that on demand as a regression check.

**Page and data document**

- **FR-036**: The page MUST obtain everything it renders from a single data document; it MUST NOT contain any figure baked into its markup.
- **FR-037**: The pack MUST be able to produce a page carrying the data document inside it, which renders completely with no network access.
- **FR-038**: The pack MUST be able to produce a page that fetches its data document from a configured location at load time.
- **FR-039**: A page configured to fetch MUST fall back to its embedded copy when the location is unreachable or unusable, and MUST state visibly that it is showing fallback data and when that data was generated.
- **FR-040**: The data document MUST carry a contract version and a generation timestamp, and the page MUST refuse to render figures from a version it does not support.
- **FR-041**: Regenerating and republishing the data document alone MUST update what an already-built fetching page displays, with no rebuild of the page.
- **FR-042**: The page MUST remain a single self-contained file with no dependency on any external service for its own code, styling or assets.

**Sharing and disclosure**

- **FR-043**: The default posture MUST be local-only: with no sharing configured, the pack MUST NOT transmit or publish anything, and the fetch location MUST default to a local path or locally-served address.
- **FR-044**: Producing a copy intended to leave the owner's machine MUST require explicit configuration; it MUST NOT be reachable as a default, a fallback, or a side effect of any other operation.
- **FR-045**: What a shared copy reveals MUST be governed by a disclosure profile expressed as data, so a new audience is a data edit rather than a behaviour change.
- **FR-046**: The pack MUST ship at least two disclosure profiles: one revealing every figure, and one withholding absolute money amounts and quantities while retaining the decision-shaped content — ratings, percentage cushions, relative comparisons and holding-period maturity.
- **FR-047**: Redaction MUST remove withheld figures from the data document itself, not merely hide them in the page's display.
- **FR-048**: A shared copy MUST state which categories of figure were withheld from it.
- **FR-049**: A disclosure profile MUST be rejected before anything is produced if a figure it claims to withhold remains derivable from the figures it retains.
- **FR-050**: Producing a full-disclosure copy MUST require an explicit confirmation that names what is about to leave the machine.

**Schemas and validation**

- **FR-051**: The pack MUST publish a schema for every kind of data file it consumes or produces, including the lot register, the page data document and disclosure profiles.
- **FR-052**: Data files MUST be validated against their schemas before being used, and a violation MUST stop the run before any figure is computed.
- **FR-053**: A validation failure MUST identify the file, the location within it, and what was expected.
- **FR-054**: The owner MUST be able to validate all data files in one command, receiving every problem at once rather than the first.
- **FR-055**: Data files MUST carry a contract version, and a version the pack does not support MUST be reported rather than reinterpreted.

**Migration and retirement**

- **FR-056**: There MUST be a documented, repeatable migration that moves the existing register and rate table from the vault into the pack's workspace data directory.
- **FR-057**: The migration MUST be verifiable by comparing figures produced before and after, and MUST be reversible until that comparison passes.
- **FR-058**: The vault-resident program MUST be retained, read-only, as a cross-check for one filing cycle after the pack goes into use — long enough to cover the FY2026-27 return.
- **FR-059**: During that period the vault program MUST NOT be edited, and any divergence between it and the pack MUST be investigated and resolved before the pack's figure is trusted.
- **FR-060**: The retirement MUST be time-boxed with a recorded target date rather than left open-ended, and the vault program deleted once the FY2026-27 return has been filed using the pack's figures.

### Key Entities

- **Position**: An instrument held by the owner — its identifier, the currency it trades in, its current market price and the date that price was observed. Owns a set of lots.
- **Lot**: One acquisition of a quantity of an instrument. Carries acquisition date, quantity, cost basis per share, cash actually paid per share, the exchange rate at acquisition, how it was funded, the broker it sits at, its provenance fingerprint, and any unverified-value flags. Reaches long-term status a defined period after acquisition.
- **Disposal**: A closed lot or part of one — the lot it came from, quantity, disposal date and price, proceeds, the holding period achieved and the rate that applied. The unit the later `tax` pack computes realised gains from.
- **Funding class**: How a lot was paid for — granted compensation, discounted purchase, or the owner's own cash. Determines how much of the position represents the owner's money rather than taxed compensation.
- **Broker profile**: A description of one broker's export — how to find its data, what its columns mean, its date and number conventions, and what each transaction label signifies. The only place broker-specific knowledge exists.
- **Tax rule set**: Holding-period thresholds, rates, surcharges and disposal-matching conventions for one jurisdiction, as far as the planning question needs them.
- **Lot rating**: A verdict on one lot — whether selling it now is cheap or expensive — with its conditions, its visual tone, and an explanation template filled from that lot's own figures.
- **Rate table**: Dated foreign-exchange rates with their provenance, distinguishing verified rates from estimated ones.
- **Data document**: The single self-describing artefact the page renders from — versioned, timestamped, containing positions, lots, disposals, ratings and the constants needed to recompute at a different price or date.
- **Disclosure profile**: A named description of what a shared copy may reveal and what it must withhold.
- **Import batch**: One ingestion of one export — its source, what it created, what it skipped as already seen, and what it could not recognise.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The pack's register and page figures match what the vault program produces from the same inputs exactly — zero discrepancies.
- **SC-002**: The pack directory contains zero personal data files and zero produced-output files; the workspace's own drift check confirms this on demand.
- **SC-003**: Adding a new instrument to the analysis requires changes to data only — zero changes to behaviour — and an owner can complete it in under 10 minutes.
- **SC-004**: Adding a new broker requires exactly one new broker profile and zero changes to behaviour, demonstrated by doing it for a broker not previously supported.
- **SC-005**: There is exactly one implementation of every shared computation — no broker has its own copy of any figure the pack produces.
- **SC-006**: Re-importing an export already imported produces a register that is byte-for-byte unchanged, in 100% of cases.
- **SC-007**: Every transaction row in a broker export is either imported or explicitly reported to the owner; zero rows are dropped without being accounted for.
- **SC-008**: The page renders complete and correct figures in all three conditions — embedded document with no network, fetched document, and fetch failure falling back to embedded — with the fallback clearly labelled in the third.
- **SC-009**: Republishing only the data document updates what an existing fetching page shows, with zero page rebuilds.
- **SC-010**: With no sharing configured, zero bytes of the owner's data leave the machine under any operation the pack offers.
- **SC-011**: A shared copy produced under a restricted disclosure profile contains zero withheld figures anywhere in its delivered bytes, verified by searching the artefact itself rather than by inspecting the rendered page.
- **SC-012**: Every kind of data file the pack uses has a published schema, and a deliberately corrupted file of each kind is rejected with a message naming the file and location, in 100% of cases.
- **SC-013**: A consumer written only against the published register schema can read the register correctly, demonstrated without reference to the pack's behaviour — the precondition for specifying the `tax` pack.
- **SC-014**: A run started with data missing or invalid produces zero figures — it never emits a partial result that could be mistaken for a complete one.
- **SC-015**: Any figure derived from an unverified input carries its flag through to every output the owner can see, including the page and the published register.

## Assumptions

- **The existing analysis is the specification of correct behaviour.** Its tax conventions, disposal-matching rules, break-even arithmetic and rating thresholds are carried over as-is. Where this feature changes a number, that is a defect unless called out as a deliberate correction.
- **Jurisdiction stays single.** The shipped rule set remains the India treatment the current work encodes. The rules are data, so a second jurisdiction is a new rule set rather than a code change, but no second jurisdiction is delivered here.
- **The reporting currency stays as today** (Indian rupees, from a foreign-currency-denominated instrument), with the existing convention that both legs of a gain are converted so that exchange-rate movement is captured. Instruments in other denominations are accommodated in the data shape but not proven in this feature.
- **The disposal-matching convention stays first-in-first-out**, with the existing specific-identification exception for same-week disposals against a compensation vest.
- **The page stays a single self-contained file.** No build pipeline, no external code, styling or asset dependencies — it must keep working when opened directly from disk years from now.
- **Sharing is opt-in and local-first**, per the resolved clarification. The pack provides the disclosure mechanism; it does not provide hosting, and choosing where a shared copy goes remains the owner's decision.
- **Corporate actions are out of scope as a capability**, but in scope as a hazard: the pack must detect and refuse rather than silently mislead.
- **The pack's visibility follows the existing convention** for packs holding personal financial data.
- **The vault keeps its own copy of the source documents** (statements, slips, notes) as the human archive; the pack's workspace data directory becomes the operational source of truth for anything the analysis reads.
- **Compensation slips and per-fiscal-year facts stay in the vault for now** and move with the `tax` pack, since only the tax-return computation consumes them. The one exception is any slip figure that governs a lot's cost basis, which the register already carries as the lot's own value.
- **The `tax` pack is specified separately** and is not blocked by anything in this feature beyond the published register schema (FR-009, SC-013).

## Dependencies

- The workspace's pack registration, job manifest and job-running mechanism.
- The workspace's configuration- and data-file injection convention, which is what makes "no data inside the pack" enforceable rather than aspirational.
- The workspace's write-sandbox, which is what turns FR-004 from documentation into a guarantee.
- The vault-resident program, retained read-only through the FY2026-27 filing cycle as the correctness oracle for SC-001 and FR-058.
- Real export files from each broker to be supported, for validating each broker profile.

## Out of Scope

- **The fiscal-year tax-return computation** — perquisite rollup, dividend and withholding gross-up with foreign tax credit, year-end foreign-asset holdings, card-spend bucketing and the slab liability estimate. This is the separate `tax` pack, specified as its own feature, reading this pack's register.
- Any new analysis, jurisdiction or asset class beyond what the existing work covers.
- Automatic fetching of market prices or exchange rates from any external service.
- Executing, recommending or placing trades. The pack reports arithmetic on the owner's own data; it holds no view on any company.
- Handling corporate actions correctly (detection and refusal only — see Assumptions).
- Hosting a shared page. The pack produces a redacted artefact; where it is served from is the owner's choice.
- Migrating the vault's other analyses (expenses, banking, purchases) into packs.
