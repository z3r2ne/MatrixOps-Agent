package streamtypes

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"pkgs/jsonextractor"
)

// FlatToolNameParamsToActionOutput converts a top-level {"call_tool":"...","params":...} into an ActionOutput.
func FlatToolNameParamsToActionOutput(name string, params json.RawMessage, index int, rawJSON string) (*ActionOutput, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("missing call_tool field")
	}
	pbytes := params
	if len(bytes.TrimSpace(pbytes)) == 0 {
		pbytes = []byte("null")
	}
	dataBytes := append([]byte(nil), pbytes...)
	raw := strings.TrimSpace(rawJSON)
	if raw == "" {
		raw = fmt.Sprintf(`{"call_tool":%q,"params":%s}`, name, strings.TrimSpace(string(pbytes)))
	}
	return &ActionOutput{
		Index:   index,
		Action:  name,
		Data:    bytes.NewReader(dataBytes),
		RawJSON: raw,
	}, nil
}

// DecodeActionOutput decodes an ActionOutput from a JSON raw message.
func DecodeActionOutput(raw json.RawMessage, index int) (*ActionOutput, error) {
	var env struct {
		CallTool string          `json:"call_tool"`
		Params   json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		preview := TruncateStringForLog(strings.TrimSpace(string(raw)), 900)
		return nil, fmt.Errorf("unmarshal tool envelope #%d: %w (raw_len=%d raw_prefix=%q)", index+1, err, len(raw), preview)
	}
	return FlatToolNameParamsToActionOutput(env.CallTool, env.Params, index, strings.TrimSpace(string(raw)))
}

// ParseActionBytes parses one or more ActionOutputs from a payload.
func ParseActionBytes(payload []byte) ([]*ActionOutput, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	actions := make([]*ActionOutput, 0, 4)
	for index := 0; ; index++ {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				return actions, nil
			}
			off := decoder.InputOffset()
			ctx := SnippetAroundBytes(payload, int(off), 160)
			return nil, fmt.Errorf("json.Decoder.Decode action envelope #%d at decoder input offset %d: %w (nearby_bytes=%q)", index+1, off, err, ctx)
		}
		action, err := DecodeActionOutput(raw, index)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
}

func isJSONWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// skipJSONWhitespace consumes ASCII JSON whitespace; EOF means no more non-whitespace input.
func skipJSONWhitespace(br *bufio.Reader) error {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return err
		}
		if !isJSONWhitespace(b) {
			return br.UnreadByte()
		}
	}
}

// readBalancedJSONObject reads bytes from br starting after the opening `{` has been consumed,
// and appends the rest of the object including the matching closing `}` into seg.
// depth is the brace depth after consuming that first `{` (i.e. 1).
func readBalancedJSONObject(br *bufio.Reader, seg *[]byte, depth int) error {
	inString := false
	escape := false
	for depth > 0 {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		*seg = append(*seg, b)
		if inString {
			if escape {
				escape = false
				continue
			}
			if b == '\\' {
				escape = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			continue
		}
		switch b {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return nil
}

// parseEnvelopeJSONStream runs jsonextractor on a single top-level tool envelope reader.
// One call corresponds to exactly one JSON object so async params handlers cannot overlap
// the next envelope's call_tool key.
func parseEnvelopeJSONStream(r io.Reader, actions chan<- *ActionOutput, actionIndex *int, setParseErr func(error)) error {
	var (
		mu          sync.Mutex
		pendingName string
		haveName    bool
		haveParams  bool
		paramsBytes []byte
		emitted     *ActionOutput
		streamBuf   *StreamingActionBuffer
	)

	var (
		firstErrMu sync.Mutex
		firstErr   error
	)
	wrap := func(e error) {
		if e == nil || errors.Is(e, io.EOF) {
			return
		}
		setParseErr(e)
		firstErrMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		firstErrMu.Unlock()
	}

	emitIfReady := func() {
		if !haveName || !haveParams || emitted != nil {
			return
		}
		out, err := FlatToolNameParamsToActionOutput(pendingName, paramsBytes, *actionIndex, "")
		if err != nil {
			wrap(err)
			return
		}
		actions <- out
		(*actionIndex)++
		emitted = out
	}

	emitStreamingIfReady := func(reader io.Reader) {
		if !haveName || haveParams || emitted != nil || reader == nil {
			return
		}
		streamBuf = NewStreamingActionBuffer()
		out := &ActionOutput{
			Index:  *actionIndex,
			Action: pendingName,
			Data:   streamBuf,
		}
		actions <- out
		(*actionIndex)++
		emitted = out
	}

	err := jsonextractor.ExtractStructuredJSONFromStream(r,
		jsonextractor.WithFormatKeyValueCallback(func(key any, data any, parents []string) {
			if len(parents) != 0 {
				return
			}
			ks, ok := key.(string)
			if !ok || ks == "params" || ks != "call_tool" {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			if haveName {
				wrap(fmt.Errorf("duplicate call_tool field"))
				return
			}
			var nameStr string
			switch v := data.(type) {
			case string:
				nameStr = v
			default:
				nameStr = strings.TrimSpace(fmt.Sprint(v))
			}
			pendingName = nameStr
			haveName = true
			emitIfReady()
		}),
		jsonextractor.WithRegisterFieldStreamHandlerAndStartCallback("params",
			func(key string, fieldReader io.Reader, parents []string) {
				if len(parents) != 0 {
					_, _ = io.Copy(io.Discard, fieldReader)
					return
				}
				var (
					buf [32 * 1024]byte
					acc bytes.Buffer
				)
				for {
					n, rerr := fieldReader.Read(buf[:])
					if n > 0 {
						chunk := append([]byte(nil), buf[:n]...)
						if _, err := acc.Write(chunk); err != nil {
							if streamBuf != nil {
								_ = streamBuf.CloseWithError(err)
							}
							wrap(err)
							return
						}
						if streamBuf != nil {
							if _, err := streamBuf.Write(chunk); err != nil {
								wrap(err)
								return
							}
						}
					}
					if rerr == nil {
						continue
					}
					if !errors.Is(rerr, io.EOF) {
						if streamBuf != nil {
							_ = streamBuf.CloseWithError(rerr)
						}
						wrap(rerr)
						return
					}
					break
				}

				mu.Lock()
				defer mu.Unlock()
				if streamBuf != nil {
					_ = streamBuf.Close()
				}
				if haveParams {
					wrap(fmt.Errorf("duplicate params field"))
					return
				}
				paramsBytes = append([]byte(nil), acc.Bytes()...)
				haveParams = true
				if emitted != nil {
					raw := strings.TrimSpace(string(paramsBytes))
					if raw == "" {
						raw = "null"
					}
					emitted.RawJSON = fmt.Sprintf(`{"call_tool":%q,"params":%s}`, pendingName, raw)
				}
				emitIfReady()
			},
			func(key string, fieldReader io.Reader, parents []string) {
				if len(parents) != 0 {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				emitStreamingIfReady(fieldReader)
			},
		),
	)

	if err == nil {
		mu.Lock()
		switch {
		case haveName && !haveParams:
			wrap(fmt.Errorf("missing params field"))
		case !haveName && haveParams:
			wrap(fmt.Errorf("missing call_tool field"))
		}
		mu.Unlock()
	}

	if err != nil && !errors.Is(err, io.EOF) {
		wrap(err)
	}
	firstErrMu.Lock()
	out := firstErr
	firstErrMu.Unlock()
	return out
}

// ParseActionStream parses one or more top-level JSON tool envelopes from r.
// It incrementally splits concatenated root objects (brace-balanced, string-aware), then
// runs jsonextractor per envelope so large "params" values still use streaming field handlers.
func ParseActionStream(r io.Reader, actions chan<- *ActionOutput, setParseErr func(error)) {
	defer close(actions)
	if closer, ok := r.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	br := bufio.NewReader(r)
	actionIndex := 0
	for {
		if err := skipJSONWhitespace(br); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			setParseErr(err)
			return
		}
		b, err := br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			setParseErr(err)
			return
		}
		if b != '{' {
			setParseErr(fmt.Errorf("expected JSON object start, got %q", b))
			return
		}
		objReader, objWriter := io.Pipe()
		parserDone := make(chan error, 1)
		go func() {
			parserDone <- parseEnvelopeJSONStream(objReader, actions, &actionIndex, setParseErr)
		}()

		if _, err := objWriter.Write([]byte{b}); err != nil {
			_ = objWriter.CloseWithError(err)
			setParseErr(err)
			return
		}

		inString := false
		escape := false
		depth := 1
		for depth > 0 {
			ch, err := br.ReadByte()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				_ = objWriter.CloseWithError(err)
				setParseErr(err)
				return
			}
			if _, err := objWriter.Write([]byte{ch}); err != nil {
				_ = objWriter.CloseWithError(err)
				setParseErr(err)
				return
			}

			if inString {
				if escape {
					escape = false
					continue
				}
				if ch == '\\' {
					escape = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if err := objWriter.Close(); err != nil {
			setParseErr(err)
			return
		}
		if err := <-parserDone; err != nil {
			return
		}
	}
}
