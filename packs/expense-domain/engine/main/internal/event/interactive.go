package event

import "os"

// isInteractive reports whether stdin is a real terminal (spec 003,
// research.md Decision 6) — the signal used to gate the retroactive-
// suggestion flow (Story 5) and the rule-capture prompt (Story 6) so neither
// ever triggers on an unattended/cron run, regardless of flag configuration
// (FR-017). Pure stdlib: no new dependency. Independent copy of the gmail
// pack's identical helper (spec 002 Decision 3 precedent — duplicated code,
// not a shared cross-repo import).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
