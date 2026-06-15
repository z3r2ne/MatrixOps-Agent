package jsonextractor

import (
	"fmt"
	"strings"
)

type bufStackKv struct {
	key any
	val any
}

type bufStack struct {
	isRoot       bool
	key          any
	parent       *bufStack
	kv           func(key any, val any)
	currentStack *Stack
	recorders    []*bufStackKv
	// 字段流上下文，绑定到当前栈层级
	fieldStreamContexts []*fieldStreamContext
}

type bufStackManager struct {
	stack           *Stack
	base            *bufStack
	callbackManager *callbackManager
}

func newBufStackManager(kv func(key any, val any, parents []string)) *bufStackManager {
	manager := &bufStackManager{
		stack: NewStack(),
	}
	manager.base = &bufStack{
		isRoot: true,
		kv: func(key any, val any) {
			kv(key, val, manager.getParentPath())
		},
		currentStack: NewStack(),
		recorders:    []*bufStackKv{},
	}
	manager.stack.Push(manager.base)
	return manager
}

func (m *bufStackManager) setCallbackManager(cm *callbackManager) {
	m.callbackManager = cm
}

func (m *bufStackManager) getCurrentKey() any {
	if m.base != nil && m.base.currentStack != nil {
		return m.base.currentStack.PeekN(1)
	}
	return nil
}

func (m *bufStackManager) prepareFieldStreamContexts(key string) {
	if m.callbackManager == nil {
		return
	}
	if len(m.base.fieldStreamContexts) > 0 {
		return
	}
	contexts := m.callbackManager.handleFieldStreamStart(key, m)
	if len(contexts) > 0 {
		m.base.fieldStreamContexts = contexts
	}
}

func (m *bufStackManager) PushKey(v any) {
	switch ret := v.(type) {
	case []byte:
		keyStr := string(ret)
		m.base.PushKey(keyStr)
		// 如果尚未准备字段流上下文，则现在准备
		m.prepareFieldStreamContexts(keyStr)
	case string:
		m.base.PushKey(ret)
		// 如果尚未准备字段流上下文，则现在准备
		m.prepareFieldStreamContexts(ret)
	case int:
		m.base.PushKey(ret)
		// 数组索引不需要字段流处理
		m.base.fieldStreamContexts = nil
	}
}

// activatePendingFieldWriter 激活待处理的字段写入器
func (m *bufStackManager) activatePendingFieldWriter() *fieldStreamFrame {
	if len(m.base.fieldStreamContexts) > 0 && m.callbackManager != nil {
		frame := m.callbackManager.pushFieldStreamFrame(m.base.fieldStreamContexts)
		m.base.fieldStreamContexts = nil
		return frame
	}
	return nil
}

// getParentPath 从 stack 中获取父路径
func (m *bufStackManager) getParentPath() []string {
	parents := make([]string, 0)

	// 从 stack 遍历父路径
	current := m.base
	for current != nil && !current.isRoot {
		if current.key != nil {
			if keyStr, ok := current.key.(string); ok {
				// 清理键名中的引号和空格
				cleanKey := strings.Trim(strings.TrimSpace(keyStr), `"`)
				// 将父路径插入到开头，保持正确的顺序
				parents = append([]string{cleanKey}, parents...)
			}
		}
		current = current.parent
	}

	return parents
}

func (m *bufStackManager) getPrefixKey() []string { // get parent path and current path prefix key
	prefix := m.getParentPath()

	// 需要检查当前正在处理的键
	if m.base != nil && m.base.currentStack != nil {
		// 获取 stack 中的所有键，除了最后一个（当前正在处理的值）
		size := m.base.currentStack.Len()
		for i := 0; i < size-1; i++ {
			if key := m.base.currentStack.PeekN(size - i); key != nil {
				if keyStr, ok := key.(string); ok {
					// 清理键名中的引号和空格
					cleanKey := strings.Trim(strings.TrimSpace(keyStr), `"`)
					prefix = append(prefix, cleanKey)
				}
			}
		}
	}

	return prefix
}

func (m *bufStackManager) PushValue(v string) {
	// 字符级流式写入现在在状态机中处理，这里不再写入
	// 清理当前栈的字段写入器（如果有的话）
	if len(m.base.fieldStreamContexts) > 0 {
		m.base.fieldStreamContexts = nil
	}
	m.base.PushValue(v)
}

func (m *bufStackManager) PushContainer() {
	var keyRaw any
	if ret := m.base.currentStack.Peek(); ret != nil {
		keyRaw = ret
	}
	sub := &bufStack{
		isRoot:       false,
		key:          keyRaw,
		parent:       m.base,
		kv:           m.base.kv,
		currentStack: NewStack(),
		recorders:    []*bufStackKv{},
		// 继承父栈的字段写入器
		fieldStreamContexts: m.base.fieldStreamContexts,
	}
	m.base = sub
	m.stack.Push(sub)
}

func (m *bufStackManager) PopContainer() {
	sub := m.stack.Pop()
	if sub != nil {
		if subSubStack, ok := sub.(*bufStack); ok {
			m.base = subSubStack.parent
			result := make(map[any]any)
			for _, v := range subSubStack.recorders {
				result[v.key] = v.val
			}
			m.base.emit(subSubStack.key, result)
			m.base.recorders = append(m.base.recorders, &bufStackKv{
				key: subSubStack.key,
				val: result,
			})
			
			// 触发对象/数组回调
			if m.callbackManager != nil {
				// 检查是否是数组（键都是整数）
				isArray := true
				for k := range result {
					if _, ok := k.(int); !ok {
						isArray = false
						break
					}
				}
				
				if isArray {
					// 转换为数组
					arr := make([]any, 0)
					for i := 0; ; i++ {
						if v, ok := result[i]; ok {
							arr = append(arr, v)
						} else {
							break
						}
					}
					if m.callbackManager.onArrayCallback != nil {
						m.callbackManager.onArrayCallback(arr)
					}
				} else {
					// 转换为 map[string]any
					strMap := make(map[string]any)
					for k, v := range result {
						if keyStr, ok := k.(string); ok {
							strMap[keyStr] = v
						}
					}
					if m.callbackManager.onObjectCallback != nil {
						m.callbackManager.onObjectCallback(strMap)
					}
					// 触发条件回调
					for _, cb := range m.callbackManager.onConditionalObjectCallback {
						cb.Feed(strMap)
					}
				}
			}
		}
	}
}

func (b *bufStack) emit(k any, v any) {
	if b.kv != nil {
		b.kv(k, v)
		return
	}
	// 静默处理，不输出日志
	_ = fmt.Sprintf("emit: %v, %v", k, v)
}

func (b *bufStack) PushKey(v any) {
	b.currentStack.Push(v)
}

func (b *bufStack) PushValue(v string) {
	// 获取当前的键（栈顶元素）
	keyRaw := b.currentStack.Peek()
	
	// 发送键值对
	b.emit(keyRaw, v)
	
	// 记录键值对
	b.recorders = append(b.recorders, &bufStackKv{
		key: keyRaw,
		val: v,
	})
	
	// 弹出键，因为这个键值对已经处理完了
	b.currentStack.Pop()
}

func (m *bufStackManager) TriggerEmit() {
	b := m.base
	for {
		if b.isRoot {
			break
		}
		b = b.parent
	}
	finalResult := make(map[any]any)
	for _, item := range b.recorders {
		finalResult[item.key] = item.val
	}
	b.emit("", finalResult)
}
