package event

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	// DeepSeek exposes an OpenAI-compatible chat-completions endpoint.
	deepseekAPIURL = "https://api.deepseek.com/chat/completions"
	// deepseek-v4-flash is the fast, non-thinking model — a good fit for this
	// structured clustering task (same default as gmail's categorize package).
	deepseekDefaultModel = "deepseek-v4-flash"
)

// NewMatcher returns the Matcher for the named provider. Empty selects the
// default (DeepSeek). Only DeepSeek is implemented today; the Matcher
// interface leaves room for others (mirrors gmail's categorize.NewAssigner).
func NewMatcher(provider, model string) (Matcher, error) {
	switch provider {
	case "", "deepseek":
		if model == "" {
			model = os.Getenv("DEEPSEEK_MODEL")
		}
		if model == "" {
			model = deepseekDefaultModel
		}
		return &deepseekMatcher{http: http.DefaultClient, model: model}, nil
	default:
		return nil, fmt.Errorf("update-event: unsupported ai provider %q (only \"deepseek\" is implemented)", provider)
	}
}

type deepseekMatcher struct {
	http  *http.Client
	model string
}

func (d *deepseekMatcher) Name() string { return "deepseek" }

func (d *deepseekMatcher) Match(ctx context.Context, events []EventRef, batch []Item) ([]MatchResult, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
	}

	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	itemsJSON, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildPrompt(string(eventsJSON), string(itemsJSON))},
		},
		"response_format": map[string]string{"type": "json_object"},
		"stream":          false,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("authorization", "Bearer "+apiKey)

	resp, err := d.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling DeepSeek API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DeepSeek API %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing DeepSeek response: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from DeepSeek")
	}

	return parseMatchResults(apiResp.Choices[0].Message.Content)
}
