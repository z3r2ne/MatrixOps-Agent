package session

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"matrixops-agent/llm"
	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

type MemoryCompactionStreamOptions struct {
	ModelSettings      *models.ModelSettings
	HTTPClient         *http.Client
	OnSummaryDelta     func(summary string)
	SummaryDeltaMinGap int // milliseconds; 0 uses default 120ms
}

type MemoryCompactionStreamResult struct {
	Summary string
	Finish  string
	Model   string
}

func StreamMemoryCompactionSummary(client llm.ChatClient, request llm.ChatRequest, options MemoryCompactionStreamOptions) (MemoryCompactionStreamResult, error) {
	request.ProviderOptions = memoryCompactionProviderOptions(request.ProviderOptions, options.ModelSettings)
	onDelta := throttleSummaryDelta(options.OnSummaryDelta, options.SummaryDeltaMinGap)
	providerOptions := request.ProviderOptions
	if providerOptions != nil && providerOptions.NativeOpenAIToolCalls {
		input, ok := buildMemoryCompactionNativeStreamInput(request, options.HTTPClient)
		if !ok {
			return MemoryCompactionStreamResult{}, fmt.Errorf("memory compaction native stream requires the last message to be a user message")
		}
		out, err := coreagent.StreamV2OpenAINativeOnce(input)
		if err != nil {
			return MemoryCompactionStreamResult{}, err
		}
		summary, finish, err := collectNativeMemoryCompactionSummary(out, onDelta)
		if err != nil {
			return MemoryCompactionStreamResult{}, err
		}
		summary, err = sanitizeMemoryCompactionSummary(summary)
		if err != nil {
			return MemoryCompactionStreamResult{}, err
		}
		return MemoryCompactionStreamResult{
			Summary: summary,
			Finish:  finish,
			Model:   request.Model,
		}, nil
	}

	textBuilder := strings.Builder{}
	streamOutput, err := Stream(StreamInput{
		Context:         request.Context,
		Model:           request.Model,
		Messages:        request.Messages,
		Tools:           nil,
		Abort:           request.Context,
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		MaxOutputTokens: request.MaxOutputTokens,
		ProviderOptions: request.ProviderOptions,
		HTTPClient:      options.HTTPClient,
	}, client, func(event llm.StreamEvent) {
		if event.Type != string(llm.GeneratorMessageTypeTextDelta) || strings.TrimSpace(event.Text) == "" {
			return
		}
		textBuilder.WriteString(event.Text)
		if onDelta != nil {
			onDelta(textBuilder.String())
		}
	})
	if err != nil {
		return MemoryCompactionStreamResult{}, err
	}
	if onDelta != nil && strings.TrimSpace(streamOutput.Text) != "" {
		onDelta(streamOutput.Text)
	}
	summary, err := sanitizeMemoryCompactionSummary(streamOutput.Text)
	if err != nil {
		return MemoryCompactionStreamResult{}, err
	}
	if onDelta != nil && summary != streamOutput.Text {
		onDelta(summary)
	}
	return MemoryCompactionStreamResult{
		Summary: summary,
		Finish:  streamOutput.Finish,
		Model:   request.Model,
	}, nil
}

func collectNativeMemoryCompactionSummary(out *coreagent.StreamOutput, onDelta func(string)) (string, string, error) {
	if out == nil {
		return "", "", nil
	}
	finish := ""
	if out.NativeAssistantTextFinishesTurn {
		finish = "stop"
	}

	reader := out.ContentReader
	if reader == nil {
		reader = out.RawTextReader
	}
	if reader != nil {
		waitErrCh := make(chan error, 1)
		go func() {
			if out.Wait != nil {
				waitErrCh <- out.Wait()
				return
			}
			waitErrCh <- nil
		}()

		summary, readErr := readCompactionSummaryStream(reader, onDelta)
		waitErr := <-waitErrCh
		if readErr != nil {
			return strings.TrimSpace(summary), finish, readErr
		}
		if waitErr != nil {
			return strings.TrimSpace(summary), finish, waitErr
		}
		if strings.TrimSpace(summary) == "" {
			summary, err := collectStreamV2AnswerText(out)
			if onDelta != nil && strings.TrimSpace(summary) != "" {
				onDelta(summary)
			}
			return strings.TrimSpace(summary), finish, err
		}
		return strings.TrimSpace(summary), finish, nil
	}

	summary, err := collectStreamV2AnswerText(out)
	if onDelta != nil && strings.TrimSpace(summary) != "" {
		onDelta(summary)
	}
	if out.Wait != nil {
		if waitErr := out.Wait(); waitErr != nil && err == nil {
			err = waitErr
		}
	}
	return strings.TrimSpace(summary), finish, err
}

func readCompactionSummaryStream(reader io.Reader, onDelta func(string)) (string, error) {
	if reader == nil {
		return "", nil
	}
	buf := make([]byte, 2048)
	accumulated := strings.Builder{}
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			accumulated.Write(buf[:n])
			if onDelta != nil {
				onDelta(accumulated.String())
			}
		}
		if err == io.EOF {
			return accumulated.String(), nil
		}
		if err != nil {
			return accumulated.String(), err
		}
	}
}

func throttleSummaryDelta(onDelta func(string), minGapMS int) func(string) {
	if onDelta == nil {
		return nil
	}
	gapMS := minGapMS
	if gapMS <= 0 {
		gapMS = 120
	}
	var (
		lastEmit int64
		pending  string
	)
	return func(summary string) {
		summary = strings.TrimSpace(summary)
		if summary == "" {
			return
		}
		now := time.Now().UnixMilli()
		if summary == pending && now-lastEmit < int64(gapMS) {
			return
		}
		pending = summary
		lastEmit = now
		onDelta(summary)
	}
}

func buildMemoryCompactionNativeStreamInput(request llm.ChatRequest, httpClient *http.Client) (coreagent.StreamInput, bool) {
	filtered := make([]*llm.ModelMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		if message == nil {
			continue
		}
		filtered = append(filtered, message)
	}
	if len(filtered) == 0 {
		return coreagent.StreamInput{}, false
	}

	last := filtered[len(filtered)-1]
	if strings.TrimSpace(last.Role) != "user" {
		return coreagent.StreamInput{}, false
	}

	instruction := ""
	history := make([]*coreagent.ModelMessage, 0, len(filtered)-1)
	for _, message := range filtered[:len(filtered)-1] {
		if strings.TrimSpace(message.Role) == "system" {
			if text, ok := message.Content.(string); ok && strings.TrimSpace(text) != "" {
				if instruction != "" {
					instruction += "\n\n"
				}
				instruction += strings.TrimSpace(text)
			}
			continue
		}
		history = append(history, &coreagent.ModelMessage{
			Role:       message.Role,
			Content:    message.Content,
			Name:       message.Name,
			ToolCallID: message.ToolCallID,
			ToolCalls:  llmToolCallsToCore(message.ToolCalls),
		})
	}

	prompt, ok := last.Content.(string)
	if !ok {
		return coreagent.StreamInput{}, false
	}
	return coreagent.StreamInput{
		Context:         request.Context,
		Model:           request.Model,
		Prompt:          prompt,
		Instruction:     instruction,
		HistoryMessages: history,
		Abort:           request.Context,
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		MaxOutputTokens: request.MaxOutputTokens,
		ProviderOptions: request.ProviderOptions,
		HTTPClient:      httpClient,
	}, true
}

func memoryCompactionProviderOptions(providerOptions *models.LLMConfig, modelSettings *models.ModelSettings) *models.LLMConfig {
	if providerOptions == nil {
		return nil
	}
	cloned := *providerOptions
	// Compaction must never expose tools (kimi-cli EmptyToolset).
	cloned.NativeOpenAIToolCalls = false
	_ = modelSettings
	return &cloned
}
