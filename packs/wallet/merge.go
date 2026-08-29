// merge.go implements the id-based merge that incremental fetch (ADR 0021)
// uses to combine newly-fetched records into the existing local mirror.
package main

import (
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

// mergeRecords combines existing with incoming, keyed by each record's
// "id" field: an id already in existing is replaced by incoming's copy
// (the fresher version — incoming was just fetched); an id only in
// incoming is appended. Records missing an "id" field (shouldn't happen in
// practice) are dropped rather than risking silent duplication. Existing
// order is preserved for ids that survive; genuinely new ids are appended
// at the end.
func mergeRecords(existing, incoming []wallet.Record) []wallet.Record {
	byID := make(map[string]wallet.Record, len(existing)+len(incoming))
	order := make([]string, 0, len(existing)+len(incoming))

	addOrReplace := func(r wallet.Record) {
		id, _ := r["id"].(string)
		if id == "" {
			return
		}
		if _, seen := byID[id]; !seen {
			order = append(order, id)
		}
		byID[id] = r
	}
	for _, r := range existing {
		addOrReplace(r)
	}
	for _, r := range incoming {
		addOrReplace(r)
	}

	merged := make([]wallet.Record, 0, len(order))
	for _, id := range order {
		merged = append(merged, byID[id])
	}
	return merged
}

// maxUpdatedAt returns the latest "updatedAt" timestamp among records,
// parsed as RFC3339 (the format the Wallet API sends). ok is false if
// records is empty or none of them has a parseable updatedAt — the caller
// then has no basis for an incremental cursor and should fall back to a
// full fetch.
func maxUpdatedAt(records []wallet.Record) (t time.Time, ok bool) {
	for _, r := range records {
		s, isStr := r["updatedAt"].(string)
		if !isStr {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, s)
		if err != nil {
			continue
		}
		if !ok || parsed.After(t) {
			t = parsed
			ok = true
		}
	}
	return t, ok
}
