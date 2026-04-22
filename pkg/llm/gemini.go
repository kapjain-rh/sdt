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
	geminiDefaultModel = "gemini-2.5-pro"
)

// GeminiProvider implements Provider for the Google Gemini API via Vertex AI.
type GeminiProvider struct {
	model      string
	maxTokens  int
	project    string
	region     string
	httpClient *http.Client
}

// NewGeminiProvider creates a Gemini provider using Vertex AI credentials.
func NewGeminiProvider(model string, maxTokens int) (*GeminiProvider, error) {
	if model == "" {
		model = geminiDefaultModel
	}

	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		project = os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	}
	region := os.Getenv("GOOGLE_CLOUD_REGION")
	if region == "" {
		region = os.Getenv("CLOUD_ML_REGION")
	}
	if project == "" || region == "" {
		return nil, fmt.Errorf("Gemini requires GOOGLE_CLOUD_PROJECT (or ANTHROPIC_VERTEX_PROJECT_ID) and GOOGLE_CLOUD_REGION (or CLOUD_ML_REGION)")
	}

	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("finding Google credentials for Gemini: %w", err)
	}

	p := &GeminiProvider{
		model:     model,
		maxTokens: maxTokens,
		project:   project,
		region:    region,
		httpClient: &http.Client{
			Timeout:   10 * time.Minute,
			Transport: &oauth2Transport{creds: creds},
		},
	}

	log.Infof("LLM", "Using Gemini (Vertex AI): project=%s region=%s model=%s", p.project, p.region, p.model)
	return p, nil
}

func (p *GeminiProvider) ModelName() string {
	return p.model
}

func (p *GeminiProvider) SendMessage(ctx context.Context, req *Request) (*Response, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = p.maxTokens
	}

	geminiReq := p.toGeminiRequest(req)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling Gemini request: %w", err)
	}

	apiURL := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.project, p.region, p.model,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating Gemini HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending Gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshaling Gemini response: %w", err)
	}

	return p.fromGeminiResponse(&geminiResp)
}

// toGeminiRequest converts our common Request to Gemini API format.
func (p *GeminiProvider) toGeminiRequest(req *Request) *geminiRequest {
	gr := &geminiRequest{
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens: req.MaxTokens,
		},
	}

	// System instruction
	if req.System != "" {
		gr.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	// Build a map of tool_use ID → name from all messages for resolving tool_result names
	toolIDToName := map[string]string{}
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "tool_use" {
				toolIDToName[block.ID] = block.Name
			}
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		gc := geminiContent{
			Role: geminiRole(msg.Role),
		}
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				gc.Parts = append(gc.Parts, geminiPart{Text: block.Text})
			case "tool_use":
				var args map[string]interface{}
				if block.Input != nil {
					_ = json.Unmarshal(block.Input, &args)
				}
				gc.Parts = append(gc.Parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: block.Name,
						Args: args,
					},
				})
			case "tool_result":
				// Look up function name from the corresponding tool_use block
				fnName := block.Name
				if fnName == "" {
					fnName = toolIDToName[block.ToolUseID]
				}
				gc.Parts = append(gc.Parts, geminiPart{
					FunctionResponse: &geminiFunctionResponse{
						Name: fnName,
						Response: map[string]interface{}{
							"result":   block.Content,
							"is_error": block.IsError,
						},
					},
				})
			}
		}
		if len(gc.Parts) > 0 {
			gr.Contents = append(gr.Contents, gc)
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var decls []geminiFunctionDeclaration
		for _, tool := range req.Tools {
			var params map[string]interface{}
			if tool.InputSchema != nil {
				_ = json.Unmarshal(tool.InputSchema, &params)
			}
			decls = append(decls, geminiFunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			})
		}
		gr.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	return gr
}

// fromGeminiResponse converts a Gemini response to our common Response format.
func (p *GeminiProvider) fromGeminiResponse(gr *geminiResponse) (*Response, error) {
	if len(gr.Candidates) == 0 {
		return nil, fmt.Errorf("Gemini returned no candidates")
	}

	candidate := gr.Candidates[0]
	resp := &Response{
		Model: p.model,
		Role:  RoleAssistant,
	}

	// Map finish reason
	switch candidate.FinishReason {
	case "STOP":
		resp.StopReason = "end_turn"
	case "MAX_TOKENS":
		resp.StopReason = "max_tokens"
	case "SAFETY":
		resp.StopReason = "end_turn"
	default:
		resp.StopReason = "end_turn"
	}

	// Convert parts to content blocks
	toolCallIdx := 0
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			resp.Content = append(resp.Content, ContentBlock{
				Type: "text",
				Text: part.Text,
			})
		}
		if part.FunctionCall != nil {
			inputJSON, _ := json.Marshal(part.FunctionCall.Args)
			toolCallIdx++
			resp.Content = append(resp.Content, ContentBlock{
				Type:  "tool_use",
				ID:    fmt.Sprintf("gemini_call_%d", toolCallIdx),
				Name:  part.FunctionCall.Name,
				Input: json.RawMessage(inputJSON),
			})
			resp.StopReason = "tool_use"
		}
	}

	// Map usage
	if gr.UsageMetadata != nil {
		resp.Usage = Usage{
			InputTokens:  gr.UsageMetadata.PromptTokenCount,
			OutputTokens: gr.UsageMetadata.CandidatesTokenCount,
		}
	}

	return resp, nil
}

// geminiRole maps our Role to Gemini's role string.
func geminiRole(role Role) string {
	switch role {
	case RoleUser:
		return "user"
	case RoleAssistant:
		return "model"
	default:
		return string(role)
	}
}

// --- Gemini API types ---

type geminiRequest struct {
	Contents          []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	Tools             []geminiTool           `json:"tools,omitempty"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                 `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}
