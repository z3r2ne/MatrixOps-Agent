package jsonextractor

// Stack 简单的栈实现
type Stack struct {
	items []interface{}
}

// New 创建新的栈
func NewStack() *Stack {
	return &Stack{
		items: make([]interface{}, 0),
	}
}

// Push 压栈
func (s *Stack) Push(item interface{}) {
	s.items = append(s.items, item)
}

// Pop 出栈
func (s *Stack) Pop() interface{} {
	if len(s.items) == 0 {
		return nil
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}

// Peek 查看栈顶元素（不弹出）
func (s *Stack) Peek() interface{} {
	if len(s.items) == 0 {
		return nil
	}
	return s.items[len(s.items)-1]
}

// PeekN 查看栈中倒数第 n 个元素（1-based，1表示栈顶）
func (s *Stack) PeekN(n int) interface{} {
	if n <= 0 || n > len(s.items) {
		return nil
	}
	return s.items[len(s.items)-n]
}

// Len 返回栈的长度
func (s *Stack) Len() int {
	return len(s.items)
}

// IsEmpty 判断栈是否为空
func (s *Stack) IsEmpty() bool {
	return len(s.items) == 0
}
