# Sharing: the framework + packs model

This explains how the system is split so you can share it with others without
sharing your private automations, and how a collaborator gets on board.

## The three repos

| repo | who sees it | contains |
|------|-------------|----------|
| **automation-framework** | public | the `auto` CLI, template, schema, docs — the *parent* |
| **automation-pack-shared** | your team | shared jobs (`visibility: shared`/`public`) |
| **your workspace** (this repo) | only you | `machines.yaml`, `data/`, `packs.yaml`, your `packs/private/` jobs |

Your workspace mounts the other two as **git submodules**. Access is controlled
per-repo by your git host — a private job in your workspace physically cannot
reach someone who only has the framework or the shared pack. `visibility` +
`auto doctor` is the second safety net (see `docs/adr/0002…`, `0003`).

```
your-workspace/               (private repo)
├── framework/                → submodule: automation-framework   (public)
├── packs/
│   ├── shared/               → submodule: automation-pack-shared (team)
│   └── private/              your jobs — never a submodule, never shared
├── packs.yaml   machines.yaml   data/
```

## One-time: split this workspace into the three repos

Right now everything lives in one folder for convenience. To turn it into the
real model, promote `framework/` and `packs/shared/` into their own repos and
re-mount them. `tools/split-into-repos.sh` prints/executes the commands; the gist:

```bash
# 1. framework → its own repo
cd framework && git init && git add -A && git commit -m "framework 0.2.0"
git remote add origin git@github.com:<you>/automation-framework.git && git push -u origin main

# 2. shared pack → its own repo
cd ../packs/shared && git init && git add -A && git commit -m "shared pack"
git remote add origin git@github.com:<you>/automation-pack-shared.git && git push -u origin main

# 3. in the workspace, replace the folders with submodules
cd ../../
rm -rf framework packs/shared
git submodule add git@github.com:<you>/automation-framework.git framework
git submodule add git@github.com:<you>/automation-pack-shared.git packs/shared
git add -A && git commit -m "mount framework + shared pack as submodules"
```

`packs/private/` stays a plain folder in the workspace — that's what keeps it
private.

## A collaborator onboards

```bash
# Their own private workspace, reusing your framework + shared pack:
git clone --recursive git@github.com:<them>/their-workspace.git
cd their-workspace && ./auto bootstrap        # inits submodules, git-lfs
./auto packs                                  # shows framework + shared mounted
./auto list                                   # every job they can see
./auto run hello-report                       # use a shared automation
```

To start from scratch, a collaborator creates a workspace with a `packs.yaml`
that mounts your public framework and (if invited) the shared pack, plus their
own `packs/private/`.

## Contributing a job back to the shared pack

```bash
./auto new                     # choose pack: shared
# ...write the job, set visibility: shared...
./auto doctor                  # must pass (blocks private leaks, dup ids)
cd packs/shared && git checkout -b add-<job> && git commit -am "add <job>" && git push
# open a PR against automation-pack-shared
```

## Keeping a job private

Put it in `packs/private/` (via `auto new` → pack `private`). It has
`visibility: private`, never becomes a submodule, and `auto doctor` refuses to
let it sit in a shared pack. If it later becomes useful to others, move the
folder into `packs/shared/jobs/` and change `visibility` to `shared`.

## Updating

- Pull framework/shared updates: `git submodule update --remote && ./auto doctor`.
- Publish your own shared contributions: PR to the shared-pack repo; teammates
  bump their submodule pointer when ready. Nobody is forced onto a bad commit.
