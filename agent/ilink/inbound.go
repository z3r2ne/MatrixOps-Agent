package ilink

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// InboundAttachment is a decoded media attachment from a WeChat message.
type InboundAttachment struct {
	Filename string
	MimeType string
	Data     []byte
	Kind     string // image, video, file
}

// InboundMessage is parsed user input from a WeChat message.
type InboundMessage struct {
	Text        string
	Attachments []InboundAttachment
}

func (m InboundMessage) HasContent() bool {
	if m.Text != "" {
		return true
	}
	return len(m.Attachments) > 0
}

// ParseInboundMessage extracts text and downloadable media from a user message.
func ParseInboundMessage(ctx context.Context, msg WeixinMessage) (InboundMessage, error) {
	out := InboundMessage{}
	if msg.MessageType != MessageTypeUser {
		return out, nil
	}

	for _, item := range msg.ItemList {
		switch item.Type {
		case ItemTypeText:
			if item.TextItem != nil {
				text := strings.TrimSpace(item.TextItem.Text)
				if text != "" {
					if out.Text != "" {
						out.Text += "\n"
					}
					out.Text += text
				}
			}
		case ItemTypeVoice:
			if item.VoiceItem != nil {
				text := strings.TrimSpace(item.VoiceItem.Text)
				if text != "" {
					if out.Text != "" {
						out.Text += "\n"
					}
					out.Text += text
				}
			}
		case ItemTypeImage:
			attachment, err := downloadInboundItem(ctx, "image", "image.jpg", imageCarrier{item: item.ImageItem})
			if err != nil {
				return out, err
			}
			if attachment != nil {
				out.Attachments = append(out.Attachments, *attachment)
			}
		case ItemTypeVideo:
			attachment, err := downloadInboundItem(ctx, "video", "video.mp4", videoCarrier{item: item.VideoItem})
			if err != nil {
				return out, err
			}
			if attachment != nil {
				out.Attachments = append(out.Attachments, *attachment)
			}
		case ItemTypeFile:
			name := "file"
			if item.FileItem != nil && strings.TrimSpace(item.FileItem.FileName) != "" {
				name = strings.TrimSpace(item.FileItem.FileName)
			}
			attachment, err := downloadInboundItem(ctx, "file", name, fileCarrier{item: item.FileItem})
			if err != nil {
				return out, err
			}
			if attachment != nil {
				out.Attachments = append(out.Attachments, *attachment)
			}
		}
	}

	if out.Text == "" && len(out.Attachments) > 0 {
		out.Text = defaultInboundText(out.Attachments)
	}
	return out, nil
}

type mediaCarrier interface {
	getMedia() *MediaInfo
}

type imageCarrier struct{ item *ImageItem }

func (c imageCarrier) getMedia() *MediaInfo {
	if c.item == nil {
		return nil
	}
	return c.item.Media
}

type videoCarrier struct{ item *VideoItem }

func (c videoCarrier) getMedia() *MediaInfo {
	if c.item == nil {
		return nil
	}
	return c.item.Media
}

type fileCarrier struct{ item *FileItem }

func (c fileCarrier) getMedia() *MediaInfo {
	if c.item == nil {
		return nil
	}
	return c.item.Media
}

func downloadInboundItem(ctx context.Context, kind, fallbackName string, carrier mediaCarrier) (*InboundAttachment, error) {
	if carrier == nil {
		return nil, nil
	}
	media := carrier.getMedia()
	if media == nil || strings.TrimSpace(media.EncryptQueryParam) == "" {
		return nil, nil
	}
	data, err := DownloadFileFromCDN(ctx, media.EncryptQueryParam, media.AESKey)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", kind, err)
	}
	filename := fallbackName
	if fc, ok := carrier.(fileCarrier); ok && fc.item != nil && strings.TrimSpace(fc.item.FileName) != "" {
		filename = strings.TrimSpace(fc.item.FileName)
	}
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	return &InboundAttachment{
		Filename: filename,
		MimeType: mimeType,
		Data:     data,
		Kind:     kind,
	}, nil
}

func defaultInboundText(attachments []InboundAttachment) string {
	if len(attachments) == 1 {
		switch attachments[0].Kind {
		case "image":
			return "[用户发送图片]"
		case "video":
			return "[用户发送视频]"
		default:
			name := strings.TrimSpace(attachments[0].Filename)
			if name == "" {
				return "[用户发送文件]"
			}
			return fmt.Sprintf("[用户发送文件: %s]", name)
		}
	}
	return fmt.Sprintf("[用户发送 %d 个附件]", len(attachments))
}
