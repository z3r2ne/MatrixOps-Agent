package openai_native

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"pkgs/db/models"
	"pkgs/jsonextractor"

	"matrixops.local/core_agent/action_providers/compatible"
	"matrixops.local/core_agent/streamtypes"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)
func streamV2OpenAIChatCompletionsOnce(
	input streamtypes.StreamInput,
	llm *models.LLMConfig,
	model string,
	ctx context.Context,
	opts []option.RequestOption,
	transportState *openAINativeTransportState,
) (*streamtypes.StreamOutput, error) {
	client := openai.NewClient(opts...)
	params := openai.ChatCompletionNewParams{
		Messages: buildOpenAIChatCompletionMessages(input.SystemPrompt, input.Instruction, input.Prompt, input.UserContentParts, input.HistoryMessages),
		Model:    shared.ChatModel(model),
		Tools:    buildOpenAIChatCompletionTools(input.Tools),
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.NewOpt(true),
		},
	}
	if input.ParallelToolCalls {
		params.ParallelToolCalls = param.NewOpt(true)
	}
	if effort := strings.TrimSpace(input.ReasoningEffort); effort != "" {
		params.ReasoningEffort = shared.ReasoningEffort(effort)
	}
	if input.Temperature != 0 {
		params.Temperature = param.NewOpt(input.Temperature)
	}
	if input.TopP != 0 {
		params.TopP = param.NewOpt(input.TopP)
	}
	if input.MaxOutputTokens > 0 {
		params.MaxTokens = param.NewOpt(int64(input.MaxOutputTokens))
	}
	if extra := mergeNativeOpenAIThinkingExtras(input.ThinkingType, input.EnableThinking); extra != nil {
		params.SetExtraFields(extra)
	}
	requestOpts := append([]option.RequestOption{}, opts...)
	if cacheKey := strings.TrimSpace(input.PromptCacheKey); cacheKey != "" {
		requestOpts = append(requestOpts, option.WithJSONSet("prompt_cache_key", cacheKey))
	}

	stream := client.Chat.Completions.NewStreaming(ctx, params, requestOpts...)
	toolCalls := make(chan *streamtypes.CallToolRequest, 16)
	reasonReader, reasonWriter := jsonextractor.NewPipe()
	contentReader, contentWriter := jsonextractor.NewPipe()
	var contentBuf bytes.Buffer
	var (
		rawText          bytes.Buffer
		rawResponse      bytes.Buffer
		usage            *streamtypes.Usage
		waitErr          error
		actionIndex      int
		textMode         nativeOpenAITextMode
		textBuffer       bytes.Buffer
		sawToolCall      bool
		sawReasoningOutput bool
		answerState      *nativeOpenAIAnswerState
		toolStates       = map[string]*nativeOpenAIToolState{}
		streamDone       = make(chan struct{})
		streamEventCount int
	)

	go func() {
		defer close(streamDone)
		defer close(toolCalls)
		defer func() { _ = stream.Close() }()

		setErr := func(err error) {
			if err != nil && waitErr == nil {
				waitErr = err
			}
		}

		appendReasoningDelta := func(fragment string) {
			if fragment == "" {
				return
			}
			sawReasoningOutput = true
			if _, err := reasonWriter.Write([]byte(fragment)); err != nil {
				setErr(err)
			}
		}

		emitToolCall := func(name, callID string, reader io.Reader) *streamtypes.CallToolRequest {
			req := &streamtypes.CallToolRequest{
				Index:     actionIndex,
				CallID:    strings.TrimSpace(callID),
				Name:      strings.TrimSpace(name),
				Arguments: reader,
			}
			actionIndex++
			toolCalls <- req
			return req
		}
		toolKey := func(choiceIndex, toolIndex int64) string {
			return fmt.Sprintf("%d:%d", choiceIndex, toolIndex)
		}
		ensureToolState := func(choiceIndex, toolIndex int64, name, callID string) *nativeOpenAIToolState {
			key := toolKey(choiceIndex, toolIndex)
			state := toolStates[key]
			if state == nil {
				state = &nativeOpenAIToolState{
					itemID:      key,
					outputIndex: toolIndex,
					callID:      strings.TrimSpace(callID),
					name:        strings.TrimSpace(name),
					buffer:      streamtypes.NewStreamingActionBuffer(),
				}
				toolStates[key] = state
			}
			if strings.TrimSpace(name) != "" {
				state.name = strings.TrimSpace(name)
			}
			if strings.TrimSpace(callID) != "" {
				state.callID = strings.TrimSpace(callID)
			}
			if state.request == nil && strings.TrimSpace(state.name) != "" {
				state.request = emitToolCall(state.name, state.callID, state.buffer)
			}
			return state
		}
		finishAccumulatedToolCalls := func(choiceIndex int64, toolCalls []openai.ChatCompletionMessageToolCall) {
			for index, toolCall := range toolCalls {
				state := ensureToolState(choiceIndex, int64(index), toolCall.Function.Name, toolCall.ID)
				if err := state.syncFull(toolCall.Function.Arguments); err != nil {
					setErr(err)
				}
				if err := state.finish(); err != nil {
					setErr(err)
				}
			}
		}
		ensureAnswerState := func(outputIndex int64, itemID string) *nativeOpenAIAnswerState {
			if answerState != nil {
				return answerState
			}
			answerState = &nativeOpenAIAnswerState{
				itemID:      itemID,
				outputIndex: outputIndex,
			}
			return answerState
		}
		handleTextDelta := func(delta string, outputIndex int64, itemID string) {
			if delta == "" {
				return
			}
			rawText.WriteString(delta)
			contentBuf.WriteString(delta)
			if _, err := contentWriter.Write([]byte(delta)); err != nil {
				setErr(err)
			}
			if sawToolCall {
				return
			}
			textBuffer.WriteString(delta)
			switch textMode {
			case nativeOpenAITextModeUnknown:
				first := firstNonWhitespaceByte(textBuffer.Bytes())
				if first == 0 {
					return
				}
				if first == '{' {
					textMode = nativeOpenAITextModeJSON
					return
				}
				textMode = nativeOpenAITextModePlain
				state := ensureAnswerState(outputIndex, itemID)
				if err := state.write(textBuffer.String()); err != nil {
					setErr(err)
				}
				textBuffer.Reset()
			case nativeOpenAITextModePlain:
				state := ensureAnswerState(outputIndex, itemID)
				if err := state.write(delta); err != nil {
					setErr(err)
				}
			case nativeOpenAITextModeJSON:
			}
		}

		acc := openai.ChatCompletionAccumulator{}
		for stream.Next() {
			if input.Abort != nil {
				select {
				case <-input.Abort.Done():
					setErr(input.Abort.Err())
					goto finalize
				default:
				}
			}

			chunk := stream.Current()
			streamEventCount++
			if raw := strings.TrimSpace(chunk.RawJSON()); raw != "" {
				rawResponse.WriteString(raw)
				rawResponse.WriteByte('\n')
			}
			if !acc.AddChunk(chunk) {
				setErr(fmt.Errorf("openai native chat stream accumulator mismatch"))
			}
			if chunk.Usage.PromptTokens > 0 || chunk.Usage.TotalTokens > 0 {
				usage = chatCompletionUsageToCoreUsage(chunk.Usage)
			}

			for _, choice := range chunk.Choices {
				if rc := openAIJSONFieldReasoningContent(choice.Delta.RawJSON()); rc != "" {
					appendReasoningDelta(rc)
				}
				if choice.Delta.Content != "" {
					handleTextDelta(choice.Delta.Content, choice.Index, "")
				}
				if choice.Delta.Refusal != "" {
					handleTextDelta(choice.Delta.Refusal, choice.Index, "")
				}
				for _, toolCall := range choice.Delta.ToolCalls {
					sawToolCall = true
					state := ensureToolState(choice.Index, toolCall.Index, toolCall.Function.Name, toolCall.ID)
					if toolCall.Function.Arguments != "" {
						if err := state.write(toolCall.Function.Arguments); err != nil {
							setErr(err)
						}
					}
				}
				if choice.FinishReason == "tool_calls" || choice.FinishReason == "function_call" {
					if int(choice.Index) < len(acc.Choices) {
						finishAccumulatedToolCalls(choice.Index, acc.Choices[choice.Index].Message.ToolCalls)
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			setErr(err)
		}

	finalize:
		for _, state := range toolStates {
			if err := state.finish(); err != nil {
				setErr(err)
			}
		}
		if !sawToolCall {
			switch textMode {
			case nativeOpenAITextModePlain:
				if answerState != nil {
					if err := answerState.finish(); err != nil {
						setErr(err)
					}
				}
			case nativeOpenAITextModeJSON:
				payload := bytes.TrimSpace(textBuffer.Bytes())
				if len(payload) > 0 {
					parsed, err := streamtypes.ParseActionBytes(payload)
					if err == nil && len(parsed) > 0 {
					for _, action := range parsed {
						if action == nil {
							continue
						}
						action.Index = actionIndex
						actionIndex++
						if err := compatible.DispatchParsedAction(action, toolCalls, input.CompatibleControlHandler); err != nil {
							setErr(err)
						}
					}
					} else if streamtypes.StreamShouldRetryParseError(err) {
						setErr(fmt.Errorf("parse JSON stream: %w", err))
					} else {
						state := ensureAnswerState(0, "")
						if err := state.write(strings.TrimSpace(string(payload))); err != nil {
							setErr(err)
						}
						if err := state.finish(); err != nil {
							setErr(err)
						}
					}
				}
			case nativeOpenAITextModeUnknown:
				payload := strings.TrimSpace(textBuffer.String())
				if payload != "" {
					state := ensureAnswerState(0, "")
					if err := state.write(payload); err != nil {
						setErr(err)
					}
					if err := state.finish(); err != nil {
						setErr(err)
					}
				}
			}
		}
		if waitErr == nil && streamEventCount > 0 {
			hasOutput := sawToolCall || sawReasoningOutput || rawText.Len() > 0 || contentBuf.Len() > 0 || textBuffer.Len() > 0
			if answerState != nil && answerState.text.Len() > 0 {
				hasOutput = true
			}
			rawResp := strings.TrimSpace(rawResponse.String())
			if err := streamtypes.RetryErrorForEmptyStreamOutput("", rawResp, hasOutput); err != nil {
				setErr(err)
			}
		}
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
			if err := openAINativeNoEventStreamError(llm, transportState.responseStatus, transportState.responseHeaders, transportState.rawBodyResponse); err != nil {
				setErr(err)
			}
		}
		if streamEventCount == 0 {
			return
		}
	}()

	out := &streamtypes.StreamOutput{
		ToolCalls:     toolCalls,
		RawTextReader: &rawText,
		ReasonReader:  reasonReader,
		ContentReader: contentReader,
	}
	out.Wait = func() error {
		<-streamDone
		out.Usage = usage
		out.NativeAssistantTextFinishesTurn = !sawToolCall && contentBuf.Len() > 0
		return waitErr
	}
	return out, nil
}
