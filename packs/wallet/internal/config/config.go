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
	LabelName          string                 // label attached to every pushed record
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
func (c *Config) ResolveAccount(code string) (accountID, paymentType string, ok bool) {
	rule, found := c.Accounts[code]
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
