package config

import "testing"

func TestResolveAccount_UsesSpecificThenDefault(t *testing.T) {
	cfg := &Config{
		DefaultPaymentType: "debit_card",
		Accounts: map[string]AccountRule{
			"3690": {AccountID: "acc1", PaymentType: "credit_card"},
		},
		DefaultAccount: AccountRule{AccountID: "acc_default"},
	}

	id, pt, ok := cfg.ResolveAccount("3690")
	if !ok || id != "acc1" || pt != "credit_card" {
		t.Fatalf("expected specific mapping, got ok=%v id=%q pt=%q", ok, id, pt)
	}

	id, pt, ok = cfg.ResolveAccount("unknown")
	if !ok || id != "acc_default" || pt != "debit_card" {
		t.Fatalf("expected default mapping with default payment type, got ok=%v id=%q pt=%q", ok, id, pt)
	}
}

func TestResolveAccount_SkipWhenNoMapping(t *testing.T) {
	cfg := &Config{DefaultPaymentType: "debit_card", Accounts: map[string]AccountRule{}, DefaultAccount: AccountRule{AccountID: ""}}
	_, _, ok := cfg.ResolveAccount("3690")
	if ok {
		t.Fatalf("expected ok=false")
	}
}
