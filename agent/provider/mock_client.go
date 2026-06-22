package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"matrixops-agent/llm"
)

// MockClient 模拟 LLM 客户端，用于测试
type MockClient struct {
	// Responses 预设的响应列表，按顺序返回
	Responses []MockResponse
	// CallCount 记录被调用的次数
	CallCount int
	// LastRequest 记录最后一次请求
	LastRequest *llm.ChatRequest
	// AllRequests 记录所有请求
	AllRequests    []*llm.ChatRequest
	StreamCallBack func(request llm.ChatRequest) (<-chan llm.StreamEvent, error)
}

// MockResponse 模拟的响应
type MockResponse struct {
	// Text 返回的文本内容
	Text string
	// ToolCalls 返回的工具调用
	ToolCalls []llm.ToolCall
	// Finish 完成原因
	Finish string
	// Error 是否返回错误
	Error error
	// Usage token 使用情况
	Usage *llm.Usage
}

// NewMockClient 创建一个新的 Mock 客户端
func NewMockClient(responses ...MockResponse) *MockClient {
	if len(responses) == 0 {
		// 默认响应
		responses = []MockResponse{
			{
				Text:   "This is a mock response.",
				Finish: "stop",
				Usage:  &llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
		}
	}
	return &MockClient{
		Responses:   responses,
		AllRequests: make([]*llm.ChatRequest, 0),
	}
}

func NewMockClientWithStreamCallback(streamCallback func(request llm.ChatRequest) (<-chan llm.StreamEvent, error)) *MockClient {
	return &MockClient{
		StreamCallBack: streamCallback,
	}
}

// NewMockClientWithText 创建一个返回简单文本的 Mock 客户端
func NewMockClientWithText(text string) *MockClient {
	return NewMockClient(MockResponse{
		Text:   text,
		Finish: "stop",
		Usage:  &llm.Usage{InputTokens: 100, OutputTokens: 50},
	})
}

// NewMockClientWithToolCall 创建一个返回工具调用的 Mock 客户端
func NewMockClientWithToolCall(toolName string, args map[string]interface{}) *MockClient {
	return NewMockClient(MockResponse{
		ToolCalls: []llm.ToolCall{
			{
				ID:        "call_1",
				Name:      toolName,
				Arguments: args,
			},
		},
		Finish: "tool_calls",
		Usage:  &llm.Usage{InputTokens: 100, OutputTokens: 50},
	})
}

// Chat 实现 llm.ChatClient 接口
func (m *MockClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	// 记录请求
	m.LastRequest = &request
	m.AllRequests = append(m.AllRequests, &request)

	// 获取响应
	response := m.getResponse()
	m.CallCount++

	if response.Error != nil {
		return llm.ChatResponse{}, response.Error
	}

	return llm.ChatResponse{
		Message: llm.ModelMessage{
			Role:    "assistant",
			Content: response.Text,
		},
		ToolCalls: response.ToolCalls,
		Finish:    response.Finish,
		Usage:     response.Usage,
	}, nil
}

// StreamChat 实现 llm.StreamChatClient 接口
func (m *MockClient) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	m.LastRequest = &request
	m.AllRequests = append(m.AllRequests, &request)
	if m.StreamCallBack != nil {
		return m.StreamCallBack(request)
	}

	// 获取响应
	response := m.getResponse()
	m.CallCount++

	if response.Error != nil {
		return nil, response.Error
	}

	// 创建事件通道
	events := make(chan llm.StreamEvent, 10)

	go func() {
		defer close(events)

		// 发送文本增量事件
		if response.Text != "" {
			// 模拟分块发送
			chunkSize := 10
			for i := 0; i < len(response.Text); i += chunkSize {
				end := i + chunkSize
				if end > len(response.Text) {
					end = len(response.Text)
				}
				events <- llm.StreamEvent{
					Type: "text-delta",
					Text: response.Text[i:end],
				}
			}
		}

		// 发送工具调用事件
		for i, toolCall := range response.ToolCalls {
			argsJSON, _ := json.Marshal(toolCall.Arguments)
			events <- llm.StreamEvent{
				Type:          "tool-delta",
				ToolIndex:     i,
				ToolCallID:    toolCall.ID,
				ToolName:      toolCall.Name,
				ToolArguments: string(argsJSON),
			}
		}

		// 发送完成事件
		events <- llm.StreamEvent{
			Type:   "finish",
			Finish: response.Finish,
			Usage:  response.Usage,
		}
	}()

	return events, nil
}

// StreamChatWithOptions 实现 llm.StreamChatClientWithOptions 接口
func (m *MockClient) StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	// 直接调用 StreamChat，忽略选项
	return m.StreamChat(request)
}

// getResponse 获取当前应该返回的响应
func (m *MockClient) getResponse() MockResponse {
	if m.CallCount >= len(m.Responses) {
		// 如果超出预设响应数量，返回最后一个
		return m.Responses[len(m.Responses)-1]
	}
	return m.Responses[m.CallCount]
}

// Reset 重置 Mock 客户端状态
func (m *MockClient) Reset() {
	m.CallCount = 0
	m.LastRequest = nil
	m.AllRequests = make([]*llm.ChatRequest, 0)
}

// GetSystemPrompt 获取最后一次请求中的系统提示词
func (m *MockClient) GetSystemPrompt() string {
	if m.LastRequest == nil {
		return ""
	}

	for _, msg := range m.LastRequest.Messages {
		if msg.Role == "system" {
			if text, ok := msg.Content.(string); ok {
				return text
			}
		}
	}
	return ""
}

// GetAllSystemPrompts 获取最后一次请求中的所有系统提示词
func (m *MockClient) GetAllSystemPrompts() []string {
	if m.LastRequest == nil {
		return nil
	}

	prompts := make([]string, 0)
	for _, msg := range m.LastRequest.Messages {
		if msg.Role == "system" {
			if text, ok := msg.Content.(string); ok {
				prompts = append(prompts, text)
			}
		}
	}
	return prompts
}

// ContainsInSystemPrompt 检查系统提示词中是否包含指定文本
func (m *MockClient) ContainsInSystemPrompt(text string) bool {
	systemPrompt := m.GetSystemPrompt()
	return strings.Contains(systemPrompt, text)
}

// ContainsInAnySystemPrompt 检查任何系统提示词中是否包含指定文本
func (m *MockClient) ContainsInAnySystemPrompt(text string) bool {
	prompts := m.GetAllSystemPrompts()
	for _, prompt := range prompts {
		if strings.Contains(prompt, text) {
			return true
		}
	}
	return false
}

// GetLastUserMessage 获取最后一次请求中的用户消息
func (m *MockClient) GetLastUserMessage() string {
	if m.LastRequest == nil {
		return ""
	}
	return LastUserMessageContent(m.LastRequest)
}

// FirstUserMessageContent 返回请求中第一条 user 消息文本。
func FirstUserMessageContent(req *llm.ChatRequest) string {
	if req == nil {
		return ""
	}
	for _, msg := range req.Messages {
		if msg != nil && msg.Role == "user" {
			return messageContentString(msg.Content)
		}
	}
	return ""
}

// LastUserMessageContent 返回请求中最后一条 user 消息文本。
func LastUserMessageContent(req *llm.ChatRequest) string {
	if req == nil {
		return ""
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg != nil && msg.Role == "user" {
			return messageContentString(msg.Content)
		}
	}
	return ""
}

// FirstSystemMessageContent 返回请求中第一条 system 消息文本。
func FirstSystemMessageContent(req *llm.ChatRequest) string {
	if req == nil {
		return ""
	}
	for _, msg := range req.Messages {
		if msg != nil && msg.Role == "system" {
			return messageContentString(msg.Content)
		}
	}
	return ""
}

// FindRequestWithUserInput 在已记录的请求中查找 user 消息等于 inputText 的请求。
func (m *MockClient) FindRequestWithUserInput(inputText string) *llm.ChatRequest {
	inputText = strings.TrimSpace(inputText)
	for _, req := range m.AllRequests {
		if req == nil {
			continue
		}
		if strings.TrimSpace(FirstUserMessageContent(req)) == inputText {
			return req
		}
	}
	return nil
}

func messageContentString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// MockAnswerActionStream 返回符合 StreamV2 解析器的 answer action 流。
func MockAnswerActionStream(answer string) <-chan llm.StreamEvent {
	stream := make(chan llm.StreamEvent, 2)
	go func() {
		defer close(stream)
		stream <- llm.StreamEvent{
			Type: string(llm.GeneratorMessageTypeTextDelta),
			Text: fmt.Sprintf(`{"@action":"answer","data":%q}`, answer),
		}
		stream <- llm.StreamEvent{
			Type:  string(llm.GeneratorMessageTypeFinish),
			Usage: &llm.Usage{InputTokens: 1, OutputTokens: 1},
		}
	}()
	return stream
}

// GetToolDefinitions 获取请求中的工具定义
func (m *MockClient) GetToolDefinitions() []llm.ToolDefinition {
	if m.LastRequest == nil {
		return nil
	}
	return m.LastRequest.Tools
}

// HasTool 检查请求中是否包含指定工具
func (m *MockClient) HasTool(toolName string) bool {
	tools := m.GetToolDefinitions()
	for _, tool := range tools {
		if tool.Name == toolName {
			return true
		}
	}
	return false
}

// DebugLastRequest 输出最后一次请求的详细信息（用于调试）
func (m *MockClient) DebugLastRequest() string {
	if m.LastRequest == nil {
		return "No request recorded"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Last Request (Call #%d) ===\n", m.CallCount))
	sb.WriteString(fmt.Sprintf("Model: %v\n", m.LastRequest.ProviderOptions.Model))
	sb.WriteString(fmt.Sprintf("Temperature: %.2f\n", m.LastRequest.Temperature))
	sb.WriteString(fmt.Sprintf("Messages: %d\n", len(m.LastRequest.Messages)))

	for i, msg := range m.LastRequest.Messages {
		sb.WriteString(fmt.Sprintf("\nMessage %d [%s]:\n", i+1, msg.Role))
		if text, ok := msg.Content.(string); ok {
			if len(text) > 200 {
				sb.WriteString(text[:200] + "...\n")
			} else {
				sb.WriteString(text + "\n")
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\nTools: %d\n", len(m.LastRequest.Tools)))
	for _, tool := range m.LastRequest.Tools {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", tool.Name, tool.Description))
	}

	return sb.String()
}
