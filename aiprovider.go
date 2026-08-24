package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/crydensync/cryden/v2/ai"
)

// llmProvider is csax's ai.LLMProvider implementation. It's a thin
// OpenAI-chat-completions-shaped client — Groq, OpenRouter, and most
// other hosted providers all speak this same request/response shape,
// so supporting another one is a base-URL entry here, not a new
// client. ai.LLMProvider ships zero implementations upstream in
// cryden by design — this is the consumer bringing its own provider
// and key, same pattern as notify.EmailSender and logger.Logger.
type llmProvider struct {
	baseURL string
	apiKey  string
	model   string
}

// chatCompletionsBaseURL maps AI_PROVIDER to its endpoint. Adding a
// new OpenAI-compatible provider is one line here.
var chatCompletionsBaseURL = map[string]string{
	"groq":       "https://api.groq.com/openai/v1/chat/completions",
	"openrouter": "https://openrouter.ai/api/v1/chat/completions",
}

func newLLMProvider(cfg csaxConfig) (*llmProvider, error) {
	if cfg.AIProvider == "" {
		return nil, fmt.Errorf("AI_PROVIDER is not set (expected one of: groq, openrouter)")
	}
	baseURL, ok := chatCompletionsBaseURL[cfg.AIProvider]
	if !ok {
		known := make([]string, 0, len(chatCompletionsBaseURL))
		for k := range chatCompletionsBaseURL {
			known = append(known, k)
		}
		return nil, fmt.Errorf("unknown AI_PROVIDER %q (expected one of: %s)", cfg.AIProvider, strings.Join(known, ", "))
	}
	if cfg.AIAPIKeyEnv == "" {
		return nil, fmt.Errorf("AI_API_KEY_ENV is not set — point it at the env var holding your %s API key", cfg.AIProvider)
	}
	apiKey := os.Getenv(cfg.AIAPIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("env var %s (named by AI_API_KEY_ENV) is not set or empty", cfg.AIAPIKeyEnv)
	}
	model := cfg.AIModel
	if model == "" {
		return nil, fmt.Errorf("AI_MODEL is not set")
	}
	return &llmProvider{baseURL: baseURL, apiKey: apiKey, model: model}, nil
}

// ParseQueryIntent asks the model to translate natural language into
// JSON matching ai.QueryIntent's shape. The model's raw output is
// still just text at this point — untrusted — and gets validated
// against the real allowlist by ai.ExecuteQuery AFTER this returns,
// never trusted just because it parsed as valid JSON.
func (p *llmProvider) ParseQueryIntent(ctx context.Context, naturalLanguage string) (ai.QueryIntent, error) {
	systemPrompt := strings.TrimSpace(`
You translate an admin's natural-language request into a JSON object with this exact shape:
{"entity": "users|sessions|audit_events", "filters": [{"field": "...", "operator": "=|>|<|contains", "value": "..."}], "aggregate": "|count|group_by", "group_by": "", "limit": 0}
Only use these fields per entity:
- users: id, email, failed_attempts, locked_until, created_at
- sessions: id, user_id, ip, user_agent, created_at, revoked_at
- audit_events: id, type, user_id, ip, created_at
Never invent a field or entity outside these lists. Reply with ONLY the JSON object, no other text, no markdown fences.
`)

	raw, err := p.chatCompletion(ctx, systemPrompt, naturalLanguage)
	if err != nil {
		return ai.QueryIntent{}, err
	}

	var parsed struct {
		Entity    string           `json:"entity"`
		Filters   []ai.QueryFilter `json:"filters"`
		Aggregate string           `json:"aggregate"`
		GroupBy   string           `json:"group_by"`
		Limit     int              `json:"limit"`
	}
	if err := json.Unmarshal([]byte(stripCodeFence(raw)), &parsed); err != nil {
		return ai.QueryIntent{}, fmt.Errorf("model did not return valid JSON: %w", err)
	}
	return ai.QueryIntent{
		Entity:    parsed.Entity,
		Filters:   parsed.Filters,
		Aggregate: parsed.Aggregate,
		GroupBy:   parsed.GroupBy,
		Limit:     parsed.Limit,
	}, nil
}

// Summarize is used by `csax ai logs` and `csax ai audit` — plain
// text in, plain text out, no QueryIntent structure needed since
// those commands aren't building a database query from the model's
// output, just asking it to narrate something csax already fetched
// itself.
func (p *llmProvider) Summarize(ctx context.Context, systemPrompt, userContent string) (string, error) {
	return p.chatCompletion(ctx, systemPrompt, userContent)
}

func (p *llmProvider) chatCompletion(ctx context.Context, systemPrompt, userContent string) (string, error) {
	reqBody := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach the AI provider: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI provider request failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// stripCodeFence handles the common case of a model wrapping its
// JSON in ```json ... ``` despite being told not to — defensive, not
// load-bearing: ai.validateIntent still runs on whatever this
// produces, so a model that ignores instructions in some OTHER way
// still can't produce an unsafe query, just an error.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
