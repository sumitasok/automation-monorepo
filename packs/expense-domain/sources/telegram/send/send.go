// Package send delivers the finished digest as one (or, if long, a few)
// Telegram message(s). It resolves the destination — the configured chat_id or,
// when none is set, the account's own Saved Messages — and calls
// messages.sendMessage. This is the only write the pack performs.
package send

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/sumitasok/sa.automation.telegram/config"
	"github.com/sumitasok/sa.automation.telegram/fetch"
)

// telegramMaxChars is the hard per-message length limit.
const telegramMaxChars = 4096

// Send delivers text to the configured destination, splitting into multiple
// messages if it exceeds the per-message limit.
func Send(ctx context.Context, api *tg.Client, cfg *config.Config, text string) error {
	dest, label, err := resolveDest(ctx, api, cfg)
	if err != nil {
		return err
	}

	chunks := splitMessage(text, telegramMaxChars)
	for i, chunk := range chunks {
		rid, err := randomID()
		if err != nil {
			return fmt.Errorf("generating random id: %w", err)
		}
		if _, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:      dest,
			Message:   chunk,
			RandomID:  rid,
			NoWebpage: true, // don't expand link previews in the digest
		}); err != nil {
			return fmt.Errorf("sending digest part %d/%d to %s: %w", i+1, len(chunks), label, err)
		}
	}
	return nil
}

// resolveDest returns the InputPeer for the digest destination and a human label.
// Empty DigestChatID → Saved Messages (InputPeerSelf). Otherwise the chat is
// located among the account's dialogs (which carry the needed access hash).
func resolveDest(ctx context.Context, api *tg.Client, cfg *config.Config) (tg.InputPeerClass, string, error) {
	if cfg.DigestChatID == 0 {
		return &tg.InputPeerSelf{}, "Saved Messages", nil
	}
	peers, err := fetch.ScanPeers(ctx, api)
	if err != nil {
		return nil, "", fmt.Errorf("scanning dialogs to resolve digest chat: %w", err)
	}
	for _, p := range peers {
		if matchesChatID(p.ChatID, cfg.DigestChatID) {
			return p.Input, p.Title, nil
		}
	}
	return nil, "", fmt.Errorf(
		"digest chat_id %d not found in your dialogs — leave TELEGRAM_DIGEST_CHAT_ID empty to use Saved Messages, "+
			"or ensure that chat is one you're a member of and appears in your recent dialogs",
		cfg.DigestChatID)
}

// matchesChatID reports whether a raw MTProto chat id equals the configured id,
// accepting the Bot-API encodings a user might paste (-100… for channels, -… for
// basic groups) as well as the bare raw id.
func matchesChatID(raw, want int64) bool {
	if raw == want {
		return true
	}
	if -(1_000_000_000_000 + raw) == want { // channel / supergroup Bot-API form
		return true
	}
	if -raw == want { // basic group Bot-API form
		return true
	}
	return false
}

// splitMessage breaks s into chunks no longer than limit, preferring to split on
// line boundaries. A single over-long line is hard-cut.
func splitMessage(s string, limit int) []string {
	if len(s) <= limit {
		return []string{s}
	}
	var chunks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
		}
	}
	for _, line := range strings.SplitAfter(s, "\n") {
		// Hard-cut a single line longer than the limit.
		for len(line) > limit {
			flush()
			chunks = append(chunks, line[:limit])
			line = line[limit:]
		}
		if cur.Len()+len(line) > limit {
			flush()
		}
		cur.WriteString(line)
	}
	flush()

	// Tag multi-part messages so the reader knows there's more.
	if len(chunks) > 1 {
		for i := range chunks {
			chunks[i] = fmt.Sprintf("(%d/%d)\n%s", i+1, len(chunks), chunks[i])
		}
	}
	return chunks
}

func randomID() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}
