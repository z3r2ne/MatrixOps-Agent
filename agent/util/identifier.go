package util

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const maxTimestamp = int64(^uint64(0) >> 1)

func Ascending(prefix string) string {
	return formatID(prefix, time.Now().UnixNano(), false)
}

func Descending(prefix string) string {
	return formatID(prefix, time.Now().UnixNano(), true)
}

func Timestamp(id string) (int64, bool) {
	parts := strings.Split(id, "_")
	if len(parts) < 2 {
		return 0, false
	}
	raw := parts[1]
	if len(raw) < 19 {
		return 0, false
	}
	value, err := strconv.ParseInt(raw[:19], 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func formatID(prefix string, ts int64, descending bool) string {
	if ts < 0 {
		ts = 0
	}
	if descending {
		ts = maxTimestamp - ts
	}
	nonce := randomHex(4)
	return fmt.Sprintf("%s_%019d_%s", prefix, ts, nonce)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "0000"
	}
	out := make([]byte, 0, n*2)
	for _, b := range buf {
		out = append(out, hexByte(b)...)
	}
	return string(out)
}

func hexByte(b byte) []byte {
	const hex = "0123456789abcdef"
	return []byte{hex[b>>4], hex[b&0x0f]}
}
