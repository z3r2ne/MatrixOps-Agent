package ilink

import (
	"encoding/base64"
	"testing"
)

func TestParseDataURL(t *testing.T) {
	raw := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("hello"))
	mimeType, data, err := ParseDataURL(raw)
	if err != nil {
		t.Fatalf("ParseDataURL: %v", err)
	}
	if mimeType != "text/plain" {
		t.Fatalf("mime = %q", mimeType)
	}
	if string(data) != "hello" {
		t.Fatalf("data = %q", string(data))
	}
}

func TestParseInboundMessage_TextOnly(t *testing.T) {
	msg := WeixinMessage{
		MessageType: MessageTypeUser,
		ItemList: []MessageItem{{
			Type:     ItemTypeText,
			TextItem: &TextItem{Text: "你好"},
		}},
	}
	inbound, err := ParseInboundMessage(t.Context(), msg)
	if err != nil {
		t.Fatalf("ParseInboundMessage: %v", err)
	}
	if inbound.Text != "你好" {
		t.Fatalf("text = %q", inbound.Text)
	}
	if len(inbound.Attachments) != 0 {
		t.Fatalf("attachments = %d", len(inbound.Attachments))
	}
}

func TestDefaultInboundText(t *testing.T) {
	text := defaultInboundText([]InboundAttachment{{Kind: "image", Filename: "a.png"}})
	if text != "[用户发送图片]" {
		t.Fatalf("text = %q", text)
	}
}
