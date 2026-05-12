package downloader

import (
	"encoding/json"
	"strings"
)

// ImageSource 表示一张待发布图片的输入来源。
// 兼容旧版字符串输入：HTTP/HTTPS 视为 URL，其余视为本地路径。
type ImageSource struct {
	Type     string `json:"type,omitempty" jsonschema:"图片来源类型：url、path、base64"`
	URL      string `json:"url,omitempty" jsonschema:"HTTP/HTTPS 图片链接，type=url 时使用"`
	Path     string `json:"path,omitempty" jsonschema:"本地图片绝对路径，type=path 时使用"`
	Data     string `json:"data,omitempty" jsonschema:"Base64 图片数据，type=base64 时使用；支持纯 base64 或 data URL"`
	MIMEType string `json:"mime_type,omitempty" jsonschema:"图片 MIME 类型，如 image/png；base64 data URL 可省略"`
}

func NewImageSource(value string) ImageSource {
	if IsImageURL(value) {
		return ImageSource{Type: "url", URL: value}
	}
	return ImageSource{Type: "path", Path: value}
}

func (s *ImageSource) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*s = NewImageSource(value)
		return nil
	}

	type imageSource ImageSource
	var source imageSource
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}

	source.Type = strings.ToLower(strings.TrimSpace(source.Type))
	if source.Type == "" {
		source.Type = inferImageSourceType(ImageSource(source))
	}

	*s = ImageSource(source)
	return nil
}

func inferImageSourceType(source ImageSource) string {
	switch {
	case source.Data != "":
		return "base64"
	case source.URL != "":
		return "url"
	default:
		return "path"
	}
}
