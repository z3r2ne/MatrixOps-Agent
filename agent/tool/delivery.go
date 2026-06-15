package tool

// UserDeliveryParams 是 message 工具向用户投递内容的参数。
type UserDeliveryParams struct {
	Text     string
	Media    string
	Buffer   string
	FilePath string
	Filename string
	MimeType string
	Caption  string
}

type DeliverUserMessageFunc func(ctx Context, params UserDeliveryParams) error
