package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDecodeImageData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr string
	}{
		{
			name:  "data URL",
			input: "data:image/png;base64,dGVzdA==",
			want:  []byte("test"),
		},
		{
			name:  "raw base64",
			input: "dGVzdA==",
			want:  []byte("test"),
		},
		{
			name:    "invalid data URL",
			input:   "data:image/png,dGVzdA==",
			wantErr: "图片数据格式无效",
		},
		{
			name:    "empty data",
			input:   "",
			wantErr: "图片数据为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeImageData(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v, want error containing %q", err, tt.wantErr)
				}
				if tt.input != "" && strings.Contains(err.Error(), tt.input) {
					t.Fatal("error must not expose image data")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("decoded data=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeImageDataDoesNotExposeInvalidPayload(t *testing.T) {
	const payload = "secret-qr-payload"
	_, err := decodeImageData(payload)
	if err == nil {
		t.Fatal("expected invalid base64 error")
	}
	if strings.Contains(err.Error(), payload) {
		t.Fatal("error must not expose image data")
	}
}

func TestConvertToMCPResultDecodesImageDataURL(t *testing.T) {
	result := convertToMCPResult(&MCPToolResult{Content: []MCPContent{{
		Type:     "image",
		MimeType: "image/png",
		Data:     "data:image/png;base64,dGVzdA==",
	}}})

	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("result=%+v", result)
	}
	image, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content type=%T", result.Content[0])
	}
	if !bytes.Equal(image.Data, []byte("test")) || image.MIMEType != "image/png" {
		t.Fatalf("image=%+v", image)
	}
}

func TestConvertToMCPResultMarksInvalidImageAsError(t *testing.T) {
	const payload = "secret-qr-payload"
	result := convertToMCPResult(&MCPToolResult{Content: []MCPContent{{Type: "image", Data: payload}}})

	if !result.IsError || len(result.Content) != 1 {
		t.Fatalf("result=%+v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type=%T", result.Content[0])
	}
	if !strings.Contains(text.Text, "图片数据解码失败") || strings.Contains(text.Text, payload) {
		t.Fatalf("unsafe error text: %q", text.Text)
	}
}
