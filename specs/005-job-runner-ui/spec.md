# Feature Specification: One-Click Job Runner UI

**Feature Branch**: `005-job-runner-ui`

**Created**: 2026-07-26

**Status**: Draft

**Input**: User description: "bring in all the commands feasible as buttons with appropriate descriptions for one click running of actions. example. have a drop down to choose the --ai profile (e.g., deepseek.yaml). should be able to run the orchestraions, should be able tin run gmail-extract etc from the UI. every tun should be running in a tmux session, managed by the app, each session should be deleted after the run, each run should log the logs into a log file with audit ID for each line for easier recognision of runs when multiple runs are running in parallel. eacvh job should have a listing in the jobs section, where the user can see the progress and logs of that specifc run, live updated (like how jenkins shows logs)"

**Additional input (2026-07-27)**: "we need to make all the commands to request --data-dir without which the run should fail. the if ENV AUTO_DATA_DIR is available, that should be fine. but either should have a valid path with files in it. same for --config-dir (ENV AUTO_CONFIG_DIR). In UI have a common dropdown top right to choose the AI profile for all the commands."

## Clarifications

### Session 2026-07-27

- Q: What should the single top-right AI profile dropdown apply to, given pipelines set `ai:` per step and plain commands accept none? → A: It sets a workspace-wide **default** applied to every run; an orchestration step that names its own `ai:` still overrides it.
- Q: Which commands must fail without a valid data/config directory? → A: Only actions that perform workspace work (jobs and orchestrations). Read-only inspection actions (list, packs, doctor, catalog, config) remain usable without them.
- Q: How strictly should "a valid path with files in it" be checked? → A: Strictly — the directory must exist, be a directory, and contain the expected substructure (data: `state/` and `config/`; config: `ai/` or at least one pack directory), not merely be non-empty.
- Q: Should the dashboard itself refuse to start without valid directories? → A: Yes — it exits with the same error the CLI gives, rather than starting in a degraded state.

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

### User Story 3 - Choose one AI profile for everything I run (Priority: P2)

As the operator, I want to pick the AI credential profile once, in a single control at the top right of the dashboard, and have every run I start use it, so I can switch providers for my whole session without setting it per action or editing files.

**Why this priority**: Valuable and explicitly requested, but most actions either need no AI profile or already have one configured — so Stories 1 and 2 deliver working value without it.

**Independent Test**: With more than one AI profile configured, choose one in the top-right control, launch a job that uses AI, and confirm from the run's own output and its record that the chosen profile was applied.

**Acceptance Scenarios**:

1. **Given** AI profiles are configured in the workspace, **When** the user opens the dashboard, **Then** a single control at the top right lists exactly the configured, usable profiles by name; template/example profiles holding no real credentials are not offered.
2. **Given** the user has chosen a profile, **When** they launch any action, **Then** that run uses the chosen profile as its default, and the run's record shows which profile was used.
3. **Given** the user has chosen a profile **and** launches an orchestration whose step names its own profile, **When** that step runs, **Then** the step's own profile is used for it, because explicit per-step configuration outranks the session-wide default.
4. **Given** the user has chosen no profile, **When** they launch any action, **Then** the run proceeds with whatever credentials the environment already provides, exactly as it did before this control existed.

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

### User Story 5 - Never run against the wrong directories (Priority: P1)

As the operator, I want any action that touches workspace data to refuse to start unless it has been told explicitly where the data and config directories are — and unless those directories actually look like the real thing — so a misconfigured invocation fails immediately instead of silently reading nothing, writing to the wrong place, or corrupting real records.

**Why this priority**: This is a safety guard on everything the other stories launch. Several jobs read-modify-write shared CSVs and SQLite files; pointing one at an empty or wrong directory can destroy real data or produce a confidently empty result. Failing loudly up front is worth more than any convenience it costs.

**Independent Test**: Invoke a job with no directory specified, with a nonexistent path, and with an existing-but-wrong directory; confirm each is refused with a message naming what was wrong, and that nothing was read or written in any of the three cases.

**Acceptance Scenarios**:

1. **Given** neither the data directory option nor its environment variable is set, **When** the user runs a job or orchestration, **Then** it fails before doing any work, with a message stating which directory is missing and how to supply it.
2. **Given** the environment variable is set to a valid directory, **When** the user runs a job without passing the option, **Then** the run proceeds normally — the environment variable is a complete substitute for the option.
3. **Given** a directory is supplied but does not exist, is not a directory, or lacks the expected substructure, **When** the user runs a job, **Then** it fails before doing any work, naming the path and what was expected of it.
4. **Given** both the option and the environment variable are supplied with different values, **When** a run starts, **Then** the explicitly-passed option is used, and the run's record reflects the directories actually used.
5. **Given** the directories are invalid, **When** the user runs a read-only inspection action, **Then** it still works — those actions never touch the data or config directories, and remaining usable is what lets the operator diagnose the problem.

---

### Edge Cases

- **A run's underlying session dies or the dashboard is restarted mid-run**: the run must not be left displayed as "running" forever — it must be reconciled to a definite status, and whatever output was captured must be preserved.
- **An action prompts for input**: some existing actions ask the user questions when run interactively. Runs started from the dashboard proceed without prompting (FR-025) — meaning the interactive-only features of those actions are simply not exercised from the UI, and remain available only from a terminal. The user should not be led to expect otherwise.
- **A machine-altering action is clicked by accident**: installing scheduled tasks or re-syncing submodules changes state outside the workspace's data, so these require an explicit confirmation before running (FR-005).
- **The same action is launched twice while the first is still running**: several actions read and rewrite the same shared data files, so a second concurrent run of the same action could corrupt that data.
- **Launching when the run environment prerequisite is unavailable**: if the session tooling the app depends on isn't installed on the machine, the user must get a clear, actionable message rather than a silent failure.
- **A run produces an enormous amount of output**: the live view and log storage must stay usable rather than exhausting memory or freezing the page.
- **A run is still going when the user wants it stopped**: the user needs a way to stop it and have the result recorded honestly as cancelled.
- **Two dashboard browser tabs open at once**: both must show a consistent view of the same runs.
- **A directory is supplied but points somewhere plausible-but-wrong** (a home directory, an unrelated project, a freshly-created empty folder): a mere existence check would accept all of these, so the expected substructure is checked too and the run is refused.
- **A directory becomes invalid while the dashboard is running** (renamed, unmounted, deleted): a run launched afterwards must fail with the same clear message rather than starting and failing obscurely partway through.
- **An AI profile is chosen, then deleted or edited to be invalid before a run starts**: the run must not proceed with a half-resolved credential; the selection must be re-validated at launch and refused with a clear reason if it no longer resolves.
- **A run is launched while no AI profile is selected**: this is legitimate and must behave exactly as before this control existed — most actions need no AI profile at all.

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

- **FR-007**: The system MUST offer the AI credential profiles configured in this workspace, by name, in a **single control at the top right of the interface** that applies to every action — not as a separate control per action.
- **FR-008**: The system MUST exclude example/template profiles that contain no real credentials from that selection.
- **FR-009**: The system MUST apply the chosen profile to every run started while it is selected, as that run's **default** AI profile, and MUST record which profile was used with that run's record.
- **FR-010**: The system MUST let an orchestration step's own configured AI profile take precedence over the session-wide selection for that step, because explicit per-step configuration outranks a default. The selection MUST NOT silently overwrite profiles already declared in a pipeline definition.
- **FR-011**: The system MUST treat "no profile selected" as valid, leaving runs to use whatever credentials the environment already provides — the behaviour that applied before this control existed.
- **FR-012**: The system MUST re-validate the chosen profile at launch time and refuse the run with a clear reason if it no longer resolves to usable credentials.
- **FR-013**: The system MUST NOT display any credential value from a profile anywhere in the interface or in stored logs.

#### Locating workspace data and configuration

- **FR-014**: The system MUST require an explicit data directory for every action that performs workspace work (jobs and multi-step pipelines), supplied either as a command option or as its corresponding environment variable. Neither source is preferred over the other for validity purposes; a value from either is equally acceptable.
- **FR-015**: The system MUST require an explicit configuration directory on the same terms, with its own command option and environment variable.
- **FR-016**: The system MUST use the explicitly-passed option when both the option and the environment variable are supplied.
- **FR-017**: The system MUST validate each supplied directory before performing any work: it must exist, be a directory, and contain the substructure expected of it — the data directory must contain the run-state and shared-config subdirectories, and the configuration directory must contain the AI-profile subdirectory or at least one pack's configuration directory. A path that merely exists, or that exists but is empty, MUST be rejected.
- **FR-018**: The system MUST fail before reading or writing anything when a required directory is absent or fails validation, and the failure message MUST name which directory was at fault, what was wrong with it, and how to supply a correct one.
- **FR-019**: Read-only inspection actions (listing jobs, listing packs, validating the workspace, regenerating the catalog, showing config status) MUST remain usable without these directories, since they never read or write them and are what the operator needs in order to diagnose a misconfiguration.
- **FR-020**: The dashboard MUST refuse to start when the directories available to it are absent or fail validation, exiting with the same message the command line gives, rather than starting in a state where actions would fail individually.
- **FR-021**: The system MUST record the data and configuration directories actually used with each run's record, so a past run can be traced to the directories it operated on.

#### Running, isolation, and lifecycle

- **FR-022**: The system MUST execute each run inside its own managed terminal session, isolated from other runs.
- **FR-023**: The system MUST remove that session once the run has finished, leaving no orphaned sessions behind.
- **FR-024**: The system MUST let the user stop an in-progress run, and MUST record such a run's outcome as cancelled.
- **FR-025**: The system MUST run every action non-interactively, such that an action which would otherwise prompt for input instead proceeds without prompting — taking the same path it already takes on an unattended scheduled run. No run started from the interface may block waiting for input.
- **FR-026**: The system MUST reconcile any run that is no longer actually executing (for example after the dashboard restarts or a session dies) to a definite, non-running status, preserving whatever output was captured.
- **FR-027**: The system MUST verify at startup that the terminal-session tooling it depends on is available on this machine, and MUST report its absence clearly, naming what is missing and how to install it, rather than failing only when a run is attempted. The system MUST NOT silently fall back to a different execution mechanism.

#### Observing runs

- **FR-028**: The system MUST list all runs — in progress and completed — showing for each: the action name, when it started, how long it has been running or took, and its current status.
- **FR-029**: The system MUST provide a per-run view showing that run's output, which updates live as new output is produced, without the user reloading the page.
- **FR-030**: The system MUST keep each run's output separate from every other run's, including when runs execute concurrently.
- **FR-031**: The system MUST retain a completed run's full output so it remains readable after the run has ended.
- **FR-032**: The system MUST show a run's final outcome (succeeded / failed / timed out / cancelled) when it ends.
- **FR-033**: For multi-step pipeline runs, the system MUST show progress at the step level, so the user can see which step is currently executing and how earlier steps concluded.

#### Logging and audit

- **FR-034**: The system MUST assign every run a unique audit identifier at launch, and MUST surface that identifier to the user.
- **FR-035**: The system MUST write each run's output to a log file, with every line carrying that run's audit identifier and a timestamp, so lines remain attributable when multiple runs happen in parallel.
- **FR-036**: The system MUST allow a past run to be located by its audit identifier and its full log retrieved.
- **FR-037**: The system MUST NOT commit run logs to version control.

#### Access and safety

- **FR-038**: The system MUST remain reachable only from the machine it runs on, since it can now execute workspace actions with real credentials injected.
- **FR-039**: The system MUST require an explicit user action to start a run — no action may be triggered by merely loading or refreshing a page.

### Key Entities *(include if feature involves data)*

- **Action**: Something the user can launch with one click. Derived from what the workspace already defines — an individual job, or a multi-step pipeline. Has a name, a description, an availability state (with reason when unavailable), and whether it performs workspace work (and therefore requires validated directories) or is a read-only inspection action.
- **Run**: One execution of an Action. Has a unique audit identifier, the action it ran, the AI profile applied (if any), the data and configuration directories used, who/what started it, start time, end time, status (running / succeeded / failed / timed out / cancelled), and a reference to its log. For pipeline runs, also the per-step progress.
- **AI Profile**: A named set of AI provider credentials already configured in this workspace. Only its *name* is ever shown; its credential values are never displayed or logged. At most one is selected at a time, session-wide, acting as the default for runs that do not specify their own.
- **Workspace Directory Pair**: The data directory and the configuration directory a run operates against. Each is supplied by command option or environment variable, must pass structural validation before any work begins, and is recorded with the run.
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
- **SC-010**: 100% of attempts to run a job or pipeline with a missing, nonexistent, non-directory, empty, or structurally-wrong data or configuration directory are refused, with zero reads or writes performed against any directory in those cases.
- **SC-011**: Every refusal for a directory problem names the offending path and what was expected of it, so the operator can correct it without consulting documentation or source.
- **SC-012**: Read-only inspection actions remain 100% usable when the data and configuration directories are invalid or absent.
- **SC-013**: The AI profile is selectable in exactly one place in the interface, and the profile chosen there is the one recorded on 100% of runs started while it is selected — except for pipeline steps that declare their own, which use theirs.
- **SC-014**: 100% of run records identify the data and configuration directories that run operated against.

## Assumptions

- This capability extends the workspace-level dashboard that already lists packs, jobs and commands, rather than introducing a separate application — the user referred to "the jobs section", which already exists there. *(This means the dashboard stops being read-only, which was a deliberate earlier decision made on the grounds that a read-only page needs no auth and has no state to get stale; that decision is superseded by this feature and must be re-recorded, along with the new confirmation gate in FR-005 that partially compensates for it.)*
- The transactions editor UI delivered previously remains a separate, pack-level interface; unifying the two is out of scope here.
- The list of actions continues to be derived from the workspace's existing job manifests and pipeline definitions, so adding a new job or pipeline makes it appear in the UI with no separate registration step.
- AI profile selection is session-wide and acts as a **default**: it is applied to every run started while selected, but a pipeline step that names its own profile keeps using that one. The selection is not persisted between dashboard restarts.
- Requiring the data and configuration directories is a **breaking change to existing invocations**: commands that previously derived these paths from the workspace location will now refuse to run until the option or environment variable is supplied. Existing scheduled tasks, wrapper scripts, and habits that rely on the old implicit behaviour must be updated. This is accepted deliberately as the cost of never running against the wrong directory.
- The dashboard is used by a single operator on their own machine; no multi-user accounts, roles, or per-user permissions are required.
- Run history for completed runs is retained indefinitely; log files are kept on local disk and excluded from version control.
- "Progress" for an individual job means its live output plus its running/finished status; only multi-step pipelines have a more granular notion of progress (which step of how many).
- The machine running the dashboard must have the terminal-session tooling installed; this is a hard prerequisite with no fallback execution path (FR-027). *(Installed on 2026-07-27 — no longer a blocker.)*
- The expected substructure used to validate the directories (run-state and shared-config subdirectories for data; AI-profile or pack-config subdirectory for configuration) reflects how the workspace is laid out today. A legitimately fresh workspace that has not yet been initialised will therefore fail validation until those subdirectories exist — this is intended, since the alternative is silently running against an unprepared directory.
- Because every run is non-interactive (FR-025), the interactive-only capabilities of certain actions — retroactive suggestion walks and prompt-driven rule capture — are intentionally unreachable from the dashboard and stay terminal-only. This mirrors how those same features already behave on scheduled runs, so it introduces no new behavior, only a boundary on what the UI can do.

## Dependencies

- The workspace's existing job and pipeline definitions, and the existing shared execution path that runs them (so a run launched from the UI behaves the same as one launched from the terminal).
- The workspace's existing AI profile configuration.
- Terminal multiplexer tooling on the host machine, for the per-run isolated sessions (installed 2026-07-27).
- The existing run-history storage already used to record job and pipeline runs.
- A prepared workspace data directory and configuration directory, supplied explicitly. These are no longer inferred from where the workspace happens to sit on disk.
