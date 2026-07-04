# Daily hello report

**What:** appends a timestamped greeting to `data/state/hello.sqlite`.

**Why it exists:** the reference job — copy its shape when creating real jobs.

**Run by hand:** `./tools/auto run hello-report`

**Gotchas:** writes to shared SQLite, so per the plan it should ultimately run
on the single `data-writer` machine. Falls back gracefully if `sqlite3` is
missing.
