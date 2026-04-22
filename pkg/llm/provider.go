package llm

import "context"

// Provider is the interface for LLM backends (Claude, Gemini, etc.).
// The Client delegates SendMessage to the active provider.
type Provider interface {
	SendMessage(ctx context.Context, req *Request) (*Response, error)
	ModelName() string
}
