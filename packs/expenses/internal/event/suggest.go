// Story 5 (spec 003): after a comment-driven correction to an already-
// assigned transaction, offer to walk the user through other, older, similar
// transactions for individual approval — never applied automatically. Only
// runs on an interactive run with the explicit --suggest-similar opt-in
// (FR-013/FR-014). Independent copy of the gmail pack's identical shape
// (spec 002 Decision 3 precedent).
package event

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/sumitasok/sa.automation.expenses/internal/csvtxn"
)

// correction is a comment-driven correction produced this run to a
// transaction that was already assigned before this run — the seed for a
// Story 5 suggestion session. PriorSource is the corrected transaction's OWN
// Source value before this run (used to find other transactions the same
// rule used to decide); NewSource is its resulting Source after this run
// (recorded on any approved candidate as "suggested:<NewSource>",
// data-model.md Decision 10). EventID is the new outcome; "" means the
// correction landed on "no event."
type correction struct {
	ID          string
	Merchant    string
	PriorSource string
	NewSource   string
	EventID     string
}

// candidate pairs a resembling, already-assigned transaction with its
// current assignment, for display and ordering.
type candidate struct {
	Txn   csvtxn.Txn
	Entry AssignmentEntry
}

// suggestCandidates returns other already-assigned transactions resembling
// corr — same merchant (case-insensitive) and/or previously decided by the
// same rule corr used to be decided by — excluding corr's own transaction
// and any transaction that already holds the identical outcome. Oldest
// TxnDate first (research.md Decision 8).
func suggestCandidates(txns []csvtxn.Txn, assigned map[string]AssignmentEntry, corr correction) []candidate {
	var out []candidate
	for _, t := range txns {
		if t.MessageID == "" || t.MessageID == corr.ID {
			continue
		}
		entry, has := assigned[t.MessageID]
		if !has {
			continue
		}
		sameMerchant := strings.EqualFold(strings.TrimSpace(t.Merchant), strings.TrimSpace(corr.Merchant))
		sameRule := corr.PriorSource != "" && strings.HasPrefix(corr.PriorSource, "rule:") && entry.Source == corr.PriorSource
		if !sameMerchant && !sameRule {
			continue
		}
		if entry.EventID == corr.EventID {
			continue // already matches the proposed outcome — not a correction candidate
		}
		out = append(out, candidate{Txn: t, Entry: entry})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Txn.TxnDate < out[j].Txn.TxnDate })
	return out
}

// suggestSimilar presents each candidate resembling corr, one at a time, for
// explicit approve/skip (FR-015/FR-016). Approved transactions are written
// immediately, preserving their own comment-tracking field untouched — a
// suggestion never considers or alters a candidate's UserComment. Returns
// the number of transactions updated via approval.
func suggestSimilar(st *State, reg *Registry, txns []csvtxn.Txn, corr correction) int {
	assigned := st.Assigned
	candidates := suggestCandidates(txns, assigned, corr)
	if len(candidates) == 0 {
		return 0
	}

	proposedLabel := "no event"
	if corr.EventID != "" {
		if e, ok := reg.Find(corr.EventID); ok {
			proposedLabel = e.Name
		} else {
			proposedLabel = corr.EventID
		}
	}
	fmt.Printf("\n[suggest] %d transaction(s) resemble the correction just made to %s (-> %s):\n",
		len(candidates), corr.ID, proposedLabel)

	scanner := bufio.NewScanner(os.Stdin)
	approved := 0
	for _, c := range candidates {
		fmt.Printf("  %s  %s  Rs.%-10s  %-30s  current=%s  proposed=%s\n",
			c.Txn.MessageID, c.Txn.TxnDate, c.Txn.Amount, c.Txn.Merchant, c.Entry.EventID, proposedLabel)
		fmt.Print("  Approve this change? [y/N]: ")
		if !scanner.Scan() {
			break
		}
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Printf("  skipped %s\n", c.Txn.MessageID)
			continue
		}
		source := "suggested:" + corr.NewSource
		st.Mark(c.Txn.MessageID, corr.EventID, 1.0, source, c.Entry.Comment)
		if corr.EventID != "" {
			reg.Touch(corr.EventID, 1)
		}
		approved++
		fmt.Printf("  approved %s\n", c.Txn.MessageID)
	}
	if approved == 0 {
		log.Printf("[INFO] update-event: suggestion session for %s: no candidates approved", corr.ID)
	}
	return approved
}
