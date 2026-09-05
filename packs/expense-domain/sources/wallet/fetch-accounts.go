package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/config"
	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

// CachedAccount represents a Wallet account in the cache.
type CachedAccount struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrencyCode string `json:"currencyCode"`
	Archived     bool   `json:"archived"`
	LastDigits   string `json:"lastDigits"`   // last 4 chars of account identifier
	FetchedAt    string `json:"fetchedAt"`
}

// AccountCache is the structure of accounts-cache.json
type AccountCache struct {
	FetchedAt string           `json:"fetchedAt"`
	Count     int              `json:"count"`
	Accounts  []CachedAccount  `json:"accounts"`
	ByID      map[string]int   `json:"_byId"`      // ID → index in Accounts
	ByName    map[string][]int `json:"_byName"`    // name substring → indices
	ByLastFour map[string][]int `json:"_byLastFour"` // last 4 digits → indices
}

func runFetchAccounts(args []string) error {
	fs := flag.NewFlagSet("fetch-accounts", flag.ExitOnError)
	cacheFile := fs.String("cache", "", "path to save accounts cache (default: $AUTO_DATA_DIR/wallet/accounts-cache.json)")
	fs.Parse(args)

	// Resolve cache path
	if *cacheFile == "" {
		*cacheFile = resolveDataPath("wallet/accounts-cache.json", "accounts-cache.json")
	}

	fmt.Printf("Working directory: %s\n", filepath.Dir(*cacheFile))
	fmt.Printf("Cache will be saved to: %s\n\n", *cacheFile)

	// Load config to get Wallet API token
	cfg, err := config.Load("", true)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Fetch accounts from Wallet API
	client := wallet.New(cfg.BaseURL, cfg.APIToken)
	accs, err := client.GetAccounts()
	if err != nil {
		return fmt.Errorf("fetch accounts from Wallet API: %w", err)
	}

	// Build cache
	cache := AccountCache{
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Accounts:  []CachedAccount{},
		ByID:      map[string]int{},
		ByName:    map[string][]int{},
		ByLastFour: map[string][]int{},
	}

	for _, acc := range accs {
		lastDigits := ""
		if len(acc.Name) >= 4 {
			lastDigits = acc.Name[len(acc.Name)-4:]
		}

		cached := CachedAccount{
			ID:           acc.ID,
			Name:         acc.Name,
			CurrencyCode: acc.CurrencyCode,
			Archived:     acc.Archived,
			LastDigits:   lastDigits,
			FetchedAt:    cache.FetchedAt,
		}

		idx := len(cache.Accounts)
		cache.Accounts = append(cache.Accounts, cached)
		cache.ByID[acc.ID] = idx

		// Index by name substrings (last part of account identifier)
		nameParts := strings.Fields(acc.Name)
		if len(nameParts) > 0 {
			lastPart := nameParts[len(nameParts)-1]
			cache.ByName[lastPart] = append(cache.ByName[lastPart], idx)
		}

		// Index by last 4 digits
		if lastDigits != "" {
			cache.ByLastFour[lastDigits] = append(cache.ByLastFour[lastDigits], idx)
		}
	}
	cache.Count = len(cache.Accounts)

	// Save cache
	data, _ := json.MarshalIndent(cache, "", "  ")
	if err := os.MkdirAll(filepath.Dir(*cacheFile), 0755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.WriteFile(*cacheFile, data, 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}

	fmt.Printf("CACHED: %d accounts\n", cache.Count)
	fmt.Printf("Index by ID, name, last-4 digits saved to: %s\n", *cacheFile)

	// Show first few accounts
	fmt.Println("\nAccounts (first 10):")
	for i := 0; i < len(cache.Accounts) && i < 10; i++ {
		a := cache.Accounts[i]
		archived := ""
		if a.Archived {
			archived = " [archived]"
		}
		fmt.Printf("  %s %s %s%s\n", a.ID, a.CurrencyCode, a.Name, archived)
	}
	if len(cache.Accounts) > 10 {
		fmt.Printf("  ... and %d more\n", len(cache.Accounts)-10)
	}

	return nil
}

// LoadAccountCache loads the cached accounts from disk.
func LoadAccountCache(cacheFile string) (*AccountCache, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("load cache: %w", err)
	}
	var cache AccountCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	return &cache, nil
}

// FindAccountByCode looks up an account in the cache by code (last digits, name part, etc).
func FindAccountByCode(cache *AccountCache, code string) *CachedAccount {
	if cache == nil || len(cache.Accounts) == 0 {
		return nil
	}

	// Exact match first
	if idx, ok := cache.ByID[code]; ok {
		return &cache.Accounts[idx]
	}

	// Try name match (e.g., "3690" → "Checking 3690")
	if indices, ok := cache.ByName[code]; ok && len(indices) > 0 {
		return &cache.Accounts[indices[0]]
	}

	// Try last-4 match
	if indices, ok := cache.ByLastFour[code]; ok && len(indices) > 0 {
		return &cache.Accounts[indices[0]]
	}

	// Try fuzzy: if code ends with digits, search for those digits
	if len(code) >= 4 {
		lastFour := code[len(code)-4:]
		if indices, ok := cache.ByLastFour[lastFour]; ok && len(indices) > 0 {
			return &cache.Accounts[indices[0]]
		}
	}

	return nil
}
