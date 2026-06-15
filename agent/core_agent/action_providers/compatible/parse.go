package compatible

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"matrixops.local/core_agent/streamtypes"
	"pkgs/jsonextractor"
)

const actionFieldName = "@action"

// FlatActionDataToActionOutput 将 {"@action":"...","data":...} 转为 ActionOutput。
func FlatActionDataToActionOutput(actionName string, data json.RawMessage, index int, rawJSON string) (*streamtypes.ActionOutput, error) {
	actionName = strings.TrimSpace(actionName)
	if actionName == "" {
		return nil, fmt.Errorf("missing @action field")
	}
	dataBytes := data
	if len(bytes.TrimSpace(dataBytes)) == 0 {
		dataBytes = []byte("null")
	}
	payload := append([]byte(nil), dataBytes...)
	raw := strings.TrimSpace(rawJSON)
	if raw == "" {
		raw = BuildActionEnvelopeJSON(actionName, dataBytes)
	}
	return &streamtypes.ActionOutput{
		Index:   index,
		Action:  actionName,
		Data:    bytes.NewReader(payload),
		RawJSON: raw,
	}, nil
}

// BuildActionEnvelopeJSON 构造兼容模式 action 信封 JSON。
func BuildActionEnvelopeJSON(action string, data json.RawMessage) string {
	action = strings.TrimSpace(action)
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		data = []byte("null")
	}
	actionJSON, _ := json.Marshal(action)
	return fmt.Sprintf(`{"%s":%s,"data":%s}`, actionFieldName, string(actionJSON), string(data))
}

func decodeActionOutput(raw json.RawMessage, index int) (*streamtypes.ActionOutput, error) {
	var env struct {
		Action string          `json:"@action"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		preview := streamtypes.TruncateStringForLog(strings.TrimSpace(string(raw)), 900)
		return nil, fmt.Errorf("unmarshal action envelope #%d: %w (raw_len=%d raw_prefix=%q)", index+1, err, len(raw), preview)
	}
	return FlatActionDataToActionOutput(env.Action, env.Data, index, strings.TrimSpace(string(raw)))
}

// ParseActionBytes 从 payload 解析一个或多个 action 信封。
func ParseActionBytes(payload []byte) ([]*streamtypes.ActionOutput, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	actions := make([]*streamtypes.ActionOutput, 0, 4)
	for index := 0; ; index++ {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				return actions, nil
			}
			off := decoder.InputOffset()
			ctx := streamtypes.SnippetAroundBytes(payload, int(off), 160)
			return nil, fmt.Errorf("json.Decoder.Decode action envelope #%d at decoder input offset %d: %w (nearby_bytes=%q)", index+1, off, err, ctx)
		}
		action, err := decodeActionOutput(raw, index)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
}

func parseEnvelopeJSONStream(r io.Reader, actions chan<- *streamtypes.ActionOutput, actionIndex *int, setParseErr func(error)) error {
	var (
		mu           sync.Mutex
		pendingName  string
		haveName     bool
		haveData     bool
		dataBytes    []byte
		emitted      *streamtypes.ActionOutput
		streamBuf    *streamtypes.StreamingActionBuffer
		firstErrMu   sync.Mutex
		firstErr     error
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
		if !haveName || !haveData || emitted != nil {
			return
		}
		out, err := FlatActionDataToActionOutput(pendingName, dataBytes, *actionIndex, "")
		if err != nil {
			wrap(err)
			return
		}
		actions <- out
		(*actionIndex)++
		emitted = out
	}

	emitStreamingIfReady := func(reader io.Reader) {
		if !haveName || haveData || emitted != nil || reader == nil {
			return
		}
		streamBuf = streamtypes.NewStreamingActionBuffer()
		out := &streamtypes.ActionOutput{
			Index:  *actionIndex,
			Action: pendingName,
			Data:   streamBuf,
		}
		actions <- out
		(*actionIndex)++
		emitted = out
	}

	err := jsonextractor.ExtractStructuredJSONFromStream(r,
		jsonextractor.WithFormatKeyValueCallback(func(key any, value any, parents []string) {
			if len(parents) != 0 {
				return
			}
			ks, ok := key.(string)
			if !ok || ks == "data" || ks != actionFieldName {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			if haveName {
				wrap(fmt.Errorf("duplicate @action field"))
				return
			}
			var nameStr string
			switch v := value.(type) {
			case string:
				nameStr = v
			default:
				nameStr = strings.TrimSpace(fmt.Sprint(v))
			}
			pendingName = nameStr
			haveName = true
			emitIfReady()
		}),
		jsonextractor.WithRegisterFieldStreamHandlerAndStartCallback("data",
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
				if haveData {
					wrap(fmt.Errorf("duplicate data field"))
					return
				}
				dataBytes = append([]byte(nil), acc.Bytes()...)
				haveData = true
				if emitted != nil {
					raw := strings.TrimSpace(string(dataBytes))
					if raw == "" {
						raw = "null"
					}
					emitted.RawJSON = BuildActionEnvelopeJSON(pendingName, []byte(raw))
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
		case haveName && !haveData:
			wrap(fmt.Errorf("missing data field"))
		case !haveName && haveData:
			wrap(fmt.Errorf("missing @action field"))
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

// ParseActionStream 从 reader 增量解析一个或多个 {@action,data} 信封。
func ParseActionStream(r io.Reader, actions chan<- *streamtypes.ActionOutput, setParseErr func(error)) {
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

func skipJSONWhitespace(br *bufio.Reader) error {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return err
		}
		if !streamtypes.IsWhitespaceByte(b) {
			return br.UnreadByte()
		}
	}
}
