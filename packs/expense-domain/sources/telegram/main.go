// Command telegram is the app-backed binary for the `telegram` pack.
//
// Subcommands:
//
//	go run . login      Interactive one-time MTProto login; writes session.json.
//	go run . summary    (default) Fetch new messages since the last run,
//	                    summarize them with Claude, and send one digest message.
//
// It is read-only over your chats except for the single outbound digest message.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gotd/td/tg"

	"github.com/sumitasok/sa.automation.telegram/auth"
	"github.com/sumitasok/sa.automation.telegram/config"
	"github.com/sumitasok/sa.automation.telegram/fetch"
	"github.com/sumitasok/sa.automation.telegram/send"
	"github.com/sumitasok/sa.automation.telegram/summarize"
)

func main() {
	cmd := "summary"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "login":
		if err := runLogin(); err != nil {
			log.Fatalf("login: %v", err)
		}
	case "summary":
		if err := runSummary(); err != nil {
			log.Fatalf("summary: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\nusage:\n  go run . login      one-time interactive login\n  go run . summary    (default) build and send the digest\n", cmd)
		os.Exit(2)
	}
}

// runLogin performs the interactive MTProto login and writes the session file.
func runLogin() error {
	cfg, err := config.Load(false) // LLM key not needed to log in
	if err != nil {
		return err
	}
	ctx := context.Background()
	return auth.Login(ctx, cfg.APIID, cfg.APIHash, cfg.SessionPath)
}

// runSummary is the scheduled job: fetch → summarize → send → checkpoint.
func runSummary() error {
	cfg, err := config.Load(true)
	if err != nil {
		return err
	}

	state, err := config.LoadState(config.StateFile)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return auth.WithClient(ctx, cfg.APIID, cfg.APIHash, cfg.SessionPath, func(ctx context.Context, api *tg.Client) error {
		result, err := fetch.FetchNewMessages(ctx, api, cfg, state)
		if err != nil {
			return fmt.Errorf("fetching messages: %w", err)
		}
		if len(result.Chats) == 0 {
			log.Printf("No new messages since last run — nothing to send.")
			return nil
		}
		log.Printf("Fetched %d new message(s) across %d chat(s).", result.TotalMsgs, len(result.Chats))

		body, err := summarize.Summarize(ctx, cfg.AnthropicAPIKey, result.Chats)
		if err != nil {
			log.Printf("[WARN] LLM summary failed (%v) — sending count-only fallback.", err)
			body = summarize.Fallback(result.Chats)
		}

		digest := header(result.TotalMsgs, len(result.Chats)) + body
		if err := send.Send(ctx, api, cfg, digest); err != nil {
			return fmt.Errorf("sending digest: %w", err)
		}

		// Only advance checkpoints after a successful send, so a failed run
		// re-summarizes the same messages next time (never repeats after success).
		for _, c := range result.Chats {
			state.Advance(c.ID, c.NewestMessageID())
		}
		if err := state.Save(config.StateFile); err != nil {
			log.Printf("[WARN] saving state: %v — next run may re-summarize these messages", err)
		}

		log.Printf("Digest sent (%d chat(s), %d message(s) summarized).", len(result.Chats), result.TotalMsgs)
		return nil
	})
}

func header(totalMsgs, totalChats int) string {
	now := time.Now().Format("Mon 2 Jan 2006, 15:04")
	return fmt.Sprintf("Telegram digest — %s\n%d new message(s) across %d chat(s)\n\n", now, totalMsgs, totalChats)
}
