package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// ImageText 保留每张原图的识别结果，不跨图合并或去重。
type ImageText struct {
	Index      int       `json:"index"`
	URL        string    `json:"url"`
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	Lines      []OCRLine `json:"lines,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type OCRLine struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

func enrichFeedText(ctx context.Context, response *FeedDetailResponse, enabled bool, recognize func(context.Context, []string) ([]ImageText, error)) {
	detail, ok := response.Data.(*xiaohongshu.FeedDetailResponse)
	if !ok || detail == nil {
		return
	}
	sections := []string{"# " + detail.Note.Title, "## 正文\n" + detail.Note.Desc}
	if enabled && len(detail.Note.ImageList) > 0 {
		urls := make([]string, len(detail.Note.ImageList))
		for i, img := range detail.Note.ImageList {
			urls[i] = img.URLDefault
			if urls[i] == "" {
				urls[i] = img.URLPre
			}
		}
		images, err := recognize(ctx, urls)
		if err == nil && len(images) != len(urls) {
			err = fmt.Errorf("图片识别结果不完整：应返回%d张，实际%d张", len(urls), len(images))
		}
		if err != nil {
			response.OCRError = err.Error()
			sections = append(sections, "## 图片文字\n识别未完成："+err.Error())
		} else {
			response.ImageTexts = images
			for i := range response.ImageTexts {
				image := &response.ImageTexts[i]
				// 图片编号和地址由原始详情决定，防止错配来源。
				image.Index, image.URL = i+1, urls[i]
				text := image.Text
				if image.Error != "" {
					text = "识别失败：" + image.Error
					response.OCRError = "部分图片识别失败，详见image_texts"
				} else if strings.TrimSpace(text) == "" {
					text = "未识别到文字"
				}
				sections = append(sections, fmt.Sprintf("## 图片 %d\n%s", i+1, text))
			}
		}
	}
	response.TextContent = strings.Join(sections, "\n\n")
}

func runLocalOCR(ctx context.Context, urls []string) ([]ImageText, error) {
	pythonPath, scriptPath := os.Getenv("XHS_OCR_PYTHON"), os.Getenv("XHS_OCR_SCRIPT")
	if pythonPath == "" || scriptPath == "" {
		return nil, fmt.Errorf("本地OCR未配置，需要设置XHS_OCR_PYTHON和XHS_OCR_SCRIPT")
	}
	input, err := json.Marshal(map[string]any{"urls": urls})
	if err != nil {
		return nil, err
	}
	ocrCtx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ocrCtx, pythonPath, "-X", "utf8", scriptPath)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ocrCtx.Err() != nil {
			return nil, fmt.Errorf("图片文字识别被取消或超时：%w", ocrCtx.Err())
		}
		return nil, fmt.Errorf("本地OCR进程启动或执行失败：%w", err)
	}
	var result struct {
		Images []ImageText `json:"images"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("本地OCR返回了无效结果：%w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("本地OCR不可用：%s", result.Error)
	}
	return result.Images, nil
}
