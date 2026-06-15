package openai_native

import (
	"bytes"
	"fmt"
	"strings"

	"matrixops.local/core_agent/streamtypes"
)
type nativeOpenAITextMode int

const (
	nativeOpenAITextModeUnknown nativeOpenAITextMode = iota
	nativeOpenAITextModePlain
	nativeOpenAITextModeJSON
)

type nativeOpenAIToolState struct {
	itemID                     string
	outputIndex                int64
	callID                     string
	name                       string
	buffer                     *streamtypes.StreamingActionBuffer
	request                    *streamtypes.CallToolRequest
	args                       bytes.Buffer
	finished                   bool
	responsesReasoningItemRaws []string
}

func (s *nativeOpenAIToolState) write(delta string) error {
	if s == nil || s.finished || delta == "" {
		return nil
	}
	s.args.WriteString(delta)
	_, err := s.buffer.Write([]byte(delta))
	return err
}

func (s *nativeOpenAIToolState) syncFull(full string) error {
	if s == nil || s.finished {
		return nil
	}
	if len(full) <= s.args.Len() {
		return nil
	}
	return s.write(full[s.args.Len():])
}

func (s *nativeOpenAIToolState) finish() error {
	if s == nil || s.finished {
		return nil
	}
	s.finished = true
	if s.request != nil {
		s.request.RawJSON = fmt.Sprintf(`{"@action":"call_tool","data":{"name":%q,"params":%s}}`, s.name, strings.TrimSpace(s.args.String()))
	}
	return s.buffer.Close()
}

type nativeOpenAIAnswerState struct {
	itemID                     string
	outputIndex                int64
	text                       bytes.Buffer
	finished                   bool
	phase                      string
	responsesOutputMessageRaw  string
	responsesReasoningItemRaws []string
}

func (s *nativeOpenAIAnswerState) write(delta string) error {
	if s == nil || s.finished || delta == "" {
		return nil
	}
	s.text.WriteString(delta)
	return nil
}

func (s *nativeOpenAIAnswerState) finish() error {
	if s == nil || s.finished {
		return nil
	}
	s.finished = true
	return nil
}
