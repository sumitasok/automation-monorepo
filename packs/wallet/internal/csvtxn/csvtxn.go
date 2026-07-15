// Package csvtxn reads the gmail pack's transactions.csv and normalises each
// row into a Txn ready to become a Wallet record.
package csvtxn

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Txn is one normalised transaction from transactions.csv.
type Txn struct {
	MessageID string    // stable unique id (gmail message) — used for dedupe
	Date      time.Time // resolved transaction date (TxnDate, else EmailDate)
	DateOnly  bool      // true when only a calendar date was known (no time)
	Amount    float64   // always positive magnitude
	IsCredit  bool      // true = income (+), false = expense (-)
	Account   string    // CSV account code (bank last-4 / identifier)
	Merchant  string    // counter-party
	Info      string    // free-text detail
	Subject   string    // email subject
	BankFrom  string    // source bank/sender
}

// SignedAmount returns the amount with Wallet's sign convention:
// negative for expenses (Debit), positive for income (Credit).
func (t Txn) SignedAmount() float64 {
	if t.IsCredit {
		return t.Amount
	}
	return -t.Amount
}

// Read parses the CSV at path. It is tolerant: rows that cannot yield a usable
// amount or date are collected into skipped with a reason rather than aborting.
func Read(path string, loc *time.Location) (txns []Txn, skipped []Skip, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows

	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	col := indexColumns(header)
	required := []string{"MessageID", "TxnDate", "Type", "Amount", "Account"}
	for _, c := range required {
		if _, ok := col[c]; !ok {
			return nil, nil, fmt.Errorf("csv missing required column %q", c)
		}
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
		amt, aerr := parseAmount(get("Amount"))
		if aerr != nil {
			skipped = append(skipped, Skip{Line: line, MessageID: msgID, Reason: aerr.Error()})
			continue
		}
		date, dateOnly, derr := resolveDate(get("TxnDate"), get("EmailDate"), loc)
		if derr != nil {
			skipped = append(skipped, Skip{Line: line, MessageID: msgID, Reason: derr.Error()})
			continue
		}

		txns = append(txns, Txn{
			MessageID: msgID,
			Date:      date,
			DateOnly:  dateOnly,
			Amount:    amt,
			IsCredit:  strings.EqualFold(get("Type"), "Credit"),
			Account:   get("Account"),
			Merchant:  get("Merchant"),
			Info:      get("Info"),
			Subject:   get("Subject"),
			BankFrom:  get("BankFrom"),
		})
	}
	return txns, skipped, nil
}

// Skip records a row that could not be normalised.
type Skip struct {
	Line      int
	MessageID string
	Reason    string
}

func indexColumns(header []string) map[string]int {
	m := map[string]int{}
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

// parseAmount strips thousands separators and parses a positive magnitude.
func parseAmount(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	if v < 0 {
		v = -v
	}
	if v == 0 {
		return 0, fmt.Errorf("zero amount")
	}
	return v, nil
}

// dateLayouts covers the formats seen in TxnDate.
var dateLayouts = []struct {
	layout   string
	dateOnly bool
}{
	{"2006-01-02 15:04:05", false},
	{"2006-01-02T15:04:05", false},
	{"2006-01-02", true},
	{"Jan 2, 2006 03:04 PM", false},
	{"Jan 2, 2006 03:04 AM", false},
}

// emailLayouts covers EmailDate (RFC1123Z-ish, sometimes with a trailing
// "(IST)"/"(UTC)" comment which we strip before parsing).
var emailLayouts = []string{
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -0700",
}

func resolveDate(txnDate, emailDate string, loc *time.Location) (time.Time, bool, error) {
	if t, only, ok := parseTxnDate(txnDate, loc); ok {
		return t, only, nil
	}
	if t, ok := parseEmailDate(emailDate); ok {
		// EmailDate carries a real timestamp; keep it, not date-only.
		return t.In(loc), false, nil
	}
	return time.Time{}, false, fmt.Errorf("unparseable date (TxnDate=%q EmailDate=%q)", txnDate, emailDate)
}

func parseTxnDate(s string, loc *time.Location) (time.Time, bool, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false, false
	}
	for _, l := range dateLayouts {
		if t, err := time.ParseInLocation(l.layout, s, loc); err == nil {
			return t, l.dateOnly, true
		}
	}
	return time.Time{}, false, false
}

func parseEmailDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "("); i > 0 { // drop trailing "(IST)" / "(UTC)"
		s = strings.TrimSpace(s[:i])
	}
	for _, l := range emailLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
