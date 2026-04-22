package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sdt-project/sdt/pkg/log"
	"golang.org/x/oauth2/google"
)

const (
	claudeDefaultAPIURL    = "https://api.anthropic.com/v1/messages"
	claudeDefaultModel     = "claude-opus-4-6"
	claudeAPIVersion       = "2023-06-01"
	claudeVertexAPIVersion = "vertex-2023-10-16"
)

// ClaudeProvider implements Provider for the Anthropic Claude API (direct or Vertex AI).
type ClaudeProvider struct {
	apiKey        string
	apiURL        string
	model         string
	maxTokens     int
	httpClient    *http.Client
	useVertex     bool
	vertexProject string
	vertexRegion  string
}

// NewClaudeProvider creates a Claude provider using environment variables.
func NewClaudeProvider(model string, maxTokens int) (*ClaudeProvider, error) {
	if model == "" {
		model = claudeDefaultModel
	}

	p := &ClaudeProvider{
		apiKey:    os.Getenv("ANTHROPIC_API_KEY"),
		apiURL:    claudeDefaultAPIURL,
		model:     model,
		maxTokens: maxTokens,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}

	if p.apiKey == "" {
		project := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
		region := os.Getenv("CLOUD_ML_REGION")
		if project != "" && region != "" {
			p.useVertex = true
			p.vertexProject = project
			p.vertexRegion = region

			ctx := context.Background()
			creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
			if err != nil {
				return nil, fmt.Errorf("finding Google credentials: %w", err)
			}
			if creds.ProjectID != "" && project == "" {
				p.vertexProject = creds.ProjectID
			}

			p.httpClient = &http.Client{
				Timeout:   5 * time.Minute,
				Transport: &oauth2Transport{creds: creds},
			}

			log.Infof("LLM", "Using Vertex AI (Claude): project=%s region=%s model=%s", p.vertexProject, p.vertexRegion, p.model)
			return p, nil
		}
		return nil, fmt.Errorf("no Claude API credentials: set ANTHROPIC_API_KEY or ANTHROPIC_VERTEX_PROJECT_ID + CLOUD_ML_REGION")
	}

	log.Infof("LLM", "Using Anthropic API: model=%s", p.model)
	return p, nil
}

func (p *ClaudeProvider) ModelName() string {
	return p.model
}

func (p *ClaudeProvider) SendMessage(ctx context.Context, req *Request) (*Response, error) {
	if req.Model == "" {
		req.Model = p.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = p.maxTokens
	}

	apiURL := p.apiURL
	var body []byte
	var err error
	if p.useVertex {
		apiURL = p.vertexURL(req.Model)
		vReq := vertexRequest{
			MaxTokens:        req.MaxTokens,
			System:           req.System,
			Messages:         req.Messages,
			Tools:            req.Tools,
			Temperature:      req.Temperature,
			Thinking:         req.Thinking,
			AnthropicVersion: claudeVertexAPIVersion,
		}
		body, err = json.Marshal(vReq)
	} else {
		body, err = json.Marshal(req)
	}
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if !p.useVertex {
		httpReq.Header.Set("x-api-key", p.apiKey)
		httpReq.Header.Set("anthropic-version", claudeAPIVersion)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp Response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return &apiResp, nil
}

func (p *ClaudeProvider) vertexURL(model string) string {
	return fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:rawPredict",
		p.vertexRegion, p.vertexProject, p.vertexRegion, model,
	)
}

// oauth2Transport is an http.RoundTripper that adds Google OAuth2 bearer tokens.
type oauth2Transport struct {
	creds *google.Credentials
}

func (t *oauth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("getting Google access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	return http.DefaultTransport.RoundTrip(req)
}
