// Package sync is the wallet-sync orchestration: read transactions.csv, map each
// row to a Wallet account, and create one Wallet record per transaction —
// processed day by day, deduped by MessageID, and tagged with a single label so
// everything this pack writes can be filtered later.
package sync

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/config"
	"github.com/sumitasok/sa.automation.wallet/internal/csvtxn"
	"github.com/sumitasok/sa.automation.wallet/internal/state"
	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

// Options controls a sync run.
type Options struct {
	CSVPath   string
	StatePath string
	DryRun    bool
	Since     time.Time // zero = no lower bound
	Until     time.Time // zero = no upper bound
	Limit     int       // 0 = no cap on records pushed
}

// WalletClient is the small slice of the Wallet REST client that the sync
// runner needs. Kept as an interface so the sync logic can be unit-tested
// without calling the real API.
type WalletClient interface {
	EnsureLabel(name string) (string, error)
	CreateRecords(records []wallet.NewRecord) ([]wallet.RecordResult, error)
}

// Runner holds resolved dependencies for a run.
type Runner struct {
	Cfg    *config.Config
	Client WalletClient // nil in dry-run
	Loc    *time.Location
	Out    func(string, ...any) // logger, e.g. log.Printf
}

// Result summarises a run.
type Result struct {
	Total      int
	Created    int
	Skipped    int // already pushed
	Unmapped   int // no account mapping
	OutOfRange int
	Failed     int
	Malformed  int // rows that failed CSV normalisation
}

const batchSize = 20

// item pairs a source transaction with its built Wallet record.
type item struct {
	txn csvtxn.Txn
	rec wallet.NewRecord
}

// Run executes the sync and returns a Result.
func (r *Runner) Run(o Options) (Result, error) {
	var res Result

	txns, malformed, err := csvtxn.Read(o.CSVPath, r.Loc)
	if err != nil {
		return res, err
	}
	res.Malformed = len(malformed)
	res.Total = len(txns)
	for _, s := range malformed {
		r.Out("skip (malformed) line %d %s: %s", s.Line, short(s.MessageID), s.Reason)
	}

	st, err := state.Load(o.StatePath)
	if err != nil {
		return res, fmt.Errorf("load state: %w", err)
	}

	// Without an account map, every row will be skipped as unmapped. Fail fast
	// with an actionable message instead of streaming hundreds of skips.
	if !hasAnyAccountMapping(r.Cfg) {
		return res, fmt.Errorf("no Wallet account mappings found: create config/wallet/accounts.json (copy packs/wallet/accounts.sample.json) and fill in Wallet account UUIDs")
	}

	// Resolve the label once (real runs only).
	var labelIDs []string
	if !o.DryRun {
		id, err := r.Client.EnsureLabel(r.Cfg.LabelName)
		if err != nil {
			return res, fmt.Errorf("ensure label %q: %w", r.Cfg.LabelName, err)
		}
		labelIDs = []string{id}
		r.Out("using label %q (%s)", r.Cfg.LabelName, id)
	}

	// Group eligible transactions by calendar day.
	byDay := map[string][]item{}
	var days []string

	for _, t := range txns {
		if st.Has(t.MessageID) {
			res.Skipped++
			continue
		}
		if !o.Since.IsZero() && t.Date.Before(o.Since) {
			res.OutOfRange++
			continue
		}
		if !o.Until.IsZero() && t.Date.After(o.Until) {
			res.OutOfRange++
			continue
		}
		accountID, paymentType, ok := r.Cfg.ResolveAccount(t.Account)
		if !ok {
			res.Unmapped++
			r.Out("skip (unmapped account %q) %s %s %.2f", t.Account, short(t.MessageID), t.Merchant, t.Amount)
			continue
		}
		rec := wallet.NewRecord{
			AccountID:    accountID,
			Amount:       round2(t.SignedAmount()),
			RecordDate:   t.Date.Format(time.RFC3339),
			PaymentType:  paymentType,
			LabelIDs:     labelIDs,
			Note:         buildNote(t),
			CounterParty: clip(t.Merchant, 255),
		}
		day := t.Date.Format("2006-01-02")
		if _, seen := byDay[day]; !seen {
			days = append(days, day)
		}
		byDay[day] = append(byDay[day], item{txn: t, rec: rec})
	}
	sort.Strings(days)

	// Push, day by day, in batches — respecting an optional overall cap.
	for _, day := range days {
		items := byDay[day]
		if o.DryRun {
			for _, it := range items {
				if o.Limit > 0 && res.Created >= o.Limit {
					break
				}
				r.Out("DRY %s %+10.2f %-14s %s", day, it.rec.Amount, it.rec.PaymentType, it.txn.Merchant)
				res.Created++
			}
			continue
		}
		for start := 0; start < len(items); start += batchSize {
			end := start + batchSize
			if end > len(items) {
				end = len(items)
			}
			chunk := items[start:end]
			if o.Limit > 0 {
				room := o.Limit - res.Created
				if room <= 0 {
					break
				}
				if len(chunk) > room {
					chunk = chunk[:room]
				}
			}
			recs := make([]wallet.NewRecord, len(chunk))
			for i, it := range chunk {
				recs[i] = it.rec
			}
			results, err := r.Client.CreateRecords(recs)
			if err != nil && len(results) == 0 {
				return res, fmt.Errorf("create records for %s: %w", day, err)
			}
			applyResults(chunk, results, st, &res, r.Out, day)
			if err := st.Save(); err != nil {
				return res, fmt.Errorf("save state: %w", err)
			}
		}
		if o.Limit > 0 && res.Created >= o.Limit {
			break
		}
	}

	if o.DryRun {
		r.Out("DRY-RUN: no records created")
	}
	return res, nil
}

// applyResults marks successes in state and counts failures. When the API
// returns no per-item results (plain 200), all items in the chunk are treated
// as created.
func applyResults(chunk []item, results []wallet.RecordResult, st *state.State, res *Result, out func(string, ...any), day string) {
	if len(results) == 0 {
		for _, it := range chunk {
			st.Mark(it.txn.MessageID, "", day)
			res.Created++
		}
		return
	}
	for _, rr := range results {
		if rr.InputIndex < 0 || rr.InputIndex >= len(chunk) {
			continue
		}
		it := chunk[rr.InputIndex]
		if rr.Success {
			st.Mark(it.txn.MessageID, rr.ID, day)
			res.Created++
		} else {
			res.Failed++
			out("fail %s %s: %s (%s)", day, short(it.txn.MessageID), rr.Error, rr.ErrorType)
		}
	}
}

func hasAnyAccountMapping(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.DefaultAccount.AccountID != "" {
		return true
	}
	for _, r := range cfg.Accounts {
		if r.AccountID != "" {
			return true
		}
	}
	return false
}

func buildNote(t csvtxn.Txn) string {
	parts := []string{}
	if t.Info != "" {
		parts = append(parts, t.Info)
	} else if t.Subject != "" {
		parts = append(parts, t.Subject)
	}
	parts = append(parts, "[gmail-csv "+short(t.MessageID)+"]")
	return clip(strings.Join(parts, " "), 255)
}

// short returns the trailing id segment of a gmail MessageID for readable logs.
func short(msgID string) string {
	if i := strings.LastIndex(msgID, ":"); i >= 0 && i+1 < len(msgID) {
		return msgID[i+1:]
	}
	return msgID
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func round2(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*100+0.5)) / 100
	}
	return float64(int64(v*100+0.5)) / 100
}
