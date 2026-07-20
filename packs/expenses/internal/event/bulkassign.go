package event

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// BulkAssignConfig parameterises a bulk-assign run.
type BulkAssignConfig struct {
	AssignmentCSVPath string // path to CSV with MessageID,EventID columns
	TransactionCSVPath string // path to transactions.csv (to validate MessageIDs exist)
	RegistryPath      string // path to the event registry (to validate EventIDs exist)
	StatePath         string // path to the assignment ledger (to merge assignments)
	DryRun            bool   // print assignments without writing state
}

// BulkAssignResult summarises a bulk-assign run.
type BulkAssignResult struct {
	Total        int // total rows in assignment CSV
	Valid        int // rows successfully validated and imported
	InvalidEvent int // rows with unknown EventID
	InvalidTxn   int // rows with unknown MessageID
	Duplicates   int // rows with duplicate MessageID (in assignment CSV)
	Overwritten  int // existing assignments that were overwritten (manual overrides auto)
	Malformed    int // rows that failed to parse
}

// BulkAssign reads a CSV with MessageID,EventID columns and merges those
// assignments into the state ledger, validating that:
// - Every MessageID exists in transactions.csv
// - Every EventID exists in the registry
// Manual assignments override any existing auto-matched assignments.
func BulkAssign(cfg BulkAssignConfig) (BulkAssignResult, error) {
	var res BulkAssignResult

	// Load registry to validate EventIDs
	reg, err := LoadRegistry(cfg.RegistryPath)
	if err != nil {
		return res, fmt.Errorf("load registry: %w", err)
	}

	// Load existing state to merge with
	st, err := LoadState(cfg.StatePath)
	if err != nil {
		return res, fmt.Errorf("load state: %w", err)
	}

	// Read transactions.csv to validate MessageIDs
	validIDs, malformed, err := readTransactionIDs(cfg.TransactionCSVPath)
	if err != nil {
		return res, fmt.Errorf("read transactions: %w", err)
	}
	res.Malformed = len(malformed)
	for _, s := range malformed {
		log.Printf("[WARN] bulk-assign: skip txn line %d: %s", s.Line, s.Reason)
	}

	// Read the assignment CSV
	f, err := os.Open(cfg.AssignmentCSVPath)
	if err != nil {
		return res, fmt.Errorf("open assignment csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows

	header, err := r.Read()
	if err != nil {
		return res, fmt.Errorf("read assignment csv header: %w", err)
	}

	// Find column indices
	col := indexColumns(header)
	msgIDIdx, hasMsgID := col["MessageID"]
	eventIDIdx, hasEventID := col["EventID"]

	if !hasMsgID {
		return res, fmt.Errorf("assignment csv missing required column %q", "MessageID")
	}
	if !hasEventID {
		return res, fmt.Errorf("assignment csv missing required column %q", "EventID")
	}

	// Track seen MessageIDs to detect duplicates within this CSV
	seenInCSV := make(map[string]bool)

	line := 1
	for {
		rec, rerr := r.Read()
		if rerr == io.EOF {
			break
		}
		line++
		if rerr != nil {
			log.Printf("[WARN] bulk-assign: skip line %d: %s", line, rerr)
			res.Malformed++
			continue
		}

		msgID := ""
		if msgIDIdx < len(rec) {
			msgID = strings.TrimSpace(rec[msgIDIdx])
		}
		eventID := ""
		if eventIDIdx < len(rec) {
			eventID = strings.TrimSpace(rec[eventIDIdx])
		}

		res.Total++

		// Validate MessageID
		if msgID == "" {
			log.Printf("[WARN] bulk-assign: line %d: empty MessageID", line)
			res.InvalidTxn++
			continue
		}
		if !validIDs[msgID] {
			log.Printf("[WARN] bulk-assign: line %d: unknown MessageID %q", line, msgID)
			res.InvalidTxn++
			continue
		}

		// Check for duplicates within this CSV
		if seenInCSV[msgID] {
			log.Printf("[WARN] bulk-assign: line %d: duplicate MessageID %q in assignment CSV", line, msgID)
			res.Duplicates++
			continue
		}
		seenInCSV[msgID] = true

		// Validate EventID
		if eventID == "" {
			log.Printf("[WARN] bulk-assign: line %d: empty EventID for MessageID %q", line, msgID)
			res.InvalidEvent++
			continue
		}
		if _, found := reg.Find(eventID); !found {
			log.Printf("[WARN] bulk-assign: line %d: unknown EventID %q", line, eventID)
			res.InvalidEvent++
			continue
		}

		// Check if this is overwriting an existing assignment
		if st.Has(msgID) {
			res.Overwritten++
			log.Printf("  %s -> event %q (MANUAL, overwrites previous assignment)", msgID, eventID)
		} else {
			log.Printf("  %s -> event %q (MANUAL)", msgID, eventID)
		}

		if !cfg.DryRun {
			st.Mark(msgID, eventID, 1.0) // manual = full confidence
			reg.Touch(eventID, 1)        // increment transaction count for this event
		}
		res.Valid++
	}

	if cfg.DryRun {
		log.Printf("[INFO] bulk-assign: dry-run — %d valid assignment(s), %d invalid — nothing written",
			res.Valid, res.Total-res.Valid)
		return res, nil
	}

	if err := reg.Save(); err != nil {
		return res, fmt.Errorf("saving registry: %w", err)
	}
	if err := st.Save(); err != nil {
		return res, fmt.Errorf("saving state: %w", err)
	}

	log.Printf("[INFO] bulk-assign: imported %d valid assignment(s), overwrote %d existing",
		res.Valid, res.Overwritten)
	return res, nil
}

// readTransactionIDs reads transactions.csv and returns the set of valid
// MessageIDs to validate assignments against.
func readTransactionIDs(path string) (map[string]bool, []struct {
	Line   int
	Reason string
}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	col := indexColumns(header)
	msgIDIdx, ok := col["MessageID"]
	if !ok {
		return nil, nil, fmt.Errorf("csv missing required column %q", "MessageID")
	}

	ids := make(map[string]bool)
	var skipped []struct {
		Line   int
		Reason string
	}

	line := 1
	for {
		rec, rerr := r.Read()
		if rerr == io.EOF {
			break
		}
		line++
		if rerr != nil {
			skipped = append(skipped, struct {
				Line   int
				Reason string
			}{Line: line, Reason: "parse error: " + rerr.Error()})
			continue
		}

		msgID := ""
		if msgIDIdx < len(rec) {
			msgID = strings.TrimSpace(rec[msgIDIdx])
		}
		if msgID != "" {
			ids[msgID] = true
		} else {
			skipped = append(skipped, struct {
				Line   int
				Reason string
			}{Line: line, Reason: "empty MessageID"})
		}
	}

	return ids, skipped, nil
}

// indexColumns returns a map from column name to index.
func indexColumns(header []string) map[string]int {
	m := map[string]int{}
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}
