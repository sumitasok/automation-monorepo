// Package config loads runtime configuration for the telegram pack from the
// process environment. Values are injected by `auto run` (ADR 0007) from the
// workspace config/telegram/ directory; nothing here reads secrets from git.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds everything the app needs for one run.
type Config struct {
	APIID   int    // TELEGRAM_API_ID   (from my.telegram.org)
	APIHash string // TELEGRAM_API_HASH (from my.telegram.org)

	AnthropicAPIKey string // ANTHROPIC_API_KEY — used by the summarizer

	// DigestChatID is the destination for the digest message. Zero means
	// "Saved Messages" (the authenticated account's own self-chat).
	DigestChatID int64

	// FirstRunLookbackHours bounds the very first run (no checkpoint yet).
	FirstRunLookbackHours int

	// ExcludeChatIDs are chats that are always skipped, even though the
	// default scope is "all chats".
	ExcludeChatIDs map[int64]bool

	// SessionPath is the MTProto session file (the long-lived credential).
	SessionPath string
}

const (
	// SessionFile is the session filename the app reads from its workdir.
	// `auto run` symlinks the real file here from config/telegram/.
	SessionFile = "session.json"

	defaultFirstRunLookbackHours = 24
)

// Load reads and validates config from the environment. requireLLM controls
// whether ANTHROPIC_API_KEY is mandatory (the login subcommand does not need it).
func Load(requireLLM bool) (*Config, error) {
	c := &Config{
		SessionPath:           SessionFile,
		FirstRunLookbackHours: defaultFirstRunLookbackHours,
		ExcludeChatIDs:        map[int64]bool{},
	}

	rawID := strings.TrimSpace(os.Getenv("TELEGRAM_API_ID"))
	if rawID == "" {
		return nil, fmt.Errorf("TELEGRAM_API_ID not set (get it from https://my.telegram.org -> API development tools)")
	}
	id, err := strconv.Atoi(rawID)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_API_ID %q is not an integer: %w", rawID, err)
	}
	c.APIID = id

	c.APIHash = strings.TrimSpace(os.Getenv("TELEGRAM_API_HASH"))
	if c.APIHash == "" {
		return nil, fmt.Errorf("TELEGRAM_API_HASH not set (get it from https://my.telegram.org)")
	}

	if requireLLM {
		c.AnthropicAPIKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		if c.AnthropicAPIKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set (needed to write the digest)")
		}
	}

	if v := strings.TrimSpace(os.Getenv("TELEGRAM_DIGEST_CHAT_ID")); v != "" {
		chatID, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("TELEGRAM_DIGEST_CHAT_ID %q is not an integer: %w", v, err)
		}
		c.DigestChatID = chatID
	}

	if v := strings.TrimSpace(os.Getenv("TELEGRAM_FIRST_RUN_LOOKBACK_HOURS")); v != "" {
		h, err := strconv.Atoi(v)
		if err != nil || h < 0 {
			return nil, fmt.Errorf("TELEGRAM_FIRST_RUN_LOOKBACK_HOURS %q must be a non-negative integer", v)
		}
		c.FirstRunLookbackHours = h
	}

	if v := strings.TrimSpace(os.Getenv("TELEGRAM_EXCLUDE_CHAT_IDS")); v != "" {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("TELEGRAM_EXCLUDE_CHAT_IDS contains %q which is not an integer: %w", part, err)
			}
			c.ExcludeChatIDs[id] = true
		}
	}

	return c, nil
}
