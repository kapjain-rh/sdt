package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/openshift/sdt/pkg/log"
)

const (
	defaultMaxTokens = 16384
)

// Client wraps an LLM provider with convenience methods (Chat, RunAgentLoop).
// All agent code uses *Client; the provider handles the API-specific details.
type Client struct {
	provider  Provider
	maxTokens int
}

// ClientOption configures the LLM client.
type ClientOption func(*Client)

// WithMaxTokens sets the maximum output tokens.
func WithMaxTokens(tokens int) ClientOption {
	return func(c *Client) { c.maxTokens = tokens }
}

// NewClient creates a new LLM client, selecting the provider based on environment:
//   - SDT_PROVIDER=gemini → Gemini via Vertex AI
//   - Otherwise → Claude (direct Anthropic API or Vertex AI)
//
// Model can be overridden via SDT_MODEL.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		maxTokens: defaultMaxTokens,
	}

	for _, opt := range opts {
		opt(c)
	}

	model := os.Getenv("SDT_MODEL")
	providerName := os.Getenv("SDT_PROVIDER")

	var provider Provider
	var err error

	switch providerName {
	case "gemini":
		provider, err = NewGeminiProvider(model, c.maxTokens)
	default:
		provider, err = NewClaudeProvider(model, c.maxTokens)
	}

	if err != nil {
		return nil, fmt.Errorf("initializing %s provider: %w", providerName, err)
	}

	c.provider = provider
	return c, nil
}

// SendMessage sends a message to the LLM and returns the response.
func (c *Client) SendMessage(ctx context.Context, req *Request) (*Response, error) {
	return c.provider.SendMessage(ctx, req)
}

// Chat is a convenience method for a simple text message exchange.
func (c *Client) Chat(ctx context.Context, system string, userMessage string, tools []ToolDefinition) (*Response, error) {
	req := &Request{
		System: system,
		Messages: []Message{
			{
				Role: RoleUser,
				Content: []ContentBlock{
					{Type: "text", Text: userMessage},
				},
			},
		},
		Tools: tools,
	}
	return c.SendMessage(ctx, req)
}

// ChatWithThinking is like Chat but enables extended thinking (Claude-specific).
func (c *Client) ChatWithThinking(ctx context.Context, system string, userMessage string, tools []ToolDefinition, thinkingBudget int) (*Response, error) {
	req := &Request{
		System: system,
		Messages: []Message{
			{
				Role: RoleUser,
				Content: []ContentBlock{
					{Type: "text", Text: userMessage},
				},
			},
		},
		Tools: tools,
		Thinking: &ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: thinkingBudget,
		},
	}
	return c.SendMessage(ctx, req)
}

// RunAgentLoop executes a multi-turn agent loop:
// 1. Send messages to the LLM
// 2. If the LLM responds with tool calls, execute them via toolHandler
// 3. Send tool results back
// 4. Repeat until the LLM responds with end_turn (no more tool calls)
//
// When stopOnError is true, the loop stops immediately on the first tool call
// failure and returns the error along with all logs collected so far.
func (c *Client) RunAgentLoop(ctx context.Context, req *Request, toolHandler func(name string, input json.RawMessage) (string, error), stopOnError bool) (*Response, []ToolCallLog, error) {
	var logs []ToolCallLog
	messages := make([]Message, len(req.Messages))
	copy(messages, req.Messages)

	iteration := 0
	loopStart := time.Now()
	log.Infof("LLM", "Starting agent loop (model=%s, tools=%d)", c.provider.ModelName(), len(req.Tools))

	for {
		iteration++
		iterStart := time.Now()
		log.Infof("LLM", "Agent loop iteration %d — sending request with %d messages", iteration, len(messages))

		currentReq := &Request{
			Model:     req.Model,
			MaxTokens: req.MaxTokens,
			System:    req.System,
			Messages:  messages,
			Tools:     req.Tools,
			Thinking:  req.Thinking,
		}

		resp, err := c.SendMessage(ctx, currentReq)
		if err != nil {
			log.Errorf("LLM", "Agent loop iteration %d — API error after %s: %v", iteration, time.Since(iterStart), err)
			return nil, logs, fmt.Errorf("agent loop send: %w", err)
		}

		log.Infof("LLM", "Agent loop iteration %d — response received (stop_reason=%s) in %s", iteration, resp.StopReason, time.Since(iterStart))

		// If no tool calls, we're done
		if !resp.HasToolCalls() {
			log.Infof("LLM", "Agent loop completed — %d iterations, %d tool calls, total %s", iteration, len(logs), time.Since(loopStart))
			return resp, logs, nil
		}

		toolCalls := resp.ToolCalls()
		log.Debugf("LLM", "Agent loop iteration %d — %d tool call(s) requested", iteration, len(toolCalls))

		// Add assistant response to messages
		messages = append(messages, Message{
			Role:    RoleAssistant,
			Content: resp.Content,
		})

		// Process tool calls
		var toolResults []ContentBlock
		for _, tc := range toolCalls {
			log.Infof("TOOL", "Calling %s (id=%s)", tc.Name, tc.ID)
			start := time.Now()
			result, err := toolHandler(tc.Name, tc.Input)

			tcLog := ToolCallLog{
				ToolName: tc.Name,
				Input:    tc.Input,
				Duration: time.Since(start),
			}

			if err != nil {
				tcLog.Error = err.Error()
				log.Errorf("TOOL", "%s FAILED after %s: %s", tc.Name, tcLog.Duration, err.Error())
				logs = append(logs, tcLog)
				if stopOnError {
					log.Infof("LLM", "Agent loop stopping on tool error (stop_on_error=true) — %d iterations, %d tool calls, total %s",
						iteration, len(logs), time.Since(loopStart))
					return nil, logs, fmt.Errorf("tool %s failed: %w", tc.Name, err)
				}
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.ID,
					Content:   fmt.Sprintf("Error: %s", err.Error()),
					IsError:   true,
				})
			} else {
				tcLog.Output = result
				outputPreview := result
				if len(outputPreview) > 200 {
					outputPreview = outputPreview[:200] + "..."
				}
				log.Infof("TOOL", "%s OK after %s: %s", tc.Name, tcLog.Duration, outputPreview)
				logs = append(logs, tcLog)
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.ID,
					Content:   result,
				})
			}
		}

		// Add tool results as user message
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: toolResults,
		})
	}
}

// ToolCallLog records a single tool invocation during an agent loop.
type ToolCallLog struct {
	ToolName string
	Input    json.RawMessage
	Output   string
	Error    string
	Duration time.Duration
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.provider.ModelName()
}
