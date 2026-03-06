package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultOutputFolder         = "xhs_transcripts"
	defaultProvider             = "dashscope"
	providerDashScope           = "dashscope"
	providerGLM                 = "glm"
	defaultDashScopeModel       = "qwen3.5-flash"
	defaultDashScopeEndpoint    = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	defaultDashScopeVideoFPS    = 2
	defaultGLMModel             = "glm-4.6v-flash"
	defaultGLMChatEndpoint      = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
	defaultGLMRequestTimeout    = 300 * time.Second
	defaultGLMRetryTimes        = 5
	maxSubtitleCharsPerLine     = 26
	defaultSubtitleSegmentSec   = 4
	defaultSubtitleLanguageHint = "auto"
)

var (
	transcriptFieldPattern = regexp.MustCompile(`(?s)"transcript_text"\s*:\s*"(.*?)"\s*,\s*"(?:srt_text|srt|language)"`)
	srtFieldPattern        = regexp.MustCompile(`(?s)"srt_text"\s*:\s*"(.*?)"\s*(?:,\s*"language"|\s*})`)
	srtTimestampPattern    = regexp.MustCompile(`\d{2}:\d{2}:\d{2},\d{3}\s+-->\s+\d{2}:\d{2}:\d{2},\d{3}`)
)

// Request 视频转写请求。
type Request struct {
	FeedID        string
	VideoURL      string
	Provider      string
	Model         string
	Language      string
	OutputDir     string
	KeepArtifacts bool
	APIKey        string
	Endpoint      string
}

// Result 视频转写结果。
type Result struct {
	FeedID           string
	TranscriptText   string
	TXTPath          string
	SRTPath          string
	OutputDir        string
	LanguageUsed     string
	ArtifactsCleaned bool
}

// RemoveArtifacts 删除临时文件。
func RemoveArtifacts(paths []string) error {
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// TranscribeVideo 执行视频转写。
func TranscribeVideo(ctx context.Context, req Request) (*Result, error) {
	if strings.TrimSpace(req.VideoURL) == "" {
		return nil, fmt.Errorf("VALIDATION_ERROR: video_url 不能为空")
	}

	provider, err := resolveProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	apiKey, err := resolveAPIKey(provider, req.APIKey)
	if err != nil {
		return nil, err
	}
	model := resolveModel(provider, req.Model)
	endpoint := resolveEndpoint(provider, req.Endpoint)

	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		feedID := strings.TrimSpace(req.FeedID)
		if feedID == "" {
			feedID = "unknown_feed"
		}
		outputDir = filepath.Join(os.TempDir(), defaultOutputFolder, feedID, time.Now().Format("20060102150405"))
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("TRANSCRIBE_ERROR: 创建输出目录失败: %w", err)
	}

	transcriptText, subtitleText, err := transcribeVideoViaProvider(ctx, req.VideoURL, provider, apiKey, model, req.Language, endpoint)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(transcriptText) == "" {
		return nil, fmt.Errorf("TRANSCRIBE_ERROR: 模型返回空转写文本")
	}
	if strings.TrimSpace(subtitleText) == "" {
		subtitleText = BuildFallbackSRT(transcriptText)
	}

	outputPrefix := filepath.Join(outputDir, "transcript")
	txtPath := outputPrefix + ".txt"
	srtPath := outputPrefix + ".srt"

	if err := os.WriteFile(txtPath, []byte(transcriptText), 0o644); err != nil {
		return nil, fmt.Errorf("TRANSCRIBE_ERROR: 写入 txt 结果失败: %w", err)
	}
	if err := os.WriteFile(srtPath, []byte(subtitleText), 0o644); err != nil {
		return nil, fmt.Errorf("TRANSCRIBE_ERROR: 写入 srt 结果失败: %w", err)
	}

	cleaned := !req.KeepArtifacts

	languageUsed := strings.TrimSpace(req.Language)
	if languageUsed == "" {
		languageUsed = defaultSubtitleLanguageHint
	}

	return &Result{
		FeedID:           req.FeedID,
		TranscriptText:   transcriptText,
		TXTPath:          txtPath,
		SRTPath:          srtPath,
		OutputDir:        outputDir,
		LanguageUsed:     languageUsed,
		ArtifactsCleaned: cleaned,
	}, nil
}

func resolveProvider(custom string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(custom))
	if provider == "" {
		if envProvider := strings.ToLower(strings.TrimSpace(os.Getenv("VIDEO_TRANSCRIBE_PROVIDER"))); envProvider != "" {
			provider = envProvider
		} else {
			provider = defaultProvider
		}
	}
	switch provider {
	case providerDashScope, providerGLM:
		return provider, nil
	default:
		return "", fmt.Errorf("VALIDATION_ERROR: provider 仅支持 dashscope 或 glm")
	}
}

func resolveAPIKey(provider, custom string) (string, error) {
	if trimmed := strings.TrimSpace(custom); trimmed != "" {
		return trimmed, nil
	}
	switch provider {
	case providerDashScope:
		if envKey := strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY")); envKey != "" {
			return envKey, nil
		}
		return "", fmt.Errorf("DEPENDENCY_ERROR: 未配置阿里云百炼 API Key，请设置 DASHSCOPE_API_KEY")
	case providerGLM:
		if envKey := strings.TrimSpace(os.Getenv("ZHIPUAI_API_KEY")); envKey != "" {
			return envKey, nil
		}
		if envKey := strings.TrimSpace(os.Getenv("BIGMODEL_API_KEY")); envKey != "" {
			return envKey, nil
		}
		return "", fmt.Errorf("DEPENDENCY_ERROR: 未配置 GLM API Key，请设置 ZHIPUAI_API_KEY 或 BIGMODEL_API_KEY")
	default:
		return "", fmt.Errorf("VALIDATION_ERROR: provider 仅支持 dashscope 或 glm")
	}
}

func resolveModel(provider, custom string) string {
	if trimmed := strings.TrimSpace(custom); trimmed != "" {
		return trimmed
	}
	switch provider {
	case providerDashScope:
		if envModel := strings.TrimSpace(os.Getenv("DASHSCOPE_VIDEO_MODEL")); envModel != "" {
			return envModel
		}
		return defaultDashScopeModel
	case providerGLM:
		if envModel := strings.TrimSpace(os.Getenv("GLM_VIDEO_MODEL")); envModel != "" {
			return envModel
		}
		return defaultGLMModel
	default:
		return defaultDashScopeModel
	}
}

func resolveEndpoint(provider, custom string) string {
	if trimmed := strings.TrimSpace(custom); trimmed != "" {
		return trimmed
	}
	switch provider {
	case providerDashScope:
		if envEndpoint := strings.TrimSpace(os.Getenv("DASHSCOPE_VIDEO_API_ENDPOINT")); envEndpoint != "" {
			return envEndpoint
		}
		return defaultDashScopeEndpoint
	case providerGLM:
		if envEndpoint := strings.TrimSpace(os.Getenv("GLM_VIDEO_API_ENDPOINT")); envEndpoint != "" {
			return envEndpoint
		}
		return defaultGLMChatEndpoint
	default:
		return defaultDashScopeEndpoint
	}
}

func transcribeVideoViaProvider(ctx context.Context, videoURL, provider, apiKey, model, language, endpoint string) (string, string, error) {
	prompt := BuildTranscribePrompt(language)
	payload, err := buildProviderPayload(provider, model, videoURL, prompt)
	if err != nil {
		return "", "", err
	}

	rawBody, err := callTranscribeAPI(ctx, provider, apiKey, endpoint, payload)
	if err != nil {
		return "", "", err
	}
	return parseGLMResponse(rawBody)
}

func buildProviderPayload(provider, model, videoURL, prompt string) ([]byte, error) {
	switch provider {
	case providerDashScope:
		payload := map[string]any{
			"model": model,
			"messages": []map[string]any{
				{
					"role": "user",
					"content": []map[string]any{
						{
							"type": "video_url",
							"video_url": map[string]string{
								"url": videoURL,
							},
							"fps": defaultDashScopeVideoFPS,
						},
						{
							"type": "text",
							"text": prompt,
						},
					},
				},
			},
			"stream": false,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("TRANSCRIBE_ERROR: 序列化 DashScope 请求失败: %w", err)
		}
		return payloadBytes, nil
	case providerGLM:
		payload := map[string]any{
			"model": model,
			"messages": []map[string]any{
				{
					"role": "user",
					"content": []map[string]any{
						{
							"type": "video_url",
							"video_url": map[string]string{
								"url": videoURL,
							},
						},
						{
							"type": "text",
							"text": prompt,
						},
					},
				},
			},
			"thinking": map[string]string{
				"type": "disabled",
			},
			"stream": false,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("TRANSCRIBE_ERROR: 序列化 GLM 请求失败: %w", err)
		}
		return payloadBytes, nil
	default:
		return nil, fmt.Errorf("VALIDATION_ERROR: provider 仅支持 dashscope 或 glm")
	}
}

func callTranscribeAPI(ctx context.Context, provider, apiKey, endpoint string, payloadBytes []byte) ([]byte, error) {
	client := &http.Client{Timeout: defaultGLMRequestTimeout}
	lastErr := fmt.Errorf("%s_API_ERROR: 未发起请求", strings.ToUpper(provider))
	for attempt := 1; attempt <= defaultGLMRetryTimes; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
		if err != nil {
			return nil, fmt.Errorf("%s_API_ERROR: 构建请求失败: %w", strings.ToUpper(provider), err)
		}
		request.Header.Set("Authorization", "Bearer "+apiKey)
		request.Header.Set("Content-Type", "application/json")

		response, err := client.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("%s_API_ERROR: 调用失败: %w", strings.ToUpper(provider), err)
			if attempt < defaultGLMRetryTimes {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			break
		}

		rawBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("%s_API_ERROR: 读取响应失败: %w", strings.ToUpper(provider), readErr)
			if attempt < defaultGLMRetryTimes {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			break
		}

		if response.StatusCode == http.StatusOK {
			return rawBody, nil
		}

		message := parseGLMError(rawBody)
		lastErr = fmt.Errorf("%s_API_ERROR: status=%d, message=%s", strings.ToUpper(provider), response.StatusCode, message)
		if (response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError) && attempt < defaultGLMRetryTimes {
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		break
	}
	return nil, lastErr
}

func BuildTranscribePrompt(language string) string {
	languageHint := strings.TrimSpace(language)
	if languageHint == "" {
		languageHint = "auto"
	}
	return fmt.Sprintf(`请分析视频中的语音内容并返回严格 JSON，不要输出 markdown 或解释文字。
JSON 格式如下：
{"transcript_text":"完整转写文本","srt_text":"标准SRT字幕","language":"识别语言"}
要求：
1) transcript_text 必须为完整文本，按自然段组织。
2) srt_text 必须为标准 SRT 格式（编号、时间戳、字幕行）。
3) 若无法准确生成时间戳，srt_text 可留空字符串。
4) 输出语言优先使用：%s。`, languageHint)
}

func parseGLMResponse(rawBody []byte) (string, string, error) {
	type apiError struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
	}
	type choice struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	}
	type response struct {
		Error   *apiError `json:"error"`
		Choices []choice  `json:"choices"`
	}

	var parsed response
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return "", "", fmt.Errorf("GLM_API_ERROR: 解析响应失败: %w", err)
	}
	if parsed.Error != nil {
		return "", "", fmt.Errorf("GLM_API_ERROR: %s", strings.TrimSpace(parsed.Error.Message))
	}
	if len(parsed.Choices) == 0 {
		return "", "", fmt.Errorf("GLM_API_ERROR: choices 为空")
	}

	rawContent := extractMessageContent(parsed.Choices[0].Message.Content)
	if strings.TrimSpace(rawContent) == "" {
		return "", "", fmt.Errorf("GLM_API_ERROR: 返回内容为空")
	}
	transcriptText, subtitleText, parseOK := parseStructuredTranscription(rawContent)
	if !parseOK {
		transcriptText = strings.TrimSpace(rawContent)
	}
	if strings.TrimSpace(transcriptText) == "" {
		return "", "", fmt.Errorf("GLM_API_ERROR: 解析后转写文本为空")
	}
	if strings.TrimSpace(subtitleText) == "" || !isLikelySRT(subtitleText) {
		subtitleText = BuildFallbackSRT(transcriptText)
	}
	return transcriptText, subtitleText, nil
}

func extractMessageContent(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		lines := make([]string, 0, len(value))
		for _, item := range value {
			switch typedItem := item.(type) {
			case string:
				lines = append(lines, strings.TrimSpace(typedItem))
			case map[string]any:
				if textValue, ok := typedItem["text"].(string); ok {
					lines = append(lines, strings.TrimSpace(textValue))
				}
			}
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func parseStructuredTranscription(raw string) (string, string, bool) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	startIndex := strings.Index(cleaned, "{")
	endIndex := strings.LastIndex(cleaned, "}")
	if startIndex >= 0 && endIndex > startIndex {
		cleaned = cleaned[startIndex : endIndex+1]
	}

	type payload struct {
		TranscriptText string `json:"transcript_text"`
		SRTText        string `json:"srt_text"`
		Transcript     string `json:"transcript"`
		SRT            string `json:"srt"`
		Text           string `json:"text"`
	}
	var parsed payload
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		transcriptText := extractFieldByPattern(cleaned, transcriptFieldPattern)
		subtitleText := extractFieldByPattern(cleaned, srtFieldPattern)
		if transcriptText == "" {
			return "", "", false
		}
		return transcriptText, subtitleText, true
	}

	transcriptText := strings.TrimSpace(parsed.TranscriptText)
	if transcriptText == "" {
		transcriptText = strings.TrimSpace(parsed.Transcript)
	}
	if transcriptText == "" {
		transcriptText = strings.TrimSpace(parsed.Text)
	}
	if transcriptText == "" {
		return "", "", false
	}

	subtitleText := strings.TrimSpace(parsed.SRTText)
	if subtitleText == "" {
		subtitleText = strings.TrimSpace(parsed.SRT)
	}
	return transcriptText, subtitleText, true
}

func extractFieldByPattern(input string, pattern *regexp.Regexp) string {
	matches := pattern.FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	value := strings.TrimSpace(matches[1])
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, `\t`, "\t")
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `\\`, `\`)
	return strings.TrimSpace(value)
}

func BuildFallbackSRT(transcript string) string {
	cleanedTranscript := strings.TrimSpace(transcript)
	if cleanedTranscript == "" {
		return ""
	}
	normalized := strings.ReplaceAll(cleanedTranscript, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = regexp.MustCompile(`\n+`).ReplaceAllString(normalized, "\n")

	parts := make([]string, 0)
	for _, paragraph := range strings.Split(normalized, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		parts = append(parts, splitByRuneLength(paragraph, maxSubtitleCharsPerLine)...)
	}
	if len(parts) == 0 {
		parts = append(parts, splitByRuneLength(normalized, maxSubtitleCharsPerLine)...)
	}

	var builder strings.Builder
	startSeconds := 0
	for index, textLine := range parts {
		if strings.TrimSpace(textLine) == "" {
			continue
		}
		endSeconds := startSeconds + defaultSubtitleSegmentSec
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString("\n")
		builder.WriteString(formatSRTTimestamp(startSeconds))
		builder.WriteString(" --> ")
		builder.WriteString(formatSRTTimestamp(endSeconds))
		builder.WriteString("\n")
		builder.WriteString(textLine)
		builder.WriteString("\n\n")
		startSeconds = endSeconds
	}
	return strings.TrimSpace(builder.String()) + "\n"
}

func splitByRuneLength(text string, maxLength int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || maxLength <= 0 {
		return nil
	}
	runes := []rune(trimmed)
	if len(runes) <= maxLength {
		return []string{trimmed}
	}

	lines := make([]string, 0, len(runes)/maxLength+1)
	startIndex := 0
	for startIndex < len(runes) {
		endIndex := startIndex + maxLength
		if endIndex > len(runes) {
			endIndex = len(runes)
		}
		lines = append(lines, strings.TrimSpace(string(runes[startIndex:endIndex])))
		startIndex = endIndex
	}
	return lines
}

func formatSRTTimestamp(totalSeconds int) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d,000", hours, minutes, seconds)
}

func parseGLMError(rawBody []byte) string {
	type apiError struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	var parsed apiError
	if err := json.Unmarshal(rawBody, &parsed); err == nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return strings.TrimSpace(parsed.Error.Message)
	}
	bodyText := strings.TrimSpace(string(rawBody))
	if len(bodyText) > 200 {
		return bodyText[:200]
	}
	return bodyText
}

func isLikelySRT(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	return srtTimestampPattern.MatchString(trimmed)
}
