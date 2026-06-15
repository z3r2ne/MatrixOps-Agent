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
	openairesponses "github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)
func streamV2OpenAIResponsesOnce(
	input streamtypes.StreamInput,
	llm *models.LLMConfig,
	model string,
	ctx context.Context,
	opts []option.RequestOption,
	transportState *openAINativeTransportState,
) (*streamtypes.StreamOutput, error) {
	client := openai.NewClient(opts...)
	params := openairesponses.ResponseNewParams{
		Input: buildOpenAIResponsesInput(input.SystemPrompt, input.Prompt, input.UserContentParts, input.HistoryMessages),
		Model: shared.ResponsesModel(model),
		Tools: buildOpenAIResponsesTools(input.Tools),
	}
	if input.EnableEncryptedReasoning {
		params.Include = []openairesponses.ResponseIncludable{
			openairesponses.ResponseIncludableReasoningEncryptedContent,
		}
		params.Store = param.NewOpt(false)
	}
	if input.ParallelToolCalls {
		params.ParallelToolCalls = param.NewOpt(true)
	}
	if cacheKey := strings.TrimSpace(input.PromptCacheKey); cacheKey != "" {
		params.PromptCacheKey = param.NewOpt(cacheKey)
	}
	if effort := strings.TrimSpace(input.ReasoningEffort); effort != "" {
		params.Reasoning = shared.ReasoningParam{
			Effort: shared.ReasoningEffort(effort),
		}
	}
	if verbosity := strings.TrimSpace(input.TextVerbosity); verbosity != "" {
		text := openairesponses.ResponseTextConfigParam{}
		text.SetExtraFields(map[string]any{
			"verbosity": verbosity,
		})
		params.Text = text
	}
	if strings.TrimSpace(input.Instruction) != "" {
		params.Instructions = param.NewOpt(strings.TrimSpace(input.Instruction))
	}
	// if input.Temperature != 0 {
	// 	params.Temperature = param.NewOpt(input.Temperature)
	// }
	if input.TopP != 0 {
		params.TopP = param.NewOpt(input.TopP)
	}
	if extra := mergeNativeOpenAIThinkingExtras(input.ThinkingType, input.EnableThinking); extra != nil {
		params.SetExtraFields(extra)
	}

	requestOpts := append([]option.RequestOption{}, opts...)
	stream := client.Responses.NewStreaming(ctx, params, requestOpts...)
	toolCalls := make(chan *streamtypes.CallToolRequest, 16)
	reasonReader, reasonWriter := jsonextractor.NewPipe()
	contentReader, contentWriter := jsonextractor.NewPipe()
	var contentBuf bytes.Buffer
	var (
		rawText                  bytes.Buffer
		rawResponse              bytes.Buffer
		usage                    *streamtypes.Usage
		waitErr                  error
		actionIndex              int
		textMode                 nativeOpenAITextMode
		textBuffer               bytes.Buffer
		sawToolCall              bool
		sawReasoningOutput       bool
		answerState              *nativeOpenAIAnswerState
		toolStates               = map[string]*nativeOpenAIToolState{}
		pendingReasoningItemRaws []string
		streamDone               = make(chan struct{})
		streamEventCount         int
		rawEventCount            int
		textDeltaCount           int
		toolDeltaCount           int
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
		ensureToolState := func(itemID string, outputIndex int64, name, callID, arguments string) *nativeOpenAIToolState {
			state := toolStates[itemID]
			if state == nil {
				state = &nativeOpenAIToolState{
					itemID:      itemID,
					outputIndex: outputIndex,
					callID:      strings.TrimSpace(callID),
					name:        strings.TrimSpace(name),
					buffer:      streamtypes.NewStreamingActionBuffer(),
				}
				toolStates[itemID] = state
				if len(pendingReasoningItemRaws) > 0 {
					state.responsesReasoningItemRaws = append([]string(nil), pendingReasoningItemRaws...)
					pendingReasoningItemRaws = nil
				}
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
			if strings.TrimSpace(arguments) != "" {
				if err := state.syncFull(arguments); err != nil {
					setErr(err)
				}
			}
			return state
		}
		ensureAnswerState := func(outputIndex int64, itemID string) *nativeOpenAIAnswerState {
			if answerState != nil {
				return answerState
			}
			answerState = &nativeOpenAIAnswerState{
				itemID:      itemID,
				outputIndex: outputIndex,
			}
			if len(pendingReasoningItemRaws) > 0 {
				answerState.responsesReasoningItemRaws = append([]string(nil), pendingReasoningItemRaws...)
				pendingReasoningItemRaws = nil
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

		for stream.Next() {
			if input.Abort != nil {
				select {
				case <-input.Abort.Done():
					setErr(input.Abort.Err())
					goto finalize
				default:
				}
			}
			event := stream.Current()
			streamEventCount++
			if raw := strings.TrimSpace(event.RawJSON()); raw != "" {
				rawEventCount++
				rawResponse.WriteString(raw)
				rawResponse.WriteByte('\n')
			}
			switch e := event.AsAny().(type) {
			case openairesponses.ResponseOutputItemAddedEvent:
				switch e.Item.Type {
				case "function_call":
					sawToolCall = true
					ensureToolState(e.Item.ID, e.OutputIndex, e.Item.Name, e.Item.CallID, e.Item.Arguments)
				}
			case openairesponses.ResponseOutputItemDoneEvent:
				if e.Item.Type == "reasoning" {
					if raw := strings.TrimSpace(e.Item.AsReasoning().RawJSON()); raw != "" {
						pendingReasoningItemRaws = append(pendingReasoningItemRaws, raw)
					}
				}
				if e.Item.Type == "message" {
					raw := strings.TrimSpace(e.Item.AsMessage().RawJSON())
					phase, text := parseOpenAIResponsesOutputMessage(raw)
					if answerState == nil && text != "" && !sawToolCall {
						state := ensureAnswerState(e.OutputIndex, e.Item.ID)
						if textMode == nativeOpenAITextModeUnknown {
							textMode = nativeOpenAITextModePlain
						}
						if state.text.Len() == 0 {
							rawText.WriteString(text)
							contentBuf.WriteString(text)
							if _, err := contentWriter.Write([]byte(text)); err != nil {
								setErr(err)
							}
							if err := state.write(text); err != nil {
								setErr(err)
							}
						}
					}
					if answerState != nil {
						answerState.phase = phase
						answerState.responsesOutputMessageRaw = raw
						if len(answerState.responsesReasoningItemRaws) == 0 && len(pendingReasoningItemRaws) > 0 {
							answerState.responsesReasoningItemRaws = append([]string(nil), pendingReasoningItemRaws...)
							pendingReasoningItemRaws = nil
						}
					}
				}
				if e.Item.Type == "function_call" {
					state := ensureToolState(e.Item.ID, e.OutputIndex, e.Item.Name, e.Item.CallID, e.Item.Arguments)
					if err := state.finish(); err != nil {
						setErr(err)
					}
				}
			case openairesponses.ResponseFunctionCallArgumentsDeltaEvent:
				toolDeltaCount++
				state := ensureToolState(e.ItemID, e.OutputIndex, "", "", "")
				if err := state.write(e.Delta); err != nil {
					setErr(err)
				}
			case openairesponses.ResponseFunctionCallArgumentsDoneEvent:
				state := ensureToolState(e.ItemID, e.OutputIndex, "", "", e.Arguments)
				if err := state.finish(); err != nil {
					setErr(err)
				}
			case openairesponses.ResponseTextDeltaEvent:
				textDeltaCount++
				handleTextDelta(e.Delta, e.OutputIndex, e.ItemID)
			case openairesponses.ResponseRefusalDeltaEvent:
				textDeltaCount++
				handleTextDelta(e.Delta, 0, "")
			case openairesponses.ResponseCompletedEvent:
				usage = responseUsageToCoreUsage(e.Response.Usage)
				if !sawToolCall && rawText.Len() == 0 {
					if text := strings.TrimSpace(e.Response.OutputText()); text != "" {
						handleTextDelta(text, 0, "")
					}
				}
			default:
				if rc := openAIJSONFieldReasoningContent(event.RawJSON()); rc != "" {
					appendReasoningDelta(rc)
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
			if len(pendingReasoningItemRaws) > 0 {
				hasOutput = true
			}
			if answerState != nil && len(answerState.responsesReasoningItemRaws) > 0 {
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
		ToolCalls: toolCalls,
		RawTextReader: &rawText,
		ReasonReader:  reasonReader,
		ContentReader: contentReader,
	}
	out.Wait = func() error {
		<-streamDone
		out.Usage = usage
		out.NativeAssistantTextFinishesTurn = !sawToolCall && contentBuf.Len() > 0
		if answerState != nil {
			out.Phase = strings.TrimSpace(answerState.phase)
			out.ResponsesOutputMessageRaw = strings.TrimSpace(answerState.responsesOutputMessageRaw)
			if len(answerState.responsesReasoningItemRaws) > 0 {
				out.ResponsesReasoningItemRaws = append([]string(nil), answerState.responsesReasoningItemRaws...)
			}
		}
		return waitErr
	}
	return out, nil
}
