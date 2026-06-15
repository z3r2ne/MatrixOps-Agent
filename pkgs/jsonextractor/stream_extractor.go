package jsonextractor

import (
	"bytes"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// ConditionalCallback 条件回调
type ConditionalCallback struct {
	condition []string
	callback  func(data map[string]any)
}

func (c *ConditionalCallback) Feed(data map[string]any) {
	if c == nil || c.condition == nil || data == nil {
		return
	}

	for _, v := range c.condition {
		if _, existed := data[v]; !existed {
			return
		}
	}
	if c.callback == nil {
		return
	}
	c.callback(data)
}

// FieldMatchType 字段匹配类型
type FieldMatchType int

const (
	FieldMatchExact  FieldMatchType = iota // 精确匹配
	FieldMatchMulti                        // 多字段匹配（任意一个匹配即可）
	FieldMatchRegexp                       // 正则表达式匹配
	FieldMatchGlob                         // Glob模式匹配
)

// FieldStreamHandler 字段流处理器
type FieldStreamHandler struct {
	// 匹配相关
	matchType  FieldMatchType // 匹配类型
	pattern    string         // 匹配模式：可以是字段名、正则表达式或glob模式
	fieldNames []string       // 多字段匹配时使用

	syncHandler func(key string, reader io.Reader, parents []string)
	// 统一的回调函数
	handler func(key string, reader io.Reader, parents []string) // 回调函数，包含字段名和父路径
}

type fieldStreamContext struct {
	key    string
	writer io.WriteCloser
}

type fieldStreamFrame struct {
	contexts []*fieldStreamContext
}

type callbackManager struct {
	objectKeyValueCallback      func(string string, data any)
	arrayValueCallback          func(idx int, data any)
	onRootMapCallback           func(i map[string]any)
	onArrayCallback             func(data []any)
	onObjectCallback            func(data map[string]any)
	onConditionalObjectCallback []*ConditionalCallback

	fieldStreamHandlers []*FieldStreamHandler

	rawKVCallback    func(key, data any)
	formatKVCallback func(key, value any, parents []string)

	// 字段流处理相关
	activeWriters         []io.WriteCloser    // 当前活跃的写入器列表，支持多字段同时写入
	fieldStreamFrameStack []*fieldStreamFrame // 字段流写入栈，用于支持嵌套结构

	// stream finish callback
	streamFinishedCallback func()
	// stream error callback
	streamErrorCallback func(err error)

	// 每个字段流 handler 在独立 goroutine 中运行；resetFieldStreamFrames 关闭 writer 后需等待其退出，避免调用方与 handler 竞态。
	fieldHandlerWG sync.WaitGroup
}

// kv method is now defined in formatter.go with proper key-value formatting

// CallbackOption 回调选项
type CallbackOption func(*callbackManager)

// WithStreamFinishedCallback 流结束回调
func WithStreamFinishedCallback(callback func()) CallbackOption {
	return func(manager *callbackManager) {
		manager.streamFinishedCallback = callback
	}
}

// WithStreamErrorCallback 流错误回调
func WithStreamErrorCallback(callback func(err error)) CallbackOption {
	return func(manager *callbackManager) {
		manager.streamErrorCallback = callback
	}
}

// WithObjectKeyValue 对象键值回调
func WithObjectKeyValue(callback func(string string, data any)) CallbackOption {
	return func(c *callbackManager) {
		c.objectKeyValueCallback = callback
	}
}

// WithRawKeyValueCallback 原始键值回调
func WithRawKeyValueCallback(callback func(key, data any)) CallbackOption {
	return func(c *callbackManager) {
		c.rawKVCallback = callback
	}
}

// WithFormatKeyValueCallback 格式化键值回调
func WithFormatKeyValueCallback(callback func(key, data any, parents []string)) CallbackOption {
	return func(c *callbackManager) {
		c.formatKVCallback = callback
	}
}

// WithArrayCallback 数组回调
func WithArrayCallback(callback func(data []any)) CallbackOption {
	return func(c *callbackManager) {
		c.onArrayCallback = callback
	}
}

// WithRegisterConditionalObjectCallback 注册条件对象回调
func WithRegisterConditionalObjectCallback(key []string, callback func(data map[string]any)) CallbackOption {
	return func(c *callbackManager) {
		if c.onConditionalObjectCallback == nil {
			c.onConditionalObjectCallback = make([]*ConditionalCallback, 0)
		}
		c.onConditionalObjectCallback = append(c.onConditionalObjectCallback, &ConditionalCallback{
			condition: key,
			callback:  callback,
		})
	}
}

// WithObjectCallback 对象回调
func WithObjectCallback(callback func(data map[string]any)) CallbackOption {
	return func(c *callbackManager) {
		c.onObjectCallback = callback
	}
}

// WithRootMapCallback 根对象回调
func WithRootMapCallback(callback func(data map[string]any)) CallbackOption {
	return func(c *callbackManager) {
		c.onRootMapCallback = callback
	}
}

// WithRegisterFieldStreamHandler 注册字段流处理器
func WithRegisterFieldStreamHandler(fieldName string, handler func(key string, reader io.Reader, parents []string)) CallbackOption {
	return func(c *callbackManager) {
		if c.fieldStreamHandlers == nil {
			c.fieldStreamHandlers = make([]*FieldStreamHandler, 0)
		}
		c.fieldStreamHandlers = append(c.fieldStreamHandlers, &FieldStreamHandler{
			matchType: FieldMatchExact,
			pattern:   fieldName,
			handler:   handler,
		})
	}
}

// WithRegisterMultiFieldStreamHandler 注册多字段流处理器
func WithRegisterMultiFieldStreamHandler(fieldNames []string, handler func(key string, reader io.Reader, parents []string)) CallbackOption {
	return func(c *callbackManager) {
		if c.fieldStreamHandlers == nil {
			c.fieldStreamHandlers = make([]*FieldStreamHandler, 0)
		}
		c.fieldStreamHandlers = append(c.fieldStreamHandlers, &FieldStreamHandler{
			matchType:  FieldMatchMulti,
			fieldNames: fieldNames,
			handler:    handler,
		})
	}
}

// WithRegisterRegexpFieldStreamHandler 注册正则表达式字段流处理器
func WithRegisterRegexpFieldStreamHandler(pattern string, handler func(key string, reader io.Reader, parents []string)) CallbackOption {
	return func(c *callbackManager) {
		if c.fieldStreamHandlers == nil {
			c.fieldStreamHandlers = make([]*FieldStreamHandler, 0)
		}
		c.fieldStreamHandlers = append(c.fieldStreamHandlers, &FieldStreamHandler{
			matchType: FieldMatchRegexp,
			pattern:   pattern,
			handler:   handler,
		})
	}
}

// WithRegisterGlobFieldStreamHandler 注册Glob模式字段流处理器
func WithRegisterGlobFieldStreamHandler(pattern string, handler func(key string, reader io.Reader, parents []string)) CallbackOption {
	return func(c *callbackManager) {
		if c.fieldStreamHandlers == nil {
			c.fieldStreamHandlers = make([]*FieldStreamHandler, 0)
		}
		c.fieldStreamHandlers = append(c.fieldStreamHandlers, &FieldStreamHandler{
			matchType: FieldMatchGlob,
			pattern:   pattern,
			handler:   handler,
		})
	}
}

// WithRegisterFieldStreamHandlerAndStartCallback 注册字段流处理器（带同步启动回调）
func WithRegisterFieldStreamHandlerAndStartCallback(fieldName string, handler func(key string, reader io.Reader, parents []string), syncCallback func(key string, reader io.Reader, parents []string)) CallbackOption {
	return func(c *callbackManager) {
		if c.fieldStreamHandlers == nil {
			c.fieldStreamHandlers = make([]*FieldStreamHandler, 0)
		}
		c.fieldStreamHandlers = append(c.fieldStreamHandlers, &FieldStreamHandler{
			matchType:   FieldMatchExact,
			pattern:     fieldName,
			handler:     handler,
			syncHandler: syncCallback,
		})
	}
}

// handleFieldStreamStart 开始字段流处理
func (c *callbackManager) handleFieldStreamStart(fieldName string, bufManager *bufStackManager) []*fieldStreamContext {
	// 清理字段名中的引号和空格
	cleanFieldName := strings.Trim(strings.TrimSpace(fieldName), `"`)

	// 从stack获取父路径
	var prefix []string
	if bufManager != nil {
		prefix = bufManager.getPrefixKey()
	}

	var contexts []*fieldStreamContext

	// 检查所有字段处理器
	if c.fieldStreamHandlers != nil {
		for _, handler := range c.fieldStreamHandlers {
			if c.isFieldMatch(cleanFieldName, handler) {
				ctx := c.createFieldStream(cleanFieldName, handler, prefix)
				if ctx != nil {
					contexts = append(contexts, ctx)
				}
			}
		}
	}

	return contexts
}

// isFieldMatch 检查字段是否匹配处理器
func (c *callbackManager) isFieldMatch(fieldName string, handler *FieldStreamHandler) bool {
	switch handler.matchType {
	case FieldMatchExact:
		return handler.pattern == fieldName
	case FieldMatchMulti:
		return matchAnyOfSubString(fieldName, handler.fieldNames...)
	case FieldMatchRegexp:
		return matchRegexp(fieldName, handler.pattern)
	case FieldMatchGlob:
		return matchGlob(fieldName, handler.pattern)
	default:
		return false
	}
}

// matchAnyOfSubString 检查字符串是否包含任意一个子串
func matchAnyOfSubString(s string, subStrings ...string) bool {
	s = strings.ToLower(s)
	for _, sub := range subStrings {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// matchRegexp 检查字符串是否匹配正则表达式
func matchRegexp(s string, pattern string) bool {
	matched, err := regexp.MatchString(pattern, s)
	if err != nil {
		return false
	}
	return matched
}

// matchGlob 检查字符串是否匹配Glob模式
func matchGlob(s string, pattern string) bool {
	matched, err := filepath.Match(pattern, s)
	if err != nil {
		return false
	}
	return matched
}

// createFieldStream 创建字段流
func (c *callbackManager) createFieldStream(fieldName string, handler *FieldStreamHandler, parents []string) *fieldStreamContext {
	// 创建管道
	reader, writer := NewPipe()

	if handler.syncHandler != nil {
		// 调用开始回调函数 用于强同步
		handler.syncHandler(fieldName, reader, parents)
	}

	c.fieldHandlerWG.Add(1)
	// 在新的 goroutine 中调用处理函数
	go func(h *FieldStreamHandler, r io.Reader, key string, parentPath []string) {
		defer c.fieldHandlerWG.Done()
		defer func() {
			if err := recover(); err != nil {
				// 静默处理 panic
				_ = err
			}
		}()

		// 调用统一的回调函数
		if h.handler != nil {
			h.handler(key, r, parentPath)
		}
	}(handler, reader, fieldName, parents)

	return &fieldStreamContext{
		key:    fieldName,
		writer: writer,
	}
}

func (c *callbackManager) pushFieldStreamFrame(contexts []*fieldStreamContext) *fieldStreamFrame {
	if len(contexts) == 0 {
		return nil
	}
	frame := &fieldStreamFrame{
		contexts: contexts,
	}
	c.fieldStreamFrameStack = append(c.fieldStreamFrameStack, frame)
	for _, ctx := range contexts {
		if ctx != nil && ctx.writer != nil {
			c.activeWriters = append(c.activeWriters, ctx.writer)
		}
	}
	return frame
}

func (c *callbackManager) popFieldStreamFrame(frame *fieldStreamFrame) {
	if frame == nil {
		return
	}
	idx := -1
	for i := len(c.fieldStreamFrameStack) - 1; i >= 0; i-- {
		if c.fieldStreamFrameStack[i] == frame {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	// 移除frame
	c.fieldStreamFrameStack = append(c.fieldStreamFrameStack[:idx], c.fieldStreamFrameStack[idx+1:]...)

	// 重建active writers列表
	c.activeWriters = nil
	for _, frm := range c.fieldStreamFrameStack {
		for _, ctx := range frm.contexts {
			if ctx != nil && ctx.writer != nil {
				c.activeWriters = append(c.activeWriters, ctx.writer)
			}
		}
	}

	// 关闭当前frame的writer
	for _, ctx := range frame.contexts {
		if ctx != nil && ctx.writer != nil {
			ctx.writer.Close()
		}
	}
}

func (c *callbackManager) resetFieldStreamFrames() {
	for len(c.fieldStreamFrameStack) > 0 {
		frame := c.fieldStreamFrameStack[len(c.fieldStreamFrameStack)-1]
		c.popFieldStreamFrame(frame)
	}
}

// clearCurrentFieldWriter 清除当前字段写入器（已废弃，保留兼容性）
func (c *callbackManager) clearCurrentFieldWriter() {
	// 在新的多写入器架构中，此方法不再需要
}

// ExtractStructuredJSON 从字符串解析 JSON
func ExtractStructuredJSON(c string, options ...CallbackOption) error {
	return ExtractStructuredJSONFromStream(bytes.NewBufferString(c), options...)
}

// ExtractStructuredJSONFromStream 从数据流中解析 JSON 数据的核心函数
func ExtractStructuredJSONFromStream(jsonReader io.Reader, options ...CallbackOption) (err error) {
	callbackManager := &callbackManager{}
	for _, option := range options {
		option(callbackManager)
	}
	defer func() {
		if callbackManager == nil {
			return
		}
		if err != nil {
			if callbackManager.streamErrorCallback != nil {
				callbackManager.streamErrorCallback(err)
			}
			return
		}
		if callbackManager.streamFinishedCallback != nil {
			callbackManager.streamFinishedCallback()
		}
	}()
	defer func() {
		callbackManager.resetFieldStreamFrames()
		callbackManager.fieldHandlerWG.Wait()
	}()

	var mirror = new(bytes.Buffer)
	reader := newAutoPeekReader(io.TeeReader(jsonReader, mirror))

	getMirrorBytes := func() string {
		return mirror.String()
	}

	var index = -1
	var objectDepth = 0
	var objectDepthIndexTable = make(map[int]int)

	var results [][2]int
	stack := NewStack()

	type state struct {
		value string
		start int
		end   int

		isObject                 bool
		isArray                  bool
		objectValueHandledString bool
		objectValueInArray       bool
		arrayCurrentKeyIndex     int
		legalArrayItem           bool
		fieldStreamFrame         *fieldStreamFrame
	}

	bufManager := newBufStackManager(func(key any, val any, parents []string) {
		callbackManager.kv(key, val, parents)
	})
	bufManager.setCallbackManager(callbackManager)

	pushStateWithIdx := func(i string, idx int) {
		if i == state_jsonObj {
			bufManager.PushContainer()
			objectDepth++
			if _, existed := objectDepthIndexTable[objectDepth]; !existed {
				objectDepthIndexTable[objectDepth] = index
			}
		} else if i == state_jsonArray {
			bufManager.PushContainer()
		}
		stack.Push(&state{
			value: i,
			start: idx,
			end:   idx,
		})
	}
	currentState := func() string {
		basicState := stack.Peek()
		if basicState == nil {
			return state_reset
		}
		return basicState.(*state).value
	}
	currentStateIns := func() *state {
		basicState := stack.Peek()
		return basicState.(*state)
	}

	getStrSlice := func(s *state) string {
		if s.start > s.end {
			return ""
		}
		if s.start == s.end {
			return ""
		}
		c := getMirrorBytes()
		if s.end >= len(c) {
			s.end = len(c) - 1
		}
		return c[s.start:s.end]
	}
	_ = currentStateIns
	popStateWithIdx := func(idx int) {
		r := stack.Pop()
		if r != nil {
			raw, ok := r.(*state)
			if ok {
				raw.end = idx
				c := getMirrorBytes()
				if raw.end >= len(c) {
					raw.end = len(c) - 1
				}
				sliceValue := getStrSlice(raw)
				if raw.value == state_DoubleQuoteString {
					if parentRaw := stack.Peek(); parentRaw != nil {
						if parentState, ok := parentRaw.(*state); ok {
							// 只有当父状态是 objectKey 时，才准备字段流上下文
							// 如果父状态是 objectValue 或 arrayItem，说明这是一个值，不是键
							if parentState.value == state_objectKey {
								bufManager.prepareFieldStreamContexts(sliceValue)
							}
						}
					}
				}
				switch raw.value {
				case state_objectKey:
					bufManager.PushKey(sliceValue)
				case state_objectValue:
					if !raw.isObject && !raw.isArray {
						bufManager.PushValue(sliceValue)
					}
					// 字段值处理完成，清理当前活跃的写入器
					if raw.fieldStreamFrame != nil && bufManager.callbackManager != nil {
						bufManager.callbackManager.popFieldStreamFrame(raw.fieldStreamFrame)
						raw.fieldStreamFrame = nil
					}
				case state_jsonArray:
					bufManager.PopContainer()
					// 数组处理完成，清理当前活跃的写入器
					if raw.fieldStreamFrame != nil && bufManager.callbackManager != nil {
						bufManager.callbackManager.popFieldStreamFrame(raw.fieldStreamFrame)
						raw.fieldStreamFrame = nil
					}
				case state_jsonObj:
					bufManager.PopContainer()
					// 对象处理完成，清理当前活跃的写入器
					if raw.fieldStreamFrame != nil && bufManager.callbackManager != nil {
						bufManager.callbackManager.popFieldStreamFrame(raw.fieldStreamFrame)
						raw.fieldStreamFrame = nil
					}
					// 记录结果
					ret, ok := objectDepthIndexTable[objectDepth]
					if ok && ret >= 0 {
						results = append(results, [2]int{objectDepthIndexTable[objectDepth], index + 1})
					}
					delete(objectDepthIndexTable, objectDepth)
					if objectDepth == 0 {
						objectDepthIndexTable = make(map[int]int)
					}
					objectDepth--
				case state_arrayItem:
					if !raw.legalArrayItem {
						bufManager.PushValue(sliceValue)
					}
				}

			}
		}
	}
	lastState := func() string {
		basicState := stack.Peek()
		if basicState == nil {
			return state_reset
		}
		return basicState.(*state).value
	}
	_ = lastState

	// 启动栈状态机
	pushStateWithIdx(state_data, 0)
	var ch byte
	// 仅在「根 state_data、等待下一对象或结束」时把 EOF 视为正常；否则为截断输入。
	acceptingEOF := func() bool {
		if stack.Len() == 0 {
			return true
		}
		if stack.Len() != 1 {
			return false
		}
		top := stack.Peek()
		st, ok := top.(*state)
		return ok && st.value == state_data
	}
	for {
		var results = make([]byte, 1)
		n, err := io.ReadFull(reader, results)
		if n <= 0 && err != nil {
			if err == io.EOF {
				if acceptingEOF() {
					return nil
				}
				return io.ErrUnexpectedEOF
			}
			return err
		}
		results = results[:n]
		index++
		if len(results) <= 0 {
			break
		}
		ch = results[0]

		pushState := func(i string) {
			pushStateWithIdx(i, index)
		}
		popState := func() {
			popStateWithIdx(index)
		}

		// 处理字符级流式写入
		writeToFieldStream := func() {
			if len(callbackManager.activeWriters) > 0 {
				data := []byte{ch}
				for _, writer := range callbackManager.activeWriters {
					if writer != nil {
						writer.Write(data)
					}
				}
			}
		}
	RETRY:
		switch currentState() {
		case state_arrayItem:
			if unicode.IsSpace(rune(ch)) {
				continue
			}
			if ch == ',' || ch == ']' {
				popState()
				goto RETRY // array item not consume ',' and ']'
			}
			currentStateIns().legalArrayItem = true
			popState()
			pushState(state_objectValue)
			currentStateIns().objectValueInArray = true
			goto RETRY
		case state_jsonArray:
			s := currentStateIns()
			switch ch {
			case ']':
				writeToFieldStream() // 写入结束括号
				popState()
			case ',': // if get ',' means has new array item, should push state
				writeToFieldStream()             // 写入逗号
				if s.arrayCurrentKeyIndex == 0 { // if get ',' and index == 0 ,should consume it. push 0:""
					bufManager.PushKey(s.arrayCurrentKeyIndex)
					s.arrayCurrentKeyIndex++
					bufManager.PushValue("")
				}
				bufManager.PushKey(s.arrayCurrentKeyIndex)
				s.arrayCurrentKeyIndex++
				pushStateWithIdx(state_arrayItem, index+1) // item should not contains this comma
			default:
				if unicode.IsSpace(rune(ch)) {
					writeToFieldStream() // 写入空白字符
					continue
				}
				bufManager.PushKey(s.arrayCurrentKeyIndex)
				s.arrayCurrentKeyIndex++
				pushState(state_arrayItem)
				goto RETRY
			}
		case state_objectValue:
			switch ch {
			case ' ', '\t', '\r':
				writeToFieldStream() // 写入空白字符
				continue
			case '[':
				currentStateIns().isArray = true
				// 激活待处理的字段流写入器，用于处理数组类型的值
				frame := bufManager.activatePendingFieldWriter()
				writeToFieldStream() // 写入开始括号
				pushState(state_jsonArray)
				if frame != nil {
					currentStateIns().fieldStreamFrame = frame
				}
			case '{':
				currentStateIns().isObject = true
				// 激活待处理的字段流写入器，用于处理对象类型的值
				frame := bufManager.activatePendingFieldWriter()
				writeToFieldStream() // 写入开始大括号
				pushState(state_jsonObj)
				if frame != nil {
					currentStateIns().fieldStreamFrame = frame
				}
				pushStateWithIdx(state_objectKey, index+1)
				continue
			case '"':
				if ret := currentStateIns(); ret != nil {
					if ret.objectValueHandledString {
						// 处理过了
						writeToFieldStream() // 写入引号字符
						continue
					} else {
						ret.objectValueHandledString = true
						// 激活待处理的字段流写入器
						frame := bufManager.activatePendingFieldWriter()
						ret.fieldStreamFrame = frame
						writeToFieldStream() // 写入开始引号
						pushState(state_DoubleQuoteString)
						continue
					}
				}
				// 如果没有激活字段流写入器，正常处理
				pushState(state_DoubleQuoteString)
				continue
			case '}':
				if !currentStateIns().objectValueInArray {
					popState()
					goto RETRY
				}
			case '\n':
				popState()
				if currentState() == state_jsonArray {

				} else {
					pushStateWithIdx(state_objectKey, index+1)
				}
			case ',':
				if currentStateIns().objectValueInArray {
					popState()
					goto RETRY
				}
				popState()
				pushStateWithIdx(state_objectKey, index+1)
				goto RETRY
			case ']':
				if currentStateIns().objectValueInArray {
					writeToFieldStream()
					popStateWithIdx(index)
					currentStateName := currentState()
					switch currentStateName {
					case state_jsonArray:
						popStateWithIdx(index)
						continue
					}
					goto RETRY
				}
			default:
				// 处理数字、布尔值、null等其他类型
				if unicode.IsDigit(rune(ch)) || ch == '-' || ch == 't' || ch == 'f' || ch == 'n' {
					// 激活待处理的字段流写入器，用于处理数字、布尔值、null等类型的值
					frame := bufManager.activatePendingFieldWriter()
					currentStateIns().fieldStreamFrame = frame
					writeToFieldStream() // 写入当前字符
					pushState(state_primitiveValue)
					continue
				}
				continue
			}
		case state_objectKey:
			switch ch {
			case '"':
				writeToFieldStream() // 写入键名起始引号
				pushState(state_DoubleQuoteString)
				continue
			case ':':
				writeToFieldStream() // 写入键值分隔符
				popStateWithIdx(index)
				pushStateWithIdx(state_objectValue, index+1)
				continue
			case ',':
				// Check if we're inside a composite value by looking at parent state (2nd element on stack)
				if parentRaw := stack.PeekN(2); parentRaw != nil {
					if parentState, ok := parentRaw.(*state); ok {
						if parentState.value == state_jsonObj || parentState.value == state_jsonArray {
							writeToFieldStream() // 写入逗号（作为复合值的一部分）
							continue
						}
					}
				}
				popState()
				goto RETRY
			case '}':
				writeToFieldStream() // 写入对象结束符，处理空对象
				popStateWithIdx(index - 1)
				if currentState() == state_jsonObj {
					popStateWithIdx(index)
					continue
				}
				continue
			}
		case state_primitiveValue:
			// 处理数字、布尔值、null等基本类型的值
			switch ch {
			case ',', '}', ']', '\n', '\r', '\t', ' ':
				// 遇到结束符，退出基本值处理状态
				popState()
				goto RETRY
			default:
				writeToFieldStream() // 写入当前字符
			}
		case state_data:
			switch ch {
			case '{':
				pushState(state_jsonObj)
				pushStateWithIdx(state_objectKey, index+1)
				continue
			case '"':
				pushState(state_DoubleQuoteString)
				continue
			case '[':
				currentStateIns().isArray = true
				pushState(state_jsonArray)
			}
		case state_jsonObj:
			switch ch {
			case '{':
				writeToFieldStream() // 写入嵌套对象开始大括号
				pushState(state_jsonObj)
				continue
			case '"':
				writeToFieldStream() // 写入引号
				pushState(state_DoubleQuoteString)
				continue
			case '}':
				writeToFieldStream() // 写入结束大括号
				popState()
				continue
			case ',':
				writeToFieldStream() // 写入逗号（作为复合值的一部分）
				if currentState() != state_objectKey {
					pushStateWithIdx(state_objectKey, index+1)
				}
				continue
			case ':', ' ', '\t', '\n', '\r':
				writeToFieldStream() // 写入分隔符和空白字符
				continue
			default:
				writeToFieldStream() // 写入其他字符
				continue
			}
		case state_DoubleQuoteString:
			switch ch {
			case '\\':
				writeToFieldStream() // 写入转义字符
				pushState(state_quote)
				continue
			case '"':
				nextCh, peekErr := reader.Peek()
				isValidEndChar := false
				if peekErr == nil {
					if nextCh == ',' || nextCh == ':' || nextCh == '}' || nextCh == ']' ||
						nextCh == ' ' || nextCh == '\t' || nextCh == '\n' || nextCh == '\r' {
						isValidEndChar = true
					} else if nextCh == '.' {
						peek2, peekErr2 := reader.PeekN(2)
						if peekErr2 == nil && len(peek2) >= 2 {
							afterDot := peek2[1]
							if afterDot == ',' || afterDot == ':' || afterDot == '}' || afterDot == ']' ||
								afterDot == ' ' || afterDot == '\t' || afterDot == '\n' || afterDot == '\r' {
								isValidEndChar = true
							}
						}
					}
				} else if peekErr == io.EOF {
					isValidEndChar = true
				}

				if isValidEndChar {
					writeToFieldStream() // 写入结束引号
					callbackManager.clearCurrentFieldWriter()
					popStateWithIdx(index + 1)
					continue
				} else {
					writeToFieldStream() // 写入引号字符（作为字符串内容）
					continue
				}
			default:
				writeToFieldStream() // 写入普通字符
			}
		case state_quote:
			writeToFieldStream() // 写入被转义的字符
			popState()
			continue
		case state_reset:
			pushState(state_data)
		}
	}

	return nil
}

// 状态常量
const (
	state_SingleQuoteString = "s-quote"
	state_DoubleQuoteString = "d-quote"
	state_BacktickString    = "b-quote"
	state_jsonObj           = "json-object"
	state_data              = "data"
	state_reset             = "reset"
	state_quote             = "quote"

	// ex state
	state_objectKey      = "object-key"
	state_objectValue    = "object-value"
	state_jsonArray      = "json-array"
	state_arrayItem      = `json-array-item`
	state_primitiveValue = "primitive-value"
)
