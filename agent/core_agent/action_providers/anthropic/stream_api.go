package anthropic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"pkgs/jsonextractor"

	providers "matrixops.local/core_agent/providers"
	"matrixops.local/core_agent/streamtypes"

	_ "embed"

	"github.com/anthropics/anthropic-sdk-go"
)

// StreamV2Anthropic 使用官方 anthropic-sdk-go 对 Messages API 做流式调用，并转换为 streamtypes 的 Action 通道。
func StreamV2Anthropic(input streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
	return streamtypes.StreamWithRetries(input, streamV2AnthropicOnce)
}

// StreamV2AnthropicOnce 单次流式请求，不含重试。
func StreamV2AnthropicOnce(input streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
	return streamV2AnthropicOnce(input)
}

func streamV2AnthropicOnce(input streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
	if input.Abort != nil {
		select {
		case <-input.Abort.Done():
			return nil, input.Abort.Err()
		default:
		}
	}

	llm := providers.NormalizeProviderOptions(input.ProviderOptions, input.Model)
	if llm == nil {
		return nil, fmt.Errorf("anthropic native tools: provider options missing (need LLMConfig or compatible)")
	}
	apiKey := strings.TrimSpace(llm.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic native tools: API key is empty")
	}
	modelStr := strings.TrimSpace(input.Model)
	if modelStr == "" {
		modelStr = strings.TrimSpace(llm.Model)
	}
	if modelStr == "" {
		return nil, fmt.Errorf("anthropic native tools: model is empty")
	}

	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}

	transportState, opts, err := buildAnthropicClientOptions(input, llm)
	if err != nil {
		return nil, err
	}

	maxTokens := int64(input.MaxOutputTokens)
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	params, err := buildAnthropicMessageParams(input, anthropic.Model(modelStr), maxTokens)
	if err != nil {
		return nil, err
	}

	client := anthropic.NewClient(opts...)
	stream := client.Messages.NewStreaming(ctx, params)
	if err := stream.Err(); err != nil {
		return nil, mapAnthropicErrToProvider(llm, err)
	}

	toolCalls := make(chan *streamtypes.CallToolRequest, 16)
	reasonReader, reasonWriter := jsonextractor.NewPipe()
	var contentBuf bytes.Buffer
	var sawToolUse bool
	var sawReasoningOutput bool
	var (
		rawText          bytes.Buffer
		rawResponse      bytes.Buffer
		usage            *streamtypes.Usage
		waitErr          error
		actionIndex      int
		streamDone       = make(chan struct{})
		streamEventCount int
		lastStopReason   string
	)

	type textState struct {
		text     bytes.Buffer
		finished bool
	}
	type toolState struct {
		buffer   *streamtypes.StreamingActionBuffer
		request  *streamtypes.CallToolRequest
		args     bytes.Buffer
		name     string
		finished bool
	}
	type anthropicThinkingAccum struct {
		signature strings.Builder
		thinking  strings.Builder
	}
	contentReader, contentWriter := jsonextractor.NewPipe()

	out := &streamtypes.StreamOutput{
		ToolCalls: toolCalls,
		RawTextReader: &rawText,
		ReasonReader:  reasonReader,
		ContentReader: contentReader,
	}
	out.Wait = func() error {
		<-streamDone
		out.Usage = usage
		// 无 tools 的纯文本场景（记忆压缩、标题生成等）同样可能以正文结束一轮。
		out.NativeAssistantTextFinishesTurn = !sawToolUse && contentBuf.Len() > 0
		return waitErr
	}

	go func() {
		defer close(streamDone)
		defer close(toolCalls)
		defer func() { _ = stream.Close() }()

		setErr := func(err error) {
			if err != nil && waitErr == nil {
				waitErr = mapAnthropicErrToProvider(llm, err)
			}
		}

		appendReason := func(fragment string) {
			if fragment == "" {
				return
			}
			sawReasoningOutput = true
			if _, err := reasonWriter.Write([]byte(fragment)); err != nil {
				setErr(err)
			}
		}

		blocks := make(map[int64]any)

		emitToolCall := func(name string, reader io.Reader) *streamtypes.CallToolRequest {
			req := &streamtypes.CallToolRequest{
				Index:     actionIndex,
				Name:      strings.TrimSpace(name),
				Arguments: reader,
			}
			actionIndex++
			toolCalls <- req
			return req
		}

		finishText := func(st *textState) error {
			if st == nil || st.finished {
				return nil
			}
			st.finished = true
			return nil
		}

		promoteAndFinishTexts := func() {
			for idx, v := range blocks {
				st, ok := v.(*textState)
				if !ok || st == nil || st.finished {
					continue
				}
				if err := finishText(st); err != nil {
					setErr(err)
				}
				delete(blocks, idx)
			}
		}

		finishTool := func(st *toolState) error {
			if st == nil || st.finished {
				return nil
			}
			st.finished = true
			if st.request != nil {
				st.request.RawJSON = fmt.Sprintf(`{"@action":"call_tool","data":{"name":%q,"params":%s}}`, st.name, strings.TrimSpace(st.args.String()))
			}
			if st.buffer != nil {
				return st.buffer.Close()
			}
			return nil
		}

		finishAllBlocks := func() {
			for idx, v := range blocks {
				switch st := v.(type) {
				case *textState:
					if st != nil && !st.finished {
						_ = finishText(st)
					}
				case *toolState:
					_ = finishTool(st)
				case *anthropicThinkingAccum:
					if st != nil {
						out.SetAnthropicThinkingSignature(st.signature.String())
					}
				}
				delete(blocks, idx)
			}
		}

		for stream.Next() {
			if input.Abort != nil {
				select {
				case <-input.Abort.Done():
					setErr(input.Abort.Err())
					goto finalize
				default:
				}
			}

			ev := stream.Current()
			streamEventCount++
			if raw := strings.TrimSpace(ev.RawJSON()); raw != "" {
				rawResponse.WriteString(raw)
				rawResponse.WriteByte('\n')
			}

			switch variant := ev.AsAny().(type) {
			case anthropic.MessageDeltaEvent:
				if u := messageDeltaUsageToStreamUsage(variant.Usage); u != nil {
					usage = u
				}
				if sr := strings.TrimSpace(string(variant.Delta.StopReason)); sr != "" {
					lastStopReason = sr
				}
			case anthropic.MessageStopEvent:
				// message_stop 才表示整条 assistant message 结束。
				// content_block_stop 只能说明某个 block（text/thinking/tool_use）结束，
				// 不能安全地作为聚合后的 ReasonReader / ContentReader 的 EOF；
				// 同一条消息里后面仍可能继续出现新的 text/thinking block。
				goto finalize
			case anthropic.ContentBlockStartEvent:
				idx := variant.Index
				switch cb := variant.ContentBlock.AsAny().(type) {
				case anthropic.TextBlock:
					blocks[idx] = &textState{}
				case anthropic.ToolUseBlock:
					sawToolUse = true
					promoteAndFinishTexts()
					st := &toolState{
						buffer: streamtypes.NewStreamingActionBuffer(),
						name:   strings.TrimSpace(cb.Name),
					}
					st.request = emitToolCall(st.name, st.buffer)
					blocks[idx] = st
				case anthropic.ThinkingBlock:
					blocks[idx] = &anthropicThinkingAccum{}
				default:
				}
			case anthropic.ContentBlockDeltaEvent:
				deltaEv := variant
				idx := deltaEv.Index
				switch d := deltaEv.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					if st, ok := blocks[idx].(*textState); ok && st != nil && d.Text != "" {
						rawText.WriteString(d.Text)
						contentBuf.WriteString(d.Text)
						if _, err := contentWriter.Write([]byte(d.Text)); err != nil {
							setErr(err)
						}
						st.text.WriteString(d.Text)
					}
				case anthropic.InputJSONDelta:
					if st, ok := blocks[idx].(*toolState); ok && st != nil && d.PartialJSON != "" {
						st.args.WriteString(d.PartialJSON)
						if _, err := st.buffer.Write([]byte(d.PartialJSON)); err != nil {
							setErr(err)
						}
					}
				case anthropic.ThinkingDelta:
					if st, ok := blocks[idx].(*anthropicThinkingAccum); ok && st != nil && d.Thinking != "" {
						st.thinking.WriteString(d.Thinking)
					}
					appendReason(d.Thinking)
				case anthropic.SignatureDelta:
					if st, ok := blocks[idx].(*anthropicThinkingAccum); ok && st != nil && d.Signature != "" {
						st.signature.WriteString(d.Signature)
					}
				}
			case anthropic.ContentBlockStopEvent:
				idx := variant.Index
				switch st := blocks[idx].(type) {
				case *textState:
					if err := finishText(st); err != nil {
						setErr(err)
					}
				case *toolState:
					if err := finishTool(st); err != nil {
						setErr(err)
					}
				case *anthropicThinkingAccum:
					out.SetAnthropicThinkingSignature(st.signature.String())
				default:
				}
				// 这里只结束当前 block 的内部状态，不结束聚合后的 reason/content 流：
				// Anthropic 一条消息中允许多个 text/thinking block（例如 tool_use 前后文本、
				// 以及 interleaved thinking）。
				delete(blocks, idx)
			default:
			}
		}
		if err := stream.Err(); err != nil {
			setErr(mapAnthropicErrToProvider(llm, err))
		}

	finalize:
		finishAllBlocks()
		if waitErr != nil {
			_ = reasonWriter.CloseWithError(waitErr)
			_ = contentWriter.CloseWithError(waitErr)
		} else {
			_ = reasonWriter.Close()
			_ = contentWriter.Close()
		}
		if input.OnRawResponse != nil && rawResponse.Len() > 0 {
			input.OnRawResponse(strings.TrimSpace(rawResponse.String()))
		}
		if waitErr == nil && streamEventCount == 0 {
			if err := anthropicNoEventStreamError(llm, transportState.responseStatus, transportState.responseHeaders, "", transportState.rawBodyResponse); err != nil {
				setErr(err)
			}
		}
		if waitErr == nil && streamEventCount > 0 {
			hasOutput := sawToolUse || sawReasoningOutput || rawText.Len() > 0 || contentBuf.Len() > 0
			rawResp := strings.TrimSpace(rawResponse.String())
			if err := streamtypes.RetryErrorForEmptyStreamOutput(lastStopReason, rawResp, hasOutput); err != nil {
				setErr(err)
			}
		}
	}()

	return out, nil
}
