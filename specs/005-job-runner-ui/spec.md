# Feature Specification: One-Click Job Runner UI

**Feature Branch**: `005-job-runner-ui`

**Created**: 2026-07-26

**Status**: Draft

**Input**: User description: "bring in all the commands feasible as buttons with appropriate descriptions for one click running of actions. example. have a drop down to choose the --ai profile (e.g., deepseek.yaml). should be able to run the orchestraions, should be able tin run gmail-extract etc from the UI. every tun should be running in a tmux session, managed by the app, each session should be deleted after the run, each run should log the logs into a log file with audit ID for each line for easier recognision of runs when multiple runs are running in parallel. eacvh job should have a listing in the jobs section, where the user can see the progress and logs of that specifc run, live updated (like how jenkins shows logs)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch a job with one click (Priority: P1)

As the operator of this workspace, I want to see every runnable job and pipeline as a labelled button with a description of what it does, and start one by clicking it, so I don't have to remember command names, flags, or which directory to run them from.

**Why this priority**: This is the core of the request — replacing memorised terminal invocations with discoverable, one-click actions. Nothing else in this feature has value without it.

**Independent Test**: Open the dashboard, find the button for a known job (e.g. Gmail transaction extract), read its description, click it, and confirm the job actually starts.

**Acceptance Scenarios**:

1. **Given** the workspace has jobs and pipelines defined, **When** the user opens the dashboard, **Then** each runnable action appears as a button labelled with its human-readable name and accompanied by its description, grouped so jobs and multi-step pipelines are distinguishable.
2. **Given** an action button is displayed, **When** the user clicks it, **Then** the run starts and the user is given immediate confirmation that it started, including a way to reach that run's live output.
3. **Given** an action cannot be run right now (e.g. it is not permitted on this machine, or its required configuration is missing), **When** the user views it, **Then** the button is shown as unavailable with the reason stated, rather than failing only after being clicked.

---

### User Story 2 - Watch a run's progress and logs live (Priority: P1)

As the operator, I want each run to appear in a runs list with its current status, and to open it and watch its output stream in as it happens, so I can tell whether a long-running job is progressing or stuck without switching to a terminal.

**Why this priority**: Equal to Story 1 — a one-click launcher that gives no feedback is unusable for jobs that take minutes and call external APIs. The user explicitly asked for Jenkins-style live logs.

**Independent Test**: Start a job that produces output over time, open its entry in the runs list, and confirm new output lines appear without manually reloading the page, and that the entry reaches a terminal status when the run ends.

**Acceptance Scenarios**:

1. **Given** a run has been started, **When** the user opens the runs list, **Then** that run is listed with its action name, start time, elapsed time, and current status (running / succeeded / failed / timed out / cancelled).
2. **Given** a run is in progress, **When** the user opens that run's detail view, **Then** the output produced so far is shown and new output continues to appear as the run produces it, without the user reloading the page.
3. **Given** a run has finished, **When** the user opens it later, **Then** the complete output of that run is still readable, along with its final status and exit result.
4. **Given** several runs are in progress at once, **When** the user views any one of them, **Then** the output shown belongs only to that run and is not interleaved with the others.

---

### User Story 3 - Choose an AI profile before running (Priority: P2)

As the operator, I want to pick which AI credential profile a run should use from a dropdown of the profiles that actually exist, so I can run the same job against different providers without editing files or typing profile names.

**Why this priority**: Valuable and explicitly requested, but the majority of actions either need no AI profile or have a sensible default — so Stories 1 and 2 deliver working value without it.

**Independent Test**: With more than one AI profile configured, select a specific one from the dropdown, launch a job that uses AI, and confirm from the run's own output that the selected profile was the one applied.

**Acceptance Scenarios**:

1. **Given** AI profiles are configured in the workspace, **When** the user views an action that accepts one, **Then** a dropdown lists exactly the configured, usable profiles by name, and template/example profiles that hold no real credentials are not offered.
2. **Given** the user selects a profile and launches the action, **When** the run starts, **Then** that run uses the selected profile, and the run's record shows which profile was used.
3. **Given** an action does not accept an AI profile selection, **When** the user views it, **Then** no misleading dropdown is offered for it.

---

### User Story 4 - Audit and trace runs after the fact (Priority: P2)

As the operator, I want every run's output written to a log file where each line carries the run's own audit identifier, so that when several runs happen together I can tell which line came from which run, and can still investigate a run days later.

**Why this priority**: The user explicitly asked for this and it's what makes parallel runs debuggable, but live viewing (Story 2) already covers the immediate need, so this can follow.

**Independent Test**: Run two actions at the same time, then inspect the stored logs and confirm every line is attributable to exactly one run via its audit identifier, and that both runs' histories are complete.

**Acceptance Scenarios**:

1. **Given** a run is launched, **When** it produces output, **Then** every output line is recorded with that run's unique audit identifier and a timestamp.
2. **Given** two or more runs execute concurrently, **When** their logs are inspected, **Then** no line is ambiguous about which run produced it.
3. **Given** a run has completed, **When** the user looks it up by its audit identifier, **Then** they can retrieve that run's full log and its outcome.

---

### Edge Cases

- **A run's underlying session dies or the dashboard is restarted mid-run**: the run must not be left displayed as "running" forever — it must be reconciled to a definite status, and whatever output was captured must be preserved.
- **An action prompts for input**: some existing actions ask the user questions when run interactively. Runs started from the dashboard proceed without prompting (FR-015) — meaning the interactive-only features of those actions are simply not exercised from the UI, and remain available only from a terminal. The user should not be led to expect otherwise.
- **A machine-altering action is clicked by accident**: installing scheduled tasks or re-syncing submodules changes state outside the workspace's data, so these require an explicit confirmation before running (FR-005).
- **The same action is launched twice while the first is still running**: several actions read and rewrite the same shared data files, so a second concurrent run of the same action could corrupt that data.
- **Launching when the run environment prerequisite is unavailable**: if the session tooling the app depends on isn't installed on the machine, the user must get a clear, actionable message rather than a silent failure.
- **A run produces an enormous amount of output**: the live view and log storage must stay usable rather than exhausting memory or freezing the page.
- **A run is still going when the user wants it stopped**: the user needs a way to stop it and have the result recorded honestly as cancelled.
- **Two dashboard browser tabs open at once**: both must show a consistent view of the same runs.

## Requirements *(mandatory)*

### Functional Requirements

#### Discovering and launching actions

- **FR-001**: The system MUST present every runnable job defined in the workspace, and every multi-step pipeline defined in the workspace, as a distinct action with a human-readable name and its description shown to the user.
- **FR-002**: The system MUST let the user start any presented action with a single click, without typing a command or choosing a working directory.
- **FR-003**: The system MUST indicate, before the user clicks, when an action cannot currently be run (for example: not permitted on this machine, or required configuration missing), and state why.
- **FR-004**: The system MUST additionally offer, as buttons: the read-only inspection commands (listing jobs, validating the workspace for configuration and visibility problems, regenerating the catalog, showing per-pack config status), and the state-changing maintenance commands (installing this machine's scheduled tasks, and re-syncing the workspace's submodules).
- **FR-005**: The system MUST visually distinguish actions that change state outside the workspace's own data — specifically those that modify the machine's scheduled tasks or the working tree — and MUST require an explicit confirmation step before running one, so a single stray click cannot alter the machine.
- **FR-006**: The system MUST refuse to start a second run of the same action while one is already in progress, and tell the user why, because several actions rewrite the same shared data files.

#### Selecting an AI profile

- **FR-007**: The system MUST offer a selection of the AI credential profiles configured in this workspace, by name, for actions that accept one.
- **FR-008**: The system MUST exclude example/template profiles that contain no real credentials from that selection.
- **FR-009**: The system MUST apply the selected profile to the run it was chosen for, and MUST record which profile was used with that run's record.
- **FR-010**: The system MUST NOT display an AI profile selector for actions that cannot accept one.
- **FR-011**: The system MUST NOT display any credential value from a profile anywhere in the interface or in stored logs.

#### Running, isolation, and lifecycle

- **FR-012**: The system MUST execute each run inside its own managed terminal session, isolated from other runs.
- **FR-013**: The system MUST remove that session once the run has finished, leaving no orphaned sessions behind.
- **FR-014**: The system MUST let the user stop an in-progress run, and MUST record such a run's outcome as cancelled.
- **FR-015**: The system MUST run every action non-interactively, such that an action which would otherwise prompt for input instead proceeds without prompting — taking the same path it already takes on an unattended scheduled run. No run started from the interface may block waiting for input.
- **FR-016**: The system MUST reconcile any run that is no longer actually executing (for example after the dashboard restarts or a session dies) to a definite, non-running status, preserving whatever output was captured.
- **FR-017**: The system MUST verify at startup that the terminal-session tooling it depends on is available on this machine, and MUST report its absence clearly, naming what is missing and how to install it, rather than failing only when a run is attempted. The system MUST NOT silently fall back to a different execution mechanism.

#### Observing runs

- **FR-018**: The system MUST list all runs — in progress and completed — showing for each: the action name, when it started, how long it has been running or took, and its current status.
- **FR-019**: The system MUST provide a per-run view showing that run's output, which updates live as new output is produced, without the user reloading the page.
- **FR-020**: The system MUST keep each run's output separate from every other run's, including when runs execute concurrently.
- **FR-021**: The system MUST retain a completed run's full output so it remains readable after the run has ended.
- **FR-022**: The system MUST show a run's final outcome (succeeded / failed / timed out / cancelled) when it ends.
- **FR-023**: For multi-step pipeline runs, the system MUST show progress at the step level, so the user can see which step is currently executing and how earlier steps concluded.

#### Logging and audit

- **FR-024**: The system MUST assign every run a unique audit identifier at launch, and MUST surface that identifier to the user.
- **FR-025**: The system MUST write each run's output to a log file, with every line carrying that run's audit identifier and a timestamp, so lines remain attributable when multiple runs happen in parallel.
- **FR-026**: The system MUST allow a past run to be located by its audit identifier and its full log retrieved.
- **FR-027**: The system MUST NOT commit run logs to version control.

#### Access and safety

- **FR-028**: The system MUST remain reachable only from the machine it runs on, since it can now execute workspace actions with real credentials injected.
- **FR-029**: The system MUST require an explicit user action to start a run — no action may be triggered by merely loading or refreshing a page.

### Key Entities *(include if feature involves data)*

- **Action**: Something the user can launch with one click. Derived from what the workspace already defines — an individual job, or a multi-step pipeline. Has a name, a description, an availability state (with reason when unavailable), and whether it accepts an AI profile.
- **Run**: One execution of an Action. Has a unique audit identifier, the action it ran, the AI profile used (if any), who/what started it, start time, end time, status (running / succeeded / failed / timed out / cancelled), and a reference to its log. For pipeline runs, also the per-step progress.
- **AI Profile**: A named set of AI provider credentials already configured in this workspace. Only its *name* is ever shown; its credential values are never displayed or logged.
- **Run Log**: The captured output of one Run, stored as a file, every line tagged with the Run's audit identifier and a timestamp.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from opening the dashboard to having a chosen job running in under 10 seconds, without typing any command.
- **SC-002**: 100% of the workspace's runnable jobs and pipelines appear as launchable actions, with no action requiring the user to know its command-line name.
- **SC-003**: New output from a running action becomes visible in that run's live view within 2 seconds of being produced.
- **SC-004**: With at least 3 runs executing concurrently, 100% of stored log lines can be attributed to exactly one run by its audit identifier.
- **SC-005**: 100% of runs reach a definite final status — no run is left displayed as "running" after it has stopped executing, including across a dashboard restart.
- **SC-006**: No managed session remains on the machine more than 10 seconds after the run that owned it has finished.
- **SC-007**: Zero credential values appear anywhere in the interface or in stored run logs.
- **SC-008**: 100% of actions that alter state outside the workspace's own data require a confirmation step, and none can be triggered by a single click.
- **SC-009**: Zero runs started from the interface stall waiting for input that cannot be provided.

## Assumptions

- This capability extends the workspace-level dashboard that already lists packs, jobs and commands, rather than introducing a separate application — the user referred to "the jobs section", which already exists there. *(This means the dashboard stops being read-only, which was a deliberate earlier decision made on the grounds that a read-only page needs no auth and has no state to get stale; that decision is superseded by this feature and must be re-recorded, along with the new confirmation gate in FR-005 that partially compensates for it.)*
- The transactions editor UI delivered previously remains a separate, pack-level interface; unifying the two is out of scope here.
- The list of actions continues to be derived from the workspace's existing job manifests and pipeline definitions, so adding a new job or pipeline makes it appear in the UI with no separate registration step.
- AI profile selection applies to individual job runs. Multi-step pipelines already specify their AI profile per step in their own definition, so the dropdown does not override those.
- The dashboard is used by a single operator on their own machine; no multi-user accounts, roles, or per-user permissions are required.
- Run history for completed runs is retained indefinitely; log files are kept on local disk and excluded from version control.
- "Progress" for an individual job means its live output plus its running/finished status; only multi-step pipelines have a more granular notion of progress (which step of how many).
- The machine running the dashboard must have the terminal-session tooling installed; this is a hard prerequisite with no fallback execution path (FR-017). It is **not currently installed on this machine**, so it must be added before the feature can run.
- Because every run is non-interactive (FR-015), the interactive-only capabilities of certain actions — retroactive suggestion walks and prompt-driven rule capture — are intentionally unreachable from the dashboard and stay terminal-only. This mirrors how those same features already behave on scheduled runs, so it introduces no new behavior, only a boundary on what the UI can do.

## Dependencies

- The workspace's existing job and pipeline definitions, and the existing shared execution path that runs them (so a run launched from the UI behaves the same as one launched from the terminal).
- The workspace's existing AI profile configuration.
- Terminal multiplexer tooling on the host machine, for the per-run isolated sessions (**currently absent on this machine — must be installed**).
- The existing run-history storage already used to record job and pipeline runs.
