package util

import (
	"crypto/rand"
)

const slugAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func Slug() string {
	return randomString(8)
}

func randomString(length int) string {
	if length <= 0 {
		return ""
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = slugAlphabet[int(b)%len(slugAlphabet)]
	}
	return string(out)
}
