// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/plg-email-classifier/internal/apierror"
)

const (
	messagesURL = "https://api.anthropic.com/v1/messages"
	apiVersion  = "2023-06-01"
	maxTokens   = 512
)

// Config holds the configuration for the Anthropic client.
type Config struct {
	APIKey  string
	Model   string
	Timeout time.Duration
}

// Client calls the Anthropic Messages API using structured outputs so the
// response is guaranteed valid JSON matching our schema.
type Client struct {
	http   *http.Client
	apiKey string
	model  string
}

// NewClient constructs a Client. A zero Timeout defaults to 8s.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Client{http: &http.Client{Timeout: timeout}, apiKey: cfg.APIKey, model: cfg.Model}
}

const systemPrompt = `You classify the email domain of a product signup for a B2B customer-success team.
Choose exactly one category:
- corporate: a real company or organisation's own domain.
- personal: a genuine personal address on a consumer email provider.
- provider_testing: a valid provider used for a test, QA, or throwaway-looking signup.
- disposable: a temporary, throwaway, anonymous, or "burner" email service, including privacy-focused temp-mail providers - even if it is not on any blocklist.

Use your own knowledge of the domain's reputation; do not rely only on the checks below. Those checks come from small local lists, so a "false"/"not on list" value only means the domain was not found on our list - it does NOT rule out a category. If you recognise the domain as a disposable or temporary-mail service, classify it as disposable regardless of the checks.

Return only the structured fields. confidence is your certainty from 0 to 1. Keep reasoning to one short sentence.`

// classificationSchema constrains the model output. min/max are intentionally
// NOT set on the number field: the structured-outputs API rejects
// minimum/maximum on numbers; confidence is clamped downstream in scoring.
var classificationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"category": map[string]any{
			"type": "string",
			"enum": []string{"corporate", "personal", "provider_testing", "disposable"},
		},
		"confidence": map[string]any{"type": "number"},
		"reasoning":  map[string]any{"type": "string"},
	},
	"required":             []string{"category", "confidence", "reasoning"},
	"additionalProperties": false,
}

type request struct {
	Model        string       `json:"model"`
	MaxTokens    int          `json:"max_tokens"`
	System       string       `json:"system"`
	Messages     []message    `json:"messages"`
	OutputConfig outputConfig `json:"output_config"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type outputConfig struct {
	Format outputFormat `json:"format"`
}

type outputFormat struct {
	Type   string         `json:"type"`
	Schema map[string]any `json:"schema"`
}

type response struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Classify asks the model for a verdict on the domain. It returns
// *apierror.Error on a non-2xx response so callers can log the upstream status.
func (c *Client) Classify(ctx context.Context, in ClassificationInput) (Classification, error) {
	userMsg := fmt.Sprintf(
		"Email: %s\nDomain: %s\nLocal list checks (a false/negative value only means 'not on our list', not proof of anything): mx_valid=%t on_free_provider_list=%t on_disposable_list=%t role_based_mailbox=%t\nClassify this signup using your own knowledge of the domain.",
		in.Email, in.Domain, in.MXValid, in.FreeProvider, in.Disposable, in.RoleBased,
	)

	payload, err := json.Marshal(request{
		Model:        c.model,
		MaxTokens:    maxTokens,
		System:       systemPrompt,
		Messages:     []message{{Role: "user", Content: userMsg}},
		OutputConfig: outputConfig{Format: outputFormat{Type: "json_schema", Schema: classificationSchema}},
	})
	if err != nil {
		return Classification{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(payload))
	if err != nil {
		return Classification{}, fmt.Errorf("anthropic: build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(req)
	if err != nil {
		return Classification{}, fmt.Errorf("anthropic: call messages API: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Classification{}, fmt.Errorf("anthropic: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := raw
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return Classification{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return Classification{}, fmt.Errorf("anthropic: decode response: %w", err)
	}

	var text string
	for _, b := range r.Content {
		if b.Type == "text" {
			text = b.Text
			break
		}
	}
	if strings.TrimSpace(text) == "" {
		return Classification{}, fmt.Errorf("anthropic: empty model response")
	}

	var out Classification
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return Classification{}, fmt.Errorf("anthropic: parse model json: %w", err)
	}
	return out, nil
}
