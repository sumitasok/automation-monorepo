// Package config loads the wallet pack configuration from the environment and
// an accounts map file. It follows the monorepo pack-config-injection contract
// (ADR 0007): secrets/settings arrive as env vars, and file-based config is
// discovered via AUTO_PACK_CONFIG_DIR (with local fallbacks for dev).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AccountRule maps a CSV account code (bank last-4 / identifier) to a Wallet
// account and, optionally, a payment type override for records on it.
type AccountRule struct {
	AccountID   string `json:"accountId"`
	PaymentType string `json:"paymentType,omitempty"`
}

// Config is the fully-resolved runtime configuration.
type Config struct {
	BaseURL            string                 // Wallet REST base URL
	APIToken           string                 // Bearer token (secret)
	LabelName          string                 // label attached to every pushed record (display/log only once LabelID is set)
	LabelID            string                 // the label's Wallet UUID; the REST API has no /labels endpoint, so this must be supplied directly (see RUNBOOK)
	DefaultPaymentType string                 // used when an account rule has none
	Timezone           string                 // used to interpret date-only TxnDate values
	Accounts           map[string]AccountRule // CSV account code -> rule
	// DefaultAccount is used for rows whose account code is empty or unmapped.
	// Key "_default" inside the accounts map. Empty AccountID means "skip".
	DefaultAccount AccountRule
}

const (
	defaultBaseURL     = "https://rest.budgetbakers.com/wallet"
	defaultLabel       = "source:automation-monorepo"
	defaultPaymentType = "debit_card"
	defaultTimezone    = "Asia/Kolkata"
)

// Load resolves configuration. accountsPath may be empty, in which case the
// accounts file is looked up in AUTO_PACK_CONFIG_DIR then the working dir.
// requireToken lets callers (e.g. --dry-run) load config without a token.
func Load(accountsPath string, requireToken bool) (*Config, error) {
	c := &Config{
		BaseURL:            envOr("WALLET_BASE_URL", defaultBaseURL),
		APIToken:           os.Getenv("WALLET_API_TOKEN"),
		LabelName:          envOr("WALLET_LABEL", defaultLabel),
		LabelID:            os.Getenv("WALLET_LABEL_ID"),
		DefaultPaymentType: envOr("WALLET_DEFAULT_PAYMENT_TYPE", defaultPaymentType),
		Timezone:           envOr("WALLET_TIMEZONE", defaultTimezone),
		Accounts:           map[string]AccountRule{},
	}

	if requireToken && c.APIToken == "" {
		return nil, fmt.Errorf("WALLET_API_TOKEN is not set (get one at https://web.budgetbakers.com/settings/rest-api, Premium plan)")
	}

	path, err := resolveAccountsPath(accountsPath)
	if err != nil {
		return nil, err
	}
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read accounts map %s: %w", path, err)
		}
		var m map[string]AccountRule
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("parse accounts map %s: %w", path, err)
		}
		for code, rule := range m {
			if code == "_default" {
				c.DefaultAccount = rule
				continue
			}
			c.Accounts[code] = rule
		}
	}

	return c, nil
}

// ResolveAccount returns the Wallet account ID and payment type for a CSV
// account code, applying the default rule and default payment type. ok is
// false when the row should be skipped (no mapping and no default account).
//
// Bank alert emails mask account numbers inconsistently (e.g. "0878",
// "X0878", and "XXXXX860878" all refer to the same card), so an exact-key
// miss falls back to matching by trailing digits (last 4, then last 3)
// against every mapped code before giving up to DefaultAccount. A match is
// only accepted when exactly one mapped code shares that suffix — an
// ambiguous suffix (shared by two+ mapped codes) is treated as no match
// rather than guessed.
func (c *Config) ResolveAccount(code string) (accountID, paymentType string, ok bool) {
	rule, found := c.Accounts[code]
	if !found || rule.AccountID == "" {
		if key, matched := c.fuzzyMatchAccount(code); matched {
			if r, ok2 := c.Accounts[key]; ok2 && r.AccountID != "" {
				rule, found = r, true
			}
		}
	}
	if !found || rule.AccountID == "" {
		rule = c.DefaultAccount
	}
	if rule.AccountID == "" {
		return "", "", false
	}
	pt := rule.PaymentType
	if pt == "" {
		pt = c.DefaultPaymentType
	}
	return rule.AccountID, pt, true
}

// fuzzyMatchAccount finds the mapped account code sharing code's trailing
// digits, trying a 4-digit suffix first, then a 3-digit suffix. matched is
// false when no digits are found, or when a suffix is shared by more than
// one mapped code (ambiguous — never guessed).
func (c *Config) fuzzyMatchAccount(code string) (key string, matched bool) {
	for _, n := range []int{4, 3} {
		codeSuffix, ok := lastDigits(code, n)
		if !ok {
			continue
		}
		var candidates []string
		for k := range c.Accounts {
			if keySuffix, ok := lastDigits(k, n); ok && keySuffix == codeSuffix {
				candidates = append(candidates, k)
			}
		}
		if len(candidates) == 1 {
			return candidates[0], true
		}
		// 0 matches: fall through to the shorter suffix length. 2+ matches:
		// ambiguous at this length — a shorter suffix would only be MORE
		// ambiguous, so give up entirely rather than trying length 3 too.
		if len(candidates) > 1 {
			return "", false
		}
	}
	return "", false
}

// lastDigits returns the last n digit characters found in s (scanning from
// the end, ignoring non-digit characters), in their original order. ok is
// false when s contains fewer than n digit characters.
func lastDigits(s string, n int) (out string, ok bool) {
	digits := make([]byte, 0, n)
	for i := len(s) - 1; i >= 0 && len(digits) < n; i-- {
		if s[i] >= '0' && s[i] <= '9' {
			digits = append(digits, s[i])
		}
	}
	if len(digits) < n {
		return "", false
	}
	// digits was collected back-to-front; reverse it in place.
	for l, r := 0, len(digits)-1; l < r; l, r = l+1, r-1 {
		digits[l], digits[r] = digits[r], digits[l]
	}
	return string(digits), true
}

func resolveAccountsPath(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("accounts map not found at %s: %w", explicit, err)
		}
		return explicit, nil
	}
	candidates := []string{}
	if dir := os.Getenv("AUTO_PACK_CONFIG_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "accounts.json"))
	}
	candidates = append(candidates, "accounts.json")
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	// No accounts file: allowed (every row falls back to DefaultAccount, which
	// is empty -> rows are skipped). Callers warn on this.
	return "", nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
