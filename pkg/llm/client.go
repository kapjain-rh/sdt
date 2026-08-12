package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sdt-project/sdt/pkg/log"
)

// ErrStopExecution is returned by tool handlers to signal the agent loop should
// stop immediately (e.g., report_step_result reporting FAIL). Unlike stopOnError
// which stops on ANY tool error, this stops the loop regardless of stopOnError setting.
var ErrStopExecution = errors.New("execution stopped")

const (
	defaultMaxTokens       = 16384
	defaultMaxIterations   = 50
	maxConsecutiveErrors   = 5
	maxToolResultLen       = 8000
	maxDiagnosticIter      = 10
	compactAfterIteration  = 10
	keepRecentPairs        = 5
)

// Client wraps an LLM provider with convenience methods (Chat, RunAgentLoop).
// All agent code uses *Client; the provider handles the API-specific details.
type Client struct {
	provider  Provider
	maxTokens int
	usage     *UsageTracker
}

// ClientOption configures the LLM client.
type ClientOption func(*Client)

// WithMaxTokens sets the maximum output tokens.
func WithMaxTokens(tokens int) ClientOption {
	return func(c *Client) { c.maxTokens = tokens }
}

// WithProvider sets a custom LLM provider (bypasses env-var auto-detection).
func WithProvider(p Provider) ClientOption {
	return func(c *Client) { c.provider = p }
}

// NewClient creates a new LLM client, selecting the provider based on environment:
//   - SDT_PROVIDER=gemini → Gemini via Vertex AI
//   - Otherwise → Claude (direct Anthropic API or Vertex AI)
//
// Model can be overridden via SDT_MODEL.
// Use WithProvider() to inject a custom provider instead of auto-detecting.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		maxTokens: defaultMaxTokens,
		usage:     &UsageTracker{},
	}

	for _, opt := range opts {
		opt(c)
	}

	// If a provider was injected via WithProvider, skip auto-detection
	if c.provider != nil {
		return c, nil
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

// NewClientWithProvider creates a client with an explicit provider.
func NewClientWithProvider(provider Provider, opts ...ClientOption) *Client {
	c := &Client{
		provider:  provider,
		maxTokens: defaultMaxTokens,
		usage:     &UsageTracker{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SendMessage sends a message to the LLM and returns the response.
func (c *Client) SendMessage(ctx context.Context, req *Request) (*Response, error) {
	resp, err := c.provider.SendMessage(ctx, req)
	if err == nil && resp != nil {
		c.usage.Add(resp.Usage)
	}
	return resp, err
}

// Usage returns the accumulated token usage tracker.
func (c *Client) Usage() *UsageTracker {
	return c.usage
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
	consecutiveErrors := 0
	loopStart := time.Now()
	log.Infof("LLM", "Starting agent loop (model=%s, tools=%d)", c.provider.ModelName(), len(req.Tools))

	maxIter := defaultMaxIterations
	if req.MaxIterations > 0 {
		maxIter = req.MaxIterations
	}
	if v := os.Getenv("SDT_MAX_ITERATIONS"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &maxIter); n != 1 || err != nil {
			maxIter = defaultMaxIterations
		}
	}

	for {
		iteration++
		if iteration > maxIter {
			log.Warnf("LLM", "Agent loop hit max iterations (%d) after %s — stopping", maxIter, time.Since(loopStart))
			return nil, logs, fmt.Errorf("agent loop exceeded %d iterations", maxIter)
		}
		if iteration > compactAfterIteration {
			if n := compactMessages(messages, keepRecentPairs); n > 0 {
				log.Debugf("LLM", "Compacted %d blocks in older messages (keeping last %d pairs)", n, keepRecentPairs)
			}
		}

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
		allFailed := true
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
				if errors.Is(err, ErrStopExecution) {
					log.Infof("LLM", "Agent loop stopping (ErrStopExecution) — %d iterations, %d tool calls, total %s",
						iteration, len(logs), time.Since(loopStart))
					return nil, logs, fmt.Errorf("tool %s failed: %w", tc.Name, err)
				}
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
				allFailed = false
				tcLog.Output = result
				outputPreview := result
				if len(outputPreview) > 200 {
					outputPreview = outputPreview[:200] + "..."
				}
				log.Infof("TOOL", "%s OK after %s: %s", tc.Name, tcLog.Duration, outputPreview)
				logs = append(logs, tcLog)
				content := result
				if len(content) > maxToolResultLen {
					content = content[:maxToolResultLen] + "\n... (truncated, " + fmt.Sprintf("%d", len(result)) + " chars total)"
				}
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.ID,
					Content:   content,
				})
			}
		}

		if allFailed {
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Warnf("LLM", "Agent loop stopping after %d consecutive iterations with all tool calls failing", consecutiveErrors)
				return nil, logs, fmt.Errorf("agent loop stopped: %d consecutive iterations with all tool calls failing", consecutiveErrors)
			}
		} else {
			consecutiveErrors = 0
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

// compactMessages reduces context size by summarizing old tool results.
// Preserves the first message (user prompt) and the last keepRecent message
// pairs in full detail. Older tool_result blocks get one-line summaries and
// tool_use inputs are cleared.
func compactMessages(messages []Message, keepRecent int) int {
	if len(messages) < 3 {
		return 0
	}

	totalPairs := (len(messages) - 1) / 2
	if totalPairs <= keepRecent {
		return 0
	}

	compactUpTo := 1 + (totalPairs-keepRecent)*2
	compacted := 0

	for i := 1; i < compactUpTo && i < len(messages); i++ {
		msg := &messages[i]
		for j := range msg.Content {
			block := &msg.Content[j]
			if block.Type == "tool_result" && len(block.Content) > 100 {
				if block.IsError {
					summary := block.Content
					if len(summary) > 100 {
						summary = summary[:100] + "..."
					}
					block.Content = fmt.Sprintf("[failed] %s", summary)
				} else {
					block.Content = "[completed] (output compacted)"
				}
				compacted++
			}
			if block.Type == "tool_use" && len(block.Input) > 0 {
				block.Input = json.RawMessage("{}")
				compacted++
			}
		}
	}
	return compacted
}
