#!/usr/bin/env bash
# Reference job: append a timestamped greeting into the shared SQLite store.
# `auto run` sets AUTO_DATA_DIR to the repo's data/ directory.
set -euo pipefail

DATA_DIR="${AUTO_DATA_DIR:-$(cd "$(dirname "$0")/../../.." && pwd)/data}"
DB="$DATA_DIR/state/hello.sqlite"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 not installed — skipping DB write (demo still succeeds)"
  echo "hello from hello-report at $(date -u +%FT%TZ)"
  exit 0
fi

sqlite3 "$DB" "CREATE TABLE IF NOT EXISTS hello(ts TEXT, host TEXT);"
sqlite3 "$DB" "INSERT INTO hello VALUES (strftime('%Y-%m-%dT%H:%M:%SZ','now'), '$(hostname)');"
echo "hello-report: wrote a row to $DB"
