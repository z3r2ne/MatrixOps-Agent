package coreagent

import (
	"crypto/rand"
	"fmt"
	"time"
)

// IDGenerator generates IDs for messages, parts, and tool calls.
type IDGenerator func(prefix string) string

// DefaultIDGenerator matches the existing project convention closely enough for adapters.
func DefaultIDGenerator(prefix string) string {
	return fmt.Sprintf("%s_%019d_%s", prefix, time.Now().UnixNano(), randomHex(4))
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "0000"
	}
	out := make([]byte, 0, n*2)
	const hex = "0123456789abcdef"
	for _, b := range buf {
		out = append(out, hex[b>>4], hex[b&0x0f])
	}
	return string(out)
}
