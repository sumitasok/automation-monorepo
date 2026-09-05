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

// Bank alert emails mask the same account inconsistently across banks (real
// examples seen in transactions.csv: "0878" / "X0878" / "XXXXX860878" are all
// the same card) — an inexact code should still resolve via a shared 4-digit
// trailing suffix.
func TestResolveAccount_FuzzyMatchesByLast4Digits(t *testing.T) {
	cfg := &Config{
		DefaultPaymentType: "debit_card",
		Accounts: map[string]AccountRule{
			"0878": {AccountID: "acc_0878", PaymentType: "debit_card"},
		},
	}

	for _, code := range []string{"0878", "X0878", "XXXXX860878"} {
		id, _, ok := cfg.ResolveAccount(code)
		if !ok || id != "acc_0878" {
			t.Fatalf("code %q: expected fuzzy match to acc_0878, got ok=%v id=%q", code, ok, id)
		}
	}
}

// A 3-digit-only mapped code (e.g. "XXX383", which carries just 3 digit
// characters) should still match a differently-masked code sharing those 3
// trailing digits, since a 4-digit suffix can't be formed for either side.
func TestResolveAccount_FuzzyMatchesByLast3DigitsWhenNo4Available(t *testing.T) {
	cfg := &Config{
		DefaultPaymentType: "debit_card",
		Accounts: map[string]AccountRule{
			"XXX383": {AccountID: "acc_383", PaymentType: "debit_card"},
		},
	}
	id, _, ok := cfg.ResolveAccount("ICICI SB X383")
	if !ok || id != "acc_383" {
		t.Fatalf("expected fuzzy 3-digit match to acc_383, got ok=%v id=%q", ok, id)
	}
}

// A code whose trailing digits don't match any mapped code (e.g. an
// unrelated account, or a phone-number-looking value) must fall through to
// DefaultAccount rather than being guessed onto an unrelated mapping.
func TestResolveAccount_FuzzyMatchFallsThroughWhenNoSuffixShared(t *testing.T) {
	cfg := &Config{
		DefaultPaymentType: "debit_card",
		Accounts: map[string]AccountRule{
			"0878": {AccountID: "acc_0878", PaymentType: "debit_card"},
		},
		DefaultAccount: AccountRule{AccountID: "acc_default"},
	}
	id, _, ok := cfg.ResolveAccount("XX1983")
	if !ok || id != "acc_default" {
		t.Fatalf("expected fallthrough to default, got ok=%v id=%q", ok, id)
	}
}

// Two mapped codes sharing the same trailing digits must NOT be guessed —
// ambiguous suffixes fall through to DefaultAccount instead of picking one.
func TestResolveAccount_FuzzyMatchAmbiguousFallsThrough(t *testing.T) {
	cfg := &Config{
		DefaultPaymentType: "debit_card",
		Accounts: map[string]AccountRule{
			"10878": {AccountID: "acc_a", PaymentType: "debit_card"},
			"X0878": {AccountID: "acc_b", PaymentType: "debit_card"},
		},
		DefaultAccount: AccountRule{AccountID: "acc_default"},
	}
	id, _, ok := cfg.ResolveAccount("XXXXX860878")
	if !ok || id != "acc_default" {
		t.Fatalf("expected ambiguous suffix to fall through to default, got ok=%v id=%q", ok, id)
	}
}
