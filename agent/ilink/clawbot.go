package ilink

import (
	"context"
	"fmt"
	"log"
)

// WechatClawbot is a high-level iLink WeChat bot client that wraps
// authentication, message receiving, and message sending.
type WechatClawbot struct {
	creds   *Credentials
	client  *Client
	monitor *Monitor

	// OnFatalSessionExpired is forwarded to the message monitor when set.
	OnFatalSessionExpired func()
}

// NewWechatClawbot creates a high-level client from existing credentials.
func NewWechatClawbot(creds *Credentials) *WechatClawbot {
	client := NewClient(creds)
	return &WechatClawbot{
		creds:  creds,
		client: client,
	}
}

// BotID returns the bot's iLink user ID.
func (w *WechatClawbot) BotID() string {
	return w.client.BotID()
}

// Client returns the underlying low-level iLink client.
func (w *WechatClawbot) Client() *Client {
	return w.client
}

// FetchQRCode retrieves a new login QR code.
func (w *WechatClawbot) FetchQRCode(ctx context.Context) (*QRCodeResponse, error) {
	return FetchQRCode(ctx)
}

// PollQRStatus polls the QR code status until confirmed or expired.
func (w *WechatClawbot) PollQRStatus(ctx context.Context, qrcode string, onStatus func(string)) (*Credentials, error) {
	return PollQRStatus(ctx, qrcode, onStatus)
}

// Login performs a full login: fetch QR code and poll until confirmed.
// The credentials are saved to disk automatically on success.
func (w *WechatClawbot) Login(ctx context.Context, onStatus func(string)) (*Credentials, error) {
	resp, err := FetchQRCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch QR code: %w", err)
	}
	if onStatus != nil {
		onStatus(fmt.Sprintf("qr_ready|%s", resp.QRCode))
	}

	creds, err := PollQRStatus(ctx, resp.QRCode, onStatus)
	if err != nil {
		return nil, fmt.Errorf("poll QR status: %w", err)
	}

	if err := SaveCredentials(creds); err != nil {
		log.Printf("[clawbot] warning: failed to save credentials: %v", err)
	}

	w.creds = creds
	w.client = NewClient(creds)
	return creds, nil
}

// LoadCredentials loads saved credentials from disk and re-initialises the client.
func (w *WechatClawbot) LoadCredentials() ([]*Credentials, error) {
	return LoadAllCredentials()
}

// SaveCredentials persists the current credentials to disk.
func (w *WechatClawbot) SaveCredentials() error {
	if w.creds == nil {
		return fmt.Errorf("no credentials to save")
	}
	return SaveCredentials(w.creds)
}

// StartReceiving begins the long-poll message monitor in a blocking manner.
// The handler is invoked for each incoming message in its own goroutine.
// Call this in a goroutine if you need to do other work concurrently.
func (w *WechatClawbot) StartReceiving(ctx context.Context, handler MessageHandler) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	mon, err := NewMonitor(w.client, handler)
	if err != nil {
		return fmt.Errorf("create monitor: %w", err)
	}
	w.monitor = mon
	mon.OnFatalSessionExpired = w.OnFatalSessionExpired
	return mon.Run(ctx)
}

// SendText sends a plain-text reply to a user.
func (w *WechatClawbot) SendText(ctx context.Context, toUserID, text, contextToken string) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	return SendTextReply(ctx, w.client, toUserID, text, contextToken)
}

// SendTyping sends a "typing" indicator to a user.
func (w *WechatClawbot) SendTyping(ctx context.Context, userID, contextToken string) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	return SendTypingState(ctx, w.client, userID, contextToken)
}

// SendMediaFromURL downloads a file from a URL and sends it as media.
func (w *WechatClawbot) SendMediaFromURL(ctx context.Context, toUserID, mediaURL, contextToken string) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	return SendMediaFromURL(ctx, w.client, toUserID, mediaURL, contextToken)
}

// SendMediaFromPath reads a local file and sends it as media.
func (w *WechatClawbot) SendMediaFromPath(ctx context.Context, toUserID, path, contextToken string) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	return SendMediaFromPath(ctx, w.client, toUserID, path, contextToken)
}

// SendMediaFromDataURL decodes a data: URL and sends it as media.
func (w *WechatClawbot) SendMediaFromDataURL(ctx context.Context, toUserID, dataURL, filename, contextToken string) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	return SendMediaFromDataURL(ctx, w.client, toUserID, dataURL, filename, contextToken)
}

// CancelTyping clears the typing indicator for a user.
func (w *WechatClawbot) CancelTyping(ctx context.Context, userID, contextToken string) error {
	if w.client == nil {
		return fmt.Errorf("client not initialised; login or load credentials first")
	}
	return CancelTypingState(ctx, w.client, userID, contextToken)
}

// GetConfig fetches bot config (including typing_ticket) for a user.
func (w *WechatClawbot) GetConfig(ctx context.Context, userID, contextToken string) (*GetConfigResponse, error) {
	if w.client == nil {
		return nil, fmt.Errorf("client not initialised; login or load credentials first")
	}
	return w.client.GetConfig(ctx, userID, contextToken)
}
