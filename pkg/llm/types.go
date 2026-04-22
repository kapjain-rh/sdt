package llm

import "encoding/json"

// Role represents a message role in the conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message represents a single message in the conversation.
type Message struct {
	Role    Role          `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a block of content within a message.
// Can be text, tool_use, tool_result, or thinking.
type ContentBlock struct {
	Type string `json:"type"`

	// For type "text"
	Text string `json:"text,omitempty"`

	// For type "tool_use"
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For type "tool_result"
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	// For type "thinking"
	Thinking string `json:"thinking,omitempty"`
}

// ToolDefinition defines a tool that the LLM can call.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Request represents a Claude API Messages request.
type Request struct {
	Model             string           `json:"model"`
	MaxTokens         int              `json:"max_tokens"`
	System            string           `json:"system,omitempty"`
	Messages          []Message        `json:"messages"`
	Tools             []ToolDefinition `json:"tools,omitempty"`
	Temperature       *float64         `json:"temperature,omitempty"`
	Thinking          *ThinkingConfig  `json:"thinking,omitempty"`
	AnthropicVersion  string           `json:"anthropic_version,omitempty"` // Required for Vertex AI (set in body instead of header)
}

// vertexRequest is the Vertex AI variant that omits the model field from JSON
// (model is specified in the URL for Vertex rawPredict).
type vertexRequest struct {
	MaxTokens        int              `json:"max_tokens"`
	System           string           `json:"system,omitempty"`
	Messages         []Message        `json:"messages"`
	Tools            []ToolDefinition `json:"tools,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	Thinking         *ThinkingConfig  `json:"thinking,omitempty"`
	AnthropicVersion string           `json:"anthropic_version"`
	Stream           bool             `json:"stream,omitempty"`
}

// ThinkingConfig enables extended thinking (reasoning mode).
type ThinkingConfig struct {
	Type         string `json:"type"`          // "enabled"
	BudgetTokens int    `json:"budget_tokens"` // Max tokens for thinking
}

// Response represents a Claude API Messages response.
type Response struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         Role           `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	Usage        Usage          `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read_input_tokens,omitempty"`
	CacheCreate  int `json:"cache_creation_input_tokens,omitempty"`
}

// ToolCall extracts tool use blocks from a response.
func (r *Response) ToolCalls() []ContentBlock {
	var calls []ContentBlock
	for _, b := range r.Content {
		if b.Type == "tool_use" {
			calls = append(calls, b)
		}
	}
	return calls
}

// TextContent extracts all text blocks from a response, concatenated.
func (r *Response) TextContent() string {
	var parts []string
	for _, b := range r.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "\n"
		}
		result += p
	}
	return result
}

// ThinkingContent extracts thinking blocks from a response.
func (r *Response) ThinkingContent() string {
	var parts []string
	for _, b := range r.Content {
		if b.Type == "thinking" && b.Thinking != "" {
			parts = append(parts, b.Thinking)
		}
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "\n"
		}
		result += p
	}
	return result
}

// HasToolCalls returns true if the response contains tool use blocks.
func (r *Response) HasToolCalls() bool {
	for _, b := range r.Content {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}

// IsEndTurn returns true if the model stopped because it finished (not for tool use).
func (r *Response) IsEndTurn() bool {
	return r.StopReason == "end_turn"
}
