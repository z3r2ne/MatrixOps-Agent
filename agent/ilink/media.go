package ilink

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var reMarkdownImage = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

// ExtractImageURLs extracts image URLs from markdown text.
func ExtractImageURLs(text string) []string {
	matches := reMarkdownImage.FindAllStringSubmatch(text, -1)
	var urls []string
	for _, m := range matches {
		url := strings.TrimSpace(m[1])
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			urls = append(urls, url)
		}
	}
	return urls
}

// SendMediaFromURL downloads a file from a URL and sends it as a media message.
func SendMediaFromURL(ctx context.Context, client *Client, toUserID, mediaURL, contextToken string) error {
	data, contentType, err := downloadFile(ctx, mediaURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", mediaURL, err)
	}

	return sendMediaData(ctx, client, toUserID, filenameFromURL(mediaURL), mediaURL, data, contentType, contextToken)
}

// SendMediaFromPath reads a local file and sends it as a media message.
func SendMediaFromPath(ctx context.Context, client *Client, toUserID, path, contextToken string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return sendMediaData(ctx, client, toUserID, filepath.Base(path), path, data, inferContentType(path), contextToken)
}

// SendMediaFromBytes sends raw bytes as a media message.
func SendMediaFromBytes(ctx context.Context, client *Client, toUserID, fileName string, data []byte, contentType, contextToken string) error {
	if len(data) == 0 {
		return fmt.Errorf("empty media data")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(data)
	}
	return sendMediaData(ctx, client, toUserID, fileName, "bytes", data, contentType, contextToken)
}

// SendMediaFromDataURL decodes a data: URL and sends it as media.
func SendMediaFromDataURL(ctx context.Context, client *Client, toUserID, dataURL, filename, contextToken string) error {
	contentType, data, err := ParseDataURL(dataURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(filename) == "" {
		filename = "attachment"
	}
	return SendMediaFromBytes(ctx, client, toUserID, filename, data, contentType, contextToken)
}

// ParseDataURL decodes a data: URL into MIME type and bytes.
func ParseDataURL(rawURL string) (contentType string, data []byte, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "data:") {
		return "", nil, fmt.Errorf("not a data URL")
	}
	comma := strings.Index(rawURL, ",")
	if comma < 0 {
		return "", nil, fmt.Errorf("invalid data URL")
	}
	meta := rawURL[5:comma]
	payload := rawURL[comma+1:]
	if strings.HasSuffix(meta, ";base64") {
		contentType = strings.TrimSuffix(meta, ";base64")
		data, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return "", nil, fmt.Errorf("decode data URL: %w", err)
		}
		return contentType, data, nil
	}
	contentType = meta
	return contentType, []byte(payload), nil
}

func sendMediaData(ctx context.Context, client *Client, toUserID, fileName, source string, data []byte, contentType, contextToken string) error {
	if fileName == "" {
		fileName = "file"
	}

	cdnMediaType, itemType := classifyMedia(contentType, source)

	log.Printf("[media] uploading %s (%s, %d bytes) for %s", source, contentType, len(data), toUserID)

	uploaded, err := UploadFileToCDN(ctx, client, data, toUserID, cdnMediaType)
	if err != nil {
		return fmt.Errorf("upload to CDN: %w", err)
	}

	media := &MediaInfo{
		EncryptQueryParam: uploaded.DownloadParam,
		AESKey:            AESKeyToBase64(uploaded.AESKeyHex),
		EncryptType:       1,
	}

	var item MessageItem
	switch itemType {
	case ItemTypeImage:
		item = MessageItem{
			Type: ItemTypeImage,
			ImageItem: &ImageItem{
				Media:   media,
				MidSize: uploaded.CipherSize,
			},
		}
	case ItemTypeVideo:
		item = MessageItem{
			Type: ItemTypeVideo,
			VideoItem: &VideoItem{
				Media:     media,
				VideoSize: uploaded.CipherSize,
			},
		}
	default:
		item = MessageItem{
			Type: ItemTypeFile,
			FileItem: &FileItem{
				Media:    media,
				FileName: fileName,
				Len:      fmt.Sprintf("%d", uploaded.FileSize),
			},
		}
	}

	req := &SendMessageRequest{
		Msg: SendMsg{
			FromUserID:   client.BotID(),
			ToUserID:     toUserID,
			ClientID:     generateClientID(),
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList:     []MessageItem{item},
			ContextToken: contextToken,
		},
		BaseInfo: BaseInfo{},
	}

	resp, err := client.SendMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("send media message: %w", err)
	}
	if resp.Ret != 0 {
		return fmt.Errorf("send media failed: ret=%d errmsg=%s", resp.Ret, resp.ErrMsg)
	}

	log.Printf("[media] sent %s to %s from %s", contentType, toUserID, source)
	return nil
}

func downloadFile(ctx context.Context, url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = inferContentType(url)
	}

	return data, contentType, nil
}

func classifyMedia(contentType, url string) (cdnMediaType int, itemType int) {
	ct := strings.ToLower(contentType)

	if strings.HasPrefix(ct, "image/") || isImageExt(url) {
		return CDNMediaTypeImage, ItemTypeImage
	}
	if strings.HasPrefix(ct, "video/") || isVideoExt(url) {
		return CDNMediaTypeVideo, ItemTypeVideo
	}
	return CDNMediaTypeFile, ItemTypeFile
}

func isImageExt(url string) bool {
	ext := strings.ToLower(filepath.Ext(stripQuery(url)))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func isVideoExt(url string) bool {
	ext := strings.ToLower(filepath.Ext(stripQuery(url)))
	switch ext {
	case ".mp4", ".mov", ".webm", ".mkv", ".avi":
		return true
	}
	return false
}

func inferContentType(url string) string {
	ext := filepath.Ext(stripQuery(url))
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func filenameFromURL(rawURL string) string {
	u := stripQuery(rawURL)
	name := filepath.Base(u)
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}

func stripQuery(rawURL string) string {
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		return rawURL[:i]
	}
	return rawURL
}
