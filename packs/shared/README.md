# Shared pack

Team-shared automation library. Everyone with access to this repo can run these
jobs and contribute their own.

**Use a shared job:** it shows up in `auto list` automatically once this pack is
mounted; run it with `auto run <id>`.

**Contribute a job:** `auto new` → choose pack `shared`, or move an existing job
folder into `jobs/` here. Set `visibility: shared` (or `public`). Open a pull
request against this repo. `auto doctor` blocks any job marked `visibility:
private` from living here, so nothing personal leaks in.

Jobs must be self-contained or rely only on code in this pack's `lib/`, so they
work for anyone who mounts the pack.
