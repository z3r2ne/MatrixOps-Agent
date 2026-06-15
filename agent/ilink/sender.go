package ilink

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
)

// generateClientID creates a random client ID for message correlation.
func generateClientID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SendTypingState sends a typing indicator to a user via the iLink sendtyping API.
// It first fetches a typing_ticket via getconfig, then sends the typing status.
func SendTypingState(ctx context.Context, client *Client, userID, contextToken string) error {
	configResp, err := client.GetConfig(ctx, userID, contextToken)
	if err != nil {
		return fmt.Errorf("get config for typing: %w", err)
	}
	if configResp.TypingTicket == "" {
		return fmt.Errorf("no typing_ticket returned from getconfig")
	}

	if err := client.SendTyping(ctx, userID, configResp.TypingTicket, TypingStatusTyping); err != nil {
		return fmt.Errorf("send typing: %w", err)
	}

	log.Printf("[sender] sent typing indicator to %s", userID)
	return nil
}

// CancelTypingState clears the typing indicator for a user.
func CancelTypingState(ctx context.Context, client *Client, userID, contextToken string) error {
	configResp, err := client.GetConfig(ctx, userID, contextToken)
	if err != nil {
		return fmt.Errorf("get config for typing cancel: %w", err)
	}
	if configResp.TypingTicket == "" {
		return fmt.Errorf("no typing_ticket returned from getconfig")
	}
	if err := client.SendTyping(ctx, userID, configResp.TypingTicket, TypingStatusCancel); err != nil {
		return fmt.Errorf("cancel typing: %w", err)
	}
	log.Printf("[sender] cancelled typing indicator for %s", userID)
	return nil
}

// SendTextReply sends a text reply to a user through the iLink API.
func SendTextReply(ctx context.Context, client *Client, toUserID, text, contextToken string) error {
	plainText := MarkdownToPlainText(text)

	req := &SendMessageRequest{
		Msg: SendMsg{
			FromUserID:   client.BotID(),
			ToUserID:     toUserID,
			ClientID:     generateClientID(),
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList: []MessageItem{
				{
					Type: ItemTypeText,
					TextItem: &TextItem{
						Text: plainText,
					},
				},
			},
			ContextToken: contextToken,
		},
		BaseInfo: BaseInfo{},
	}

	resp, err := client.SendMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	if resp.Ret != 0 {
		return fmt.Errorf("send message failed: ret=%d errmsg=%s", resp.Ret, resp.ErrMsg)
	}

	log.Printf("[sender] sent reply to %s", toUserID)
	return nil
}
