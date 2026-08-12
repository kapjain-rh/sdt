package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/sdt-project/sdt/pkg/mcp"
)

type MCPDiscoveredTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type activeClient struct {
	client *mcp.Client
	cancel context.CancelFunc
	tools  []MCPDiscoveredTool
}

type MCPManager struct {
	mu      sync.RWMutex
	clients map[int64]*activeClient
}

func NewMCPManager() *MCPManager {
	return &MCPManager{clients: make(map[int64]*activeClient)}
}

func (m *MCPManager) Connect(serverID int64, name string, cfg mcp.MCPServerConfig) ([]MCPDiscoveredTool, error) {
	m.Disconnect(serverID)

	ctx, cancel := context.WithCancel(context.Background())
	client, err := mcp.NewClient(ctx, name, cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	mcpTools, err := client.ListTools()
	if err != nil {
		client.Close()
		cancel()
		return nil, fmt.Errorf("listing tools: %w", err)
	}

	tools := make([]MCPDiscoveredTool, len(mcpTools))
	for i, t := range mcpTools {
		tools[i] = MCPDiscoveredTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	m.mu.Lock()
	m.clients[serverID] = &activeClient{
		client: client,
		cancel: cancel,
		tools:  tools,
	}
	m.mu.Unlock()

	log.Printf("[MCP] Server %q connected with %d tools", name, len(tools))
	return tools, nil
}

func (m *MCPManager) Disconnect(serverID int64) {
	m.mu.Lock()
	ac, ok := m.clients[serverID]
	if ok {
		delete(m.clients, serverID)
	}
	m.mu.Unlock()

	if ok {
		ac.cancel()
		ac.client.Close()
		log.Printf("[MCP] Server ID=%d disconnected", serverID)
	}
}

func (m *MCPManager) IsConnected(serverID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[serverID]
	return ok
}

func (m *MCPManager) GetTools(serverID int64) ([]MCPDiscoveredTool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ac, ok := m.clients[serverID]
	if !ok {
		return nil, false
	}
	return ac.tools, true
}

func (m *MCPManager) RefreshTools(serverID int64) ([]MCPDiscoveredTool, error) {
	m.mu.RLock()
	ac, ok := m.clients[serverID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not connected")
	}

	mcpTools, err := ac.client.ListTools()
	if err != nil {
		return nil, err
	}

	tools := make([]MCPDiscoveredTool, len(mcpTools))
	for i, t := range mcpTools {
		tools[i] = MCPDiscoveredTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	m.mu.Lock()
	ac.tools = tools
	m.mu.Unlock()
	return tools, nil
}

func (m *MCPManager) GetAllToolsForServers(serverIDs []int64) map[int64][]MCPDiscoveredTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[int64][]MCPDiscoveredTool)
	for _, id := range serverIDs {
		if ac, ok := m.clients[id]; ok {
			result[id] = ac.tools
		}
	}
	return result
}

func (m *MCPManager) CallTool(serverID int64, toolName string, arguments json.RawMessage) (string, error) {
	m.mu.RLock()
	ac, ok := m.clients[serverID]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("server not connected")
	}
	return ac.client.CallTool(toolName, arguments)
}

func (m *MCPManager) CloseAll() {
	m.mu.Lock()
	clients := m.clients
	m.clients = make(map[int64]*activeClient)
	m.mu.Unlock()

	for id, ac := range clients {
		ac.cancel()
		ac.client.Close()
		log.Printf("[MCP] Server ID=%d closed", id)
	}
}
