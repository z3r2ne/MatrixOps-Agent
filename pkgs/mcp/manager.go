package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	database "pkgs/db"
	"pkgs/db/models"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gorm"
)


type ToolDescriptor struct {
	FullName    string
	ServerName  string
	ToolName    string
	Description string
	InputSchema map[string]interface{}
}

type serverRuntime struct {
	server models.McpServer
	client mcpclient.MCPClient
	tools  []ToolDescriptor
}

type Manager struct {
	mu          sync.RWMutex
	db          *gorm.DB
	servers     map[uint]*serverRuntime
	toolsByName map[string]ToolDescriptor
}

var (
	globalManager *Manager
	globalOnce    sync.Once
)

func InitManager(db *gorm.DB) *Manager {
	globalOnce.Do(func() {
		globalManager = &Manager{
			db:          db,
			servers:     map[uint]*serverRuntime{},
			toolsByName: map[string]ToolDescriptor{},
		}
	})
	return globalManager
}

func GetManager() *Manager {
	return globalManager
}

func (m *Manager) ConnectAll(ctx context.Context) {
	if m == nil || m.db == nil {
		return
	}
	servers, err := database.ListMcpServers(m.db)
	if err != nil {
		log.Printf("[mcp] list servers failed: %v", err)
		return
	}
	for i := range servers {
		server := servers[i]
		if !server.Enabled {
			m.disconnectServer(&server, "")
			continue
		}
		if err := m.ConnectServer(ctx, server.ID); err != nil {
			log.Printf("[mcp] connect %s failed: %v", server.Name, err)
		}
	}
}

func (m *Manager) ConnectServer(ctx context.Context, id uint) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("mcp manager not initialized")
	}
	server, err := database.GetMcpServerByID(m.db, id)
	if err != nil {
		return err
	}
	if !server.Enabled {
		m.disconnectServer(server, "")
		return nil
	}
	runtime, connectErr := m.dialServer(ctx, *server)
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.servers[id]; ok && old.client != nil {
		_ = old.client.Close()
	}
	if connectErr != nil {
		delete(m.servers, id)
		for name, desc := range m.toolsByName {
			if desc.ServerName == NormalizeName(server.Name) {
				delete(m.toolsByName, name)
			}
		}
		server.Connected = false
		server.ToolCount = 0
		server.LastConnectError = connectErr.Error()
		_ = database.UpdateMcpServer(m.db, server)
		return connectErr
	}
	normalized := NormalizeName(server.Name)
	for name, desc := range m.toolsByName {
		if desc.ServerName == normalized {
			delete(m.toolsByName, name)
		}
	}
	m.servers[id] = runtime
	for _, tool := range runtime.tools {
		m.toolsByName[tool.FullName] = tool
	}
	server.Connected = true
	server.ToolCount = len(runtime.tools)
	server.LastConnectError = ""
	_ = database.UpdateMcpServer(m.db, server)
	return nil
}

func (m *Manager) DisconnectServer(id uint) {
	if m == nil || m.db == nil {
		return
	}
	server, err := database.GetMcpServerByID(m.db, id)
	if err != nil {
		return
	}
	m.disconnectServer(server, "")
}

func (m *Manager) disconnectServer(server *models.McpServer, reason string) {
	if m == nil || server == nil {
		return
	}
	m.mu.Lock()
	if runtime, ok := m.servers[server.ID]; ok {
		if runtime.client != nil {
			_ = runtime.client.Close()
		}
		delete(m.servers, server.ID)
	}
	normalized := NormalizeName(server.Name)
	for name, desc := range m.toolsByName {
		if desc.ServerName == normalized {
			delete(m.toolsByName, name)
		}
	}
	m.mu.Unlock()

	server.Connected = false
	server.ToolCount = 0
	if reason != "" {
		server.LastConnectError = reason
	} else {
		server.LastConnectError = ""
	}
	_ = database.UpdateMcpServer(m.db, server)
}

func (m *Manager) dialServer(ctx context.Context, server models.McpServer) (*serverRuntime, error) {
	var (
		client mcpclient.MCPClient
		err    error
	)
	headers, err := parseStringMapJSON(server.HeadersJSON)
	if err != nil {
		return nil, fmt.Errorf("parse headers: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(server.Transport)) {
	case models.McpTransportSSE:
		url := strings.TrimSpace(server.URL)
		if url == "" {
			return nil, fmt.Errorf("SSE transport requires url")
		}
		var sseClient *mcpclient.Client
		if len(headers) > 0 {
			sseClient, err = mcpclient.NewSSEMCPClient(url, mcpclient.WithHeaders(headers))
		} else {
			sseClient, err = mcpclient.NewSSEMCPClient(url)
		}
		if err != nil {
			return nil, err
		}
		if startErr := sseClient.Start(ctx); startErr != nil {
			_ = sseClient.Close()
			return nil, fmt.Errorf("start: %w", startErr)
		}
		client = sseClient
	case models.McpTransportHTTP:
		url := strings.TrimSpace(server.URL)
		if url == "" {
			return nil, fmt.Errorf("HTTP transport requires url")
		}
		opts := make([]transport.StreamableHTTPCOption, 0, 1)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		httpClient, err := mcpclient.NewStreamableHttpClient(url, opts...)
		if err != nil {
			return nil, err
		}
		if startErr := httpClient.Start(ctx); startErr != nil {
			_ = httpClient.Close()
			return nil, fmt.Errorf("start: %w", startErr)
		}
		client = httpClient
	default:
		command := strings.TrimSpace(server.Command)
		if command == "" {
			return nil, fmt.Errorf("stdio transport requires command")
		}
		args, err := parseStringArrayJSON(server.ArgsJSON)
		if err != nil {
			return nil, fmt.Errorf("parse args: %w", err)
		}
		envMap, err := parseStringMapJSON(server.EnvJSON)
		if err != nil {
			return nil, fmt.Errorf("parse env: %w", err)
		}
		env := make([]string, 0, len(envMap))
		for key, value := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		client, err = mcpclient.NewStdioMCPClient(command, env, args...)
	}
	if err != nil {
		return nil, err
	}

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "matrixops",
		Version: "1.0.0",
	}
	if _, err := client.Initialize(initCtx, initReq); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	listCtx, listCancel := context.WithTimeout(ctx, 30*time.Second)
	defer listCancel()
	toolsResult, err := client.ListTools(listCtx, mcp.ListToolsRequest{})
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("list tools: %w", err)
	}

	descriptors := make([]ToolDescriptor, 0, len(toolsResult.Tools))
	for _, item := range toolsResult.Tools {
		schema := map[string]interface{}{}
		if item.InputSchema.Type != "" || len(item.InputSchema.Properties) > 0 {
			raw, marshalErr := json.Marshal(item.InputSchema)
			if marshalErr == nil {
				_ = json.Unmarshal(raw, &schema)
			}
		}
		desc := strings.TrimSpace(item.Description)
		if desc == "" {
			desc = fmt.Sprintf("MCP tool from server %s", server.Name)
		}
		descriptor := ToolDescriptor{
			FullName:    BuildToolFullName(server.Name, item.Name),
			ServerName:  NormalizeName(server.Name),
			ToolName:    item.Name,
			Description: desc,
			InputSchema: schema,
		}
		descriptors = append(descriptors, descriptor)
	}

	return &serverRuntime{
		server: server,
		client: client,
		tools:  descriptors,
	}, nil
}

func (m *Manager) ToolDescriptors() []ToolDescriptor {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ToolDescriptor, 0, len(m.toolsByName))
	for _, tool := range m.toolsByName {
		out = append(out, tool)
	}
	return out
}

func (m *Manager) ToolsForServer(id uint) []models.McpToolInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	runtime, ok := m.servers[id]
	m.mu.RUnlock()
	if !ok {
		return []models.McpToolInfo{}
	}
	out := make([]models.McpToolInfo, 0, len(runtime.tools))
	for _, tool := range runtime.tools {
		out = append(out, models.McpToolInfo{
			Name:        tool.ToolName,
			FullName:    tool.FullName,
			ServerName:  tool.ServerName,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return out
}

func (m *Manager) CallTool(ctx context.Context, fullName string, input map[string]interface{}) (string, error) {
	if m == nil {
		return "", fmt.Errorf("mcp manager not initialized")
	}
	serverNorm, toolName, ok := ParseToolFullName(fullName)
	if !ok {
		return "", fmt.Errorf("invalid mcp tool name: %s", fullName)
	}
	m.mu.RLock()
	var runtime *serverRuntime
	for _, item := range m.servers {
		if NormalizeName(item.server.Name) == serverNorm {
			runtime = item
			break
		}
	}
	m.mu.RUnlock()
	if runtime == nil || runtime.client == nil {
		return "", fmt.Errorf("mcp server for tool %s is not connected", fullName)
	}
	actualToolName := toolName
	for _, tool := range runtime.tools {
		if tool.FullName == fullName {
			actualToolName = tool.ToolName
			break
		}
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = actualToolName
	req.Params.Arguments = input
	result, err := runtime.client.CallTool(ctx, req)
	if err != nil {
		return "", err
	}
	return formatToolResult(result), nil
}

func formatToolResult(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	if result.IsError {
		return formatContentBlocks(result.Content)
	}
	text := formatContentBlocks(result.Content)
	if text == "" {
		raw, _ := json.MarshalIndent(result, "", "  ")
		return string(raw)
	}
	return text
}

func formatContentBlocks(blocks []mcp.Content) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch typed := block.(type) {
		case mcp.TextContent:
			if strings.TrimSpace(typed.Text) != "" {
				parts = append(parts, typed.Text)
			}
		default:
			raw, err := json.Marshal(typed)
			if err == nil {
				parts = append(parts, string(raw))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func parseStringArrayJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseStringMapJSON(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
