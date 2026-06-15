package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"matrixops-agent/llm"
	"matrixops-agent/plugin"
	"matrixops-agent/types"
	"pkgs/db/models"
)

type StreamInput struct {
	SessionID       string
	Context         context.Context
	Model           string
	Messages        []*llm.ModelMessage
	Tools           []llm.ToolDefinition
	Abort           context.Context
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions *models.LLMConfig
	HTTPClient      *http.Client
	PluginManager   *plugin.Manager
}

type StreamOutput struct {
	Text      string
	ToolCalls []llm.ToolCall
	Finish    string
	Usage     *llm.Usage
}

func Stream(input StreamInput, client llm.ChatClient, onEvent func(llm.StreamEvent)) (StreamOutput, error) {
	if input.Abort != nil {
		select {
		case <-input.Abort.Done():
			return StreamOutput{}, input.Abort.Err()
		default:
		}
	}
	tools := input.Tools

	req := llm.ChatRequest{
		Context:         input.Context,
		Messages:        input.Messages,
		Tools:           tools,
		Temperature:     input.Temperature,
		TopP:            input.TopP,
		MaxOutputTokens: input.MaxOutputTokens,
		ProviderOptions: input.ProviderOptions,
		Model:           input.Model,
	}

	parser := newReasoningParser()
	textBuilder := strings.Builder{}

	if streamer, ok := client.(llm.StreamChatClientWithOptions); ok {
		streamOpts := []llm.StreamChatOption{llm.WithOnRequest(func(request *llm.ChatRequest) error {
			if input.PluginManager == nil {
				return nil
			}
			return input.PluginManager.ApplyLLMRequest(&plugin.LLMRequest{
				Kind:     plugin.LLMRequestKindChat,
				Chat:     request,
				Generate: nil,
			})
		})}
		if input.HTTPClient != nil {
			streamOpts = append(streamOpts, llm.WithHTTPClient(input.HTTPClient))
		}
		stream, err := streamer.StreamChatWithOptions(req, streamOpts...)
		if err != nil {
			return StreamOutput{}, fmt.Errorf("stream chat with options: %w", err)
		}
		toolBuilders := map[int]*toolCallBuilder{}
		var finish string
		var usage *llm.Usage
		for {
			var (
				event llm.StreamEvent
				ok    bool
			)

			if input.Abort != nil {
				select {
				case <-input.Abort.Done():
					return StreamOutput{}, input.Abort.Err()
				case event, ok = <-stream:
				}
			} else {
				event, ok = <-stream
			}

			if !ok {
				break
			}

			if event.Type == "error" {
				return StreamOutput{}, event.Error
			}
			switch event.Type {
			case string(llm.GeneratorMessageTypeTextDelta):
				segments := parser.feed(event.Text)
				emitSegments(segments, &textBuilder, onEvent)
			case string(llm.GeneratorMessageTypeReasoningDelta):
				if onEvent != nil {
					onEvent(event)
				}
			case string(llm.GeneratorMessageTypeToolDelta):
				builder := toolBuilders[event.ToolIndex]
				if builder == nil {
					builder = &toolCallBuilder{}
					toolBuilders[event.ToolIndex] = builder
				}
				if event.ToolCallID != "" {
					builder.ID = event.ToolCallID
				}
				if event.ToolName != "" {
					builder.Name = event.ToolName
				}
				if event.ToolArguments != "" {
					builder.Arguments.WriteString(event.ToolArguments)
				}
				if onEvent != nil {
					onEvent(event)
				}
			case string(llm.GeneratorMessageTypeFinish):
				finish = event.Finish
				if event.Usage != nil {
					usage = event.Usage
				}
			}
		}
		emitSegments(parser.flush(), &textBuilder, onEvent)
		return StreamOutput{
			Text:      textBuilder.String(),
			ToolCalls: buildToolCalls(toolBuilders),
			Finish:    finish,
			Usage:     usage,
		}, nil
	}

	response, err := client.Chat(req)
	if err != nil {
		return StreamOutput{}, err
	}
	segments := parser.feed(renderContent(response.Message.Content))
	segments = append(segments, parser.flush()...)
	emitSegments(segments, &textBuilder, onEvent)
	return StreamOutput{
		Text:      textBuilder.String(),
		ToolCalls: response.ToolCalls,
		Finish:    response.Finish,
		Usage:     response.Usage,
	}, nil
}

func renderContent(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	default:
		return ""
	}
}

func HasToolCalls(messages []llm.ModelMessage) bool {
	for _, msg := range messages {
		if msg.Role == "tool" {
			return true
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

type toolCallBuilder struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

type reasoningSegment struct {
	kind string
	text string
}

type reasoningParser struct {
	inReasoning bool
	buffer      string
}

func newReasoningParser() *reasoningParser {
	return &reasoningParser{}
}

func (p *reasoningParser) feed(text string) []reasoningSegment {
	p.buffer += text
	return p.consume(false)
}

func (p *reasoningParser) flush() []reasoningSegment {
	return p.consume(true)
}

func (p *reasoningParser) consume(flush bool) []reasoningSegment {
	segments := []reasoningSegment{}
	startTag := "<think>"
	endTag := "</think>"
	for {
		if p.inReasoning {
			index := strings.Index(p.buffer, endTag)
			if index == -1 {
				if flush {
					if p.buffer != "" {
						segments = append(segments, reasoningSegment{kind: "reasoning", text: p.buffer})
					}
					p.buffer = ""
				} else {
					segments, p.buffer = splitBuffer(p.buffer, endTag, "reasoning", segments)
				}
				break
			}
			if index > 0 {
				segments = append(segments, reasoningSegment{kind: "reasoning", text: p.buffer[:index]})
			}
			p.buffer = p.buffer[index+len(endTag):]
			p.inReasoning = false
			continue
		}
		index := strings.Index(p.buffer, startTag)
		if index == -1 {
			if flush {
				if p.buffer != "" {
					segments = append(segments, reasoningSegment{kind: "text", text: p.buffer})
				}
				p.buffer = ""
			} else {
				segments, p.buffer = splitBuffer(p.buffer, startTag, "text", segments)
			}
			break
		}
		if index > 0 {
			segments = append(segments, reasoningSegment{kind: "text", text: p.buffer[:index]})
		}
		p.buffer = p.buffer[index+len(startTag):]
		p.inReasoning = true
	}
	return segments
}

func splitBuffer(buffer string, tag string, kind string, segments []reasoningSegment) ([]reasoningSegment, string) {
	minCarry := len(tag) - 1
	if minCarry < 1 || len(buffer) <= minCarry {
		return segments, buffer
	}
	safe := len(buffer) - minCarry
	segments = append(segments, reasoningSegment{kind: kind, text: buffer[:safe]})
	return segments, buffer[safe:]
}

func emitSegments(segments []reasoningSegment, textBuilder *strings.Builder, onEvent func(llm.StreamEvent)) {
	for _, segment := range segments {
		if segment.text == "" {
			continue
		}
		switch segment.kind {
		case "text":
			textBuilder.WriteString(segment.text)
			if onEvent != nil {
				onEvent(llm.StreamEvent{Type: types.PartTypeTextDelta, Text: segment.text})
			}
		case "reasoning":
			if onEvent != nil {
				onEvent(llm.StreamEvent{Type: types.PartTypeReasoningDelta, Text: segment.text})
			}
		}
	}
}

func buildToolCalls(builders map[int]*toolCallBuilder) []llm.ToolCall {
	if len(builders) == 0 {
		return nil
	}
	indices := make([]int, 0, len(builders))
	for index := range builders {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	calls := make([]llm.ToolCall, 0, len(indices))
	for _, index := range indices {
		builder := builders[index]
		if builder == nil {
			continue
		}
		args := map[string]interface{}{}
		raw := builder.Arguments.String()
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &args)
		}
		calls = append(calls, llm.ToolCall{
			ID:        builder.ID,
			Name:      builder.Name,
			Arguments: args,
		})
	}
	return calls
}
