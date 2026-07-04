# Workspace pack config (git-ignored)

Real config values and secret files for each pack live here, one folder per
pack: `config/<pack>/`. **Nothing in here except this README is versioned** — it
holds secrets and machine-local overrides.

How it works (see `docs/adr/0007`):

1. A pack ships `config.sample.yaml` at its root, declaring the `env:` keys it
   reads and the secret `files:` it needs.
2. You create `config/<pack>/config.yaml` (real env values) and drop the secret
   files into `config/<pack>/`. `auto config init <pack>` scaffolds this.
3. `auto run <job>` injects the env into the job's process and symlinks the
   declared files into the job's workdir — so the pack is configured *before* it
   is called, from the workspace, without secrets touching either git repo.

Check status any time: `auto config <pack>`.
