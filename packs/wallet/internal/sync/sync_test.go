package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/config"
	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

type fakeClient struct {
	labelName string
	records   []wallet.NewRecord
}

func (f *fakeClient) EnsureLabel(name string) (string, error) {
	f.labelName = name
	return "lab_1", nil
}

func (f *fakeClient) CreateRecords(records []wallet.NewRecord) ([]wallet.RecordResult, error) {
	f.records = append(f.records, records...)
	return nil, nil // treat as plain 200: applyResults() will mark all as created
}

func TestRunner_DryRun_UnmappedSkippedAndStateNotWritten(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "transactions.csv")
	statePath := filepath.Join(dir, "state.json")

	// Minimal CSV: required columns plus a couple optional ones.
	csv := strings.Join([]string{
		"MessageID,EmailDate,TxnDate,Type,Amount,Account,Merchant,Info,Subject,BankFrom",
		"m1,\"Mon, 01 Jul 2026 10:00:00 +0530\",2026-07-01,Debit,100.00,UNMAPPED,Shop,hello,subj,bank",
	}, "\n") + "\n"
	if err := os.WriteFile(csvPath, []byte(csv), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	cfg := &config.Config{Accounts: map[string]config.AccountRule{}, DefaultAccount: config.AccountRule{}} // no mappings
	loc, _ := time.LoadLocation("Asia/Kolkata")

	r := &Runner{Cfg: cfg, Loc: loc, Out: func(string, ...any) {}}
	res, err := r.Run(Options{CSVPath: csvPath, StatePath: statePath, DryRun: true})
	if err == nil {
		t.Fatalf("expected error due to missing account mappings")
	}
	if res.Created != 0 {
		t.Fatalf("expected 0 created, got %d", res.Created)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("expected no state file written, got stat err=%v", err)
	}
}

func TestRunner_RealRun_CreatesAndWritesState(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "transactions.csv")
	statePath := filepath.Join(dir, "state.json")

	csv := strings.Join([]string{
		"MessageID,EmailDate,TxnDate,Type,Amount,Account,Merchant,Info,Subject,BankFrom",
		"m1,\"Mon, 01 Jul 2026 10:00:00 +0530\",2026-07-01,Debit,100.00,3690,Shop,hello,subj,bank",
	}, "\n") + "\n"
	if err := os.WriteFile(csvPath, []byte(csv), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	cfg := &config.Config{
		LabelName:          "source:automation-monorepo",
		DefaultPaymentType: "debit_card",
		Accounts: map[string]config.AccountRule{
			"3690": {AccountID: "acc_1", PaymentType: "debit_card"},
		},
	}
	loc, _ := time.LoadLocation("Asia/Kolkata")
	fc := &fakeClient{}

	r := &Runner{Cfg: cfg, Client: fc, Loc: loc, Out: func(string, ...any) {}}
	res, err := r.Run(Options{CSVPath: csvPath, StatePath: statePath, DryRun: false})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("expected 1 created, got %d", res.Created)
	}
	if fc.labelName != cfg.LabelName {
		t.Fatalf("expected label %q, got %q", cfg.LabelName, fc.labelName)
	}
	if len(fc.records) != 1 {
		t.Fatalf("expected 1 record posted, got %d", len(fc.records))
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file written, stat: %v", err)
	}
	b, _ := os.ReadFile(statePath)
	if !strings.Contains(string(b), "\"m1\"") {
		t.Fatalf("expected state to contain m1, got: %s", string(b))
	}
}
