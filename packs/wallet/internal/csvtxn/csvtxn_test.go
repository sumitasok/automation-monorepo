package csvtxn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRead_NormalisesAmountAndDate(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "transactions.csv")
	csv := strings.Join([]string{
		"MessageID,EmailDate,TxnDate,Type,Amount,Account,Merchant,Info,Subject,BankFrom",
		"m1,\"Mon, 01 Jul 2026 10:00:00 +0530\",2026-07-01,Debit,\"1,500.00\",3690,Shop,info,subj,bank",
		"m2,\"Mon, 01 Jul 2026 10:00:00 +0530\",2026-07-01,Credit,-25.5,3690,Pay,info,subj,bank",
		"m3,\"Mon, 01 Jul 2026 10:00:00 +0530\",,Debit,100,3690,Shop,info,subj,bank",
		"m_bad,\"Mon, 01 Jul 2026 10:00:00 +0530\",2026-07-01,Debit,,3690,Shop,info,subj,bank",
	}, "\n") + "\n"
	if err := os.WriteFile(p, []byte(csv), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Kolkata")
	txns, skipped, err := Read(p, loc)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(skipped))
	}
	if len(txns) != 3 {
		t.Fatalf("expected 3 txns, got %d", len(txns))
	}

	if txns[0].Amount != 1500 {
		t.Fatalf("expected amount 1500, got %v", txns[0].Amount)
	}
	if txns[0].SignedAmount() >= 0 {
		t.Fatalf("expected debit to be negative signed amount, got %v", txns[0].SignedAmount())
	}
	if !txns[1].IsCredit || txns[1].SignedAmount() <= 0 {
		t.Fatalf("expected credit to be positive signed amount, got isCredit=%v signed=%v", txns[1].IsCredit, txns[1].SignedAmount())
	}
	if txns[2].Date.IsZero() {
		t.Fatalf("expected fallback to EmailDate, got zero")
	}
}

func TestResolveDate_DateOnlyUsesLocation(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	tm, only, err := resolveDate("2026-07-01", "Mon, 01 Jul 2026 01:02:03 +0000", loc)
	if err != nil {
		t.Fatalf("resolveDate: %v", err)
	}
	if !only {
		t.Fatalf("expected date-only")
	}
	// ParseInLocation should yield midnight in that location.
	if tm.Hour() != 0 || tm.Minute() != 0 {
		t.Fatalf("expected midnight time, got %s", tm.Format(time.RFC3339))
	}
	if tm.Location().String() != loc.String() {
		t.Fatalf("expected location %s, got %s", loc, tm.Location())
	}
}
