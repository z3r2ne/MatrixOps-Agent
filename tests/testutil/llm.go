package testutil

import (
	"fmt"

	llm "matrixops-agent/llm"
)

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

// FirstSystemMessageContent 从 ChatRequest 中提取首条 system 消息文本。
func FirstSystemMessageContent(req *llm.ChatRequest) string {
	if req == nil {
		return ""
	}
	for _, msg := range req.Messages {
		if msg != nil && msg.Role == "system" {
			return MessageContentString(msg.Content)
		}
	}
	if len(req.Messages) > 0 && req.Messages[0] != nil {
		return MessageContentString(req.Messages[0].Content)
	}
	return ""
}

// UserMessageContent 从 ChatRequest 中提取 user 消息文本。
func UserMessageContent(req *llm.ChatRequest) string {
	if req == nil {
		return ""
	}
	for _, msg := range req.Messages {
		if msg != nil && msg.Role == "user" {
			return MessageContentString(msg.Content)
		}
	}
	if len(req.Messages) > 1 && req.Messages[1] != nil {
		return MessageContentString(req.Messages[1].Content)
	}
	return ""
}

// MessageContentString 将 message content 转为字符串。
func MessageContentString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// FindChatRequestWithUserInput 在多次 LLM 请求中查找包含指定用户输入的请求。
func FindChatRequestWithUserInput(requests []llm.ChatRequest, inputText string) *llm.ChatRequest {
	for i := range requests {
		if UserMessageContent(&requests[i]) == inputText {
			return &requests[i]
		}
	}
	return nil
}
