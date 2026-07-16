// Package csvtxn reads the gmail pack's transactions.csv read-only. The
// expenses pack never writes to this file — gmail owns its schema and is its
// only writer (see docs/adr/0011-expenses-pack.md, decision 2). Columns are
// looked up by header name, not position, so this reader tolerates gmail
// adding columns (e.g. the Category/SubCategory/Labels enrichment from ADR
// 0010) without needing a matching change here.
package csvtxn

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// Txn is one transaction row, carrying only the fields the event matcher
// needs. MessageID is the stable join key used everywhere else in this pack
// (the assignment ledger, the AI request/response).
type Txn struct {
	MessageID string
	TxnDate   string
	Type      string
	Amount    string
	Merchant  string
	Info      string
	Subject   string
	BankFrom  string
	Category  string
	SubCategory string
}

// Skip records a row that could not be read.
type Skip struct {
	Line   int
	Reason string
}

// Read parses the CSV at path. Rows without a MessageID are skipped rather
// than aborting the whole read — the file may contain legacy short rows.
func Read(path string) (txns []Txn, skipped []Skip, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows (older rows may have fewer columns)

	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	col := indexColumns(header)
	if _, ok := col["MessageID"]; !ok {
		return nil, nil, fmt.Errorf("csv missing required column %q", "MessageID")
	}

	line := 1
	for {
		rec, rerr := r.Read()
		if rerr == io.EOF {
			break
		}
		line++
		if rerr != nil {
			skipped = append(skipped, Skip{Line: line, Reason: "parse error: " + rerr.Error()})
			continue
		}
		get := func(name string) string {
			i, ok := col[name]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}

		msgID := get("MessageID")
		if msgID == "" {
			skipped = append(skipped, Skip{Line: line, Reason: "empty MessageID"})
			continue
		}

		txns = append(txns, Txn{
			MessageID:   msgID,
			TxnDate:     get("TxnDate"),
			Type:        get("Type"),
			Amount:      get("Amount"),
			Merchant:    get("Merchant"),
			Info:        get("Info"),
			Subject:     get("Subject"),
			BankFrom:    get("BankFrom"),
			Category:    get("Category"),
			SubCategory: get("SubCategory"),
		})
	}
	return txns, skipped, nil
}

func indexColumns(header []string) map[string]int {
	m := map[string]int{}
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}
