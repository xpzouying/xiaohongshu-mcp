package xiaohongshu

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ShortLinkResult 短链接解析结果
type ShortLinkResult struct {
	FeedID      string `json:"feed_id"`
	XsecToken   string `json:"xsec_token"`
	OriginalURL string `json:"original_url"`
	RedirectURL string `json:"redirect_url"`
	ShareID     string `json:"share_id,omitempty"`
	AppPlatform string `json:"app_platform,omitempty"`
	XsecSource  string `json:"xsec_source,omitempty"`
}

// 短链接域名白名单
var shortLinkDomains = []string{
	"xhslink.com",
}

// 移动端 User-Agent，模拟 iOS Chrome 访问
const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1"

// ResolveShortLink 解析小红书短链接，提取 feed_id 和 xsec_token
func ResolveShortLink(ctx context.Context, shortURL string) (*ShortLinkResult, error) {
	// 标准化 URL
	normalizedURL, err := normalizeShortLinkURL(shortURL)
	if err != nil {
		return nil, err
	}

	logrus.Infof("解析短链接: %s", normalizedURL)

	// 创建 HTTP Client，禁止自动跟随重定向
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 禁止自动跟随重定向，我们需要手动获取 Location header
			return http.ErrUseLastResponse
		},
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", normalizedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置移动端 Headers（关键！小红书会根据 UA 返回不同响应）
	req.Header.Set("User-Agent", mobileUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh-Hans;q=0.9")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求短链接失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查是否为重定向响应
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return nil, fmt.Errorf("短链接未返回重定向，状态码: %d", resp.StatusCode)
	}

	// 获取重定向 URL
	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("短链接响应中缺少 Location header")
	}

	logrus.Infof("重定向到: %s", location)

	// 解析重定向 URL 提取参数
	return parseRedirectURL(location, normalizedURL)
}

// normalizeShortLinkURL 标准化短链接 URL
func normalizeShortLinkURL(input string) (string, error) {
	input = strings.TrimSpace(input)

	// 如果不是 URL 格式，尝试添加协议
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		// 检查是否是短链接格式 (xhslink.com/xxx)
		for _, domain := range shortLinkDomains {
			if strings.HasPrefix(input, domain) {
				input = "https://" + input
				break
			}
		}
	}

	// 解析 URL
	parsedURL, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("无效的 URL 格式: %w", err)
	}

	// 验证是否为支持的短链接域名
	isValidDomain := false
	for _, domain := range shortLinkDomains {
		if parsedURL.Host == domain || strings.HasSuffix(parsedURL.Host, "."+domain) {
			isValidDomain = true
			break
		}
	}

	if !isValidDomain {
		return "", fmt.Errorf("不支持的短链接域名: %s，支持的域名: %v", parsedURL.Host, shortLinkDomains)
	}

	// 确保使用 HTTPS
	parsedURL.Scheme = "https"

	return parsedURL.String(), nil
}

// parseRedirectURL 解析重定向 URL，提取 feed_id 和 xsec_token
func parseRedirectURL(redirectURL, originalURL string) (*ShortLinkResult, error) {
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		return nil, fmt.Errorf("解析重定向 URL 失败: %w", err)
	}

	// 从路径中提取 feed_id
	// 格式: /discovery/item/{feed_id} 或 /explore/{feed_id}
	feedID := extractFeedIDFromPath(parsedURL.Path)
	if feedID == "" {
		return nil, fmt.Errorf("无法从重定向 URL 提取 feed_id: %s", redirectURL)
	}

	// 从查询参数中提取 xsec_token
	query := parsedURL.Query()
	xsecToken := query.Get("xsec_token")
	if xsecToken == "" {
		return nil, fmt.Errorf("无法从重定向 URL 提取 xsec_token: %s", redirectURL)
	}

	result := &ShortLinkResult{
		FeedID:      feedID,
		XsecToken:   xsecToken,
		OriginalURL: originalURL,
		RedirectURL: redirectURL,
		ShareID:     query.Get("share_id"),
		AppPlatform: query.Get("app_platform"),
		XsecSource:  query.Get("xsec_source"),
	}

	logrus.Infof("解析成功: feed_id=%s, xsec_token=%s", feedID, xsecToken)

	return result, nil
}

// extractFeedIDFromPath 从 URL 路径中提取 feed_id
func extractFeedIDFromPath(path string) string {
	// 支持多种路径格式
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/discovery/item/([a-zA-Z0-9]+)`),
		regexp.MustCompile(`/explore/([a-zA-Z0-9]+)`),
		regexp.MustCompile(`/note/([a-zA-Z0-9]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(path)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}
