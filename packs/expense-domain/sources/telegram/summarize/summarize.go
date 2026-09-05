// Package summarize orders the fetched chats (DMs first, then groups, then
// channels), builds a compact transcript, and asks the Claude API to write one
// concise natural-language digest grouped by chat.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sumitasok/sa.automation.telegram/model"
)

const (
	apiURL       = "https://api.anthropic.com/v1/messages"
	defaultModel = "claude-haiku-4-5-20251001"

	// Target length for the digest body; leaves headroom under Telegram's
	// 4096-char hard limit for the header the sender prepends.
	targetChars = 3500
)

// OrderChats returns chats sorted DMs → groups → channels, and within each
// kind by most-recent activity (newest message first).
func OrderChats(chats []model.Chat) []model.Chat {
	out := make([]model.Chat, len(chats))
	copy(out, chats)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind // KindDM=0 < KindGroup=1 < KindChannel=2
		}
		return newestDate(out[i]) > newestDate(out[j])
	})
	return out
}

func newestDate(c model.Chat) int64 {
	var max int64
	for _, m := range c.Messages {
		if m.Date > max {
			max = m.Date
		}
	}
	return max
}

// Summarize builds the digest via Claude. apiKey must be non-empty. It returns
// the digest text (no leading header — the sender adds that).
func Summarize(ctx context.Context, apiKey string, chats []model.Chat) (string, error) {
	ordered := OrderChats(chats)
	transcript := BuildTranscript(ordered)

	modelName := defaultModel
	if m := strings.TrimSpace(os.Getenv("TELEGRAM_SUMMARY_MODEL")); m != "" {
		modelName = m
	}

	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 2048,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": buildPrompt(transcript)},
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Claude API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Claude API %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("parsing API response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return strings.TrimSpace(apiResp.Content[0].Text), nil
}

// BuildTranscript renders the ordered chats into the compact text the model
// summarizes. Media messages are described by type, never transcribed.
func BuildTranscript(ordered []model.Chat) string {
	var b strings.Builder
	for _, c := range ordered {
		fmt.Fprintf(&b, "### [%s] %s (%d new)\n", c.Kind, c.Title, len(c.Messages))
		for _, m := range c.Messages {
			line := renderMessage(m)
			if line != "" {
				b.WriteString(line)
				b.WriteByte('\n')
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func renderMessage(m model.Message) string {
	sender := m.Sender
	if sender == "" {
		sender = "—"
	}
	text := strings.TrimSpace(strings.ReplaceAll(m.Text, "\n", " "))
	switch {
	case text != "" && m.MediaType != "":
		return fmt.Sprintf("%s: [%s] %s", sender, m.MediaType, text)
	case text != "":
		return fmt.Sprintf("%s: %s", sender, text)
	case m.MediaType != "":
		return fmt.Sprintf("%s: [sent a %s]", sender, m.MediaType)
	default:
		return ""
	}
}

// Fallback renders a deterministic, non-LLM digest. Used only when the Claude
// call fails, so a run still delivers something useful.
func Fallback(chats []model.Chat) string {
	ordered := OrderChats(chats)
	var b strings.Builder
	b.WriteString("(automatic summary unavailable — raw counts)\n\n")
	for _, c := range ordered {
		fmt.Fprintf(&b, "• %s %q: %d new message(s)\n", c.Kind, c.Title, len(c.Messages))
	}
	return strings.TrimSpace(b.String())
}
