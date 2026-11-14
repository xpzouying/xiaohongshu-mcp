package xiaohongshu

// 视频字幕的下载与解析。
//
// 视频画面本身对调用方（大模型）没有价值，真正可读的是字幕——它等于视频内容的
// 文字转录。字幕的 URL 躺在 __INITIAL_STATE__ 里，但正文要另外下载 .srt，
// 且链接带签名有时效（url 里的 sign/t 参数），客户端拿到 URL 也取不动，
// 所以在服务端取好、去掉时间轴，直接把台词交出去。

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

const (
	// subtitleFetchTimeout 单个字幕文件的下载超时。
	// 实测一条 166 秒视频的 srt 约 4.7KB / 0.2s，给足余量又不拖累详情接口。
	subtitleFetchTimeout = 8 * time.Second

	// subtitleMaxBytes 字幕正文读取上限，防异常大文件撑爆内存（服务器只有 1.6G）。
	subtitleMaxBytes = 2 << 20 // 2MB
)

// subtitleLangPriority 挑字幕的优先级。
// source 是视频的原始语种，最贴近里面实际说的话；其次简中。
var subtitleLangPriority = []string{"source", "zh-CN", "zh"}

// pickSubtitle 从多语言字幕里挑一档，返回语言标识与下载地址。
// 都取不到时返回空串，调用方据此跳过。
func pickSubtitle(subs map[string][]VideoSubtitle) (lang, url string) {
	pick := func(key string) (string, string, bool) {
		items, ok := subs[key]
		if !ok || len(items) == 0 || items[0].URL == "" {
			return "", "", false
		}
		l := items[0].Language
		if l == "" {
			l = key
		}
		return l, items[0].URL, true
	}

	for _, key := range subtitleLangPriority {
		if l, u, ok := pick(key); ok {
			return l, u
		}
	}
	// 优先级都没命中就退回任意一档，聊胜于无。
	for key := range subs {
		if l, u, ok := pick(key); ok {
			return l, u
		}
	}
	return "", ""
}

// parseSRT 去掉 srt 的序号与时间轴，只留台词。
// 每条字幕单独成行——原文本就是按语句切的，保留换行比硬拼成一段更好读。
func parseSRT(r io.Reader) string {
	var b strings.Builder

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.Contains(line, "-->") {
			continue
		}
		// 纯数字行是字幕序号，丢掉
		if _, err := strconv.Atoi(line); err == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	if err := sc.Err(); err != nil {
		logrus.Warnf("解析字幕中断: %v", err)
	}

	return b.String()
}

// fillSubtitleText 下载字幕正文并填进 v。
// 字幕是附加信息，任何一步失败都只告警不报错——不该因为它拖垮整个详情接口。
func fillSubtitleText(ctx context.Context, v *VideoDetail) {
	if v == nil || len(v.Subtitles) == 0 {
		return
	}

	lang, url := pickSubtitle(v.Subtitles)
	if url == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, subtitleFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logrus.Warnf("构造字幕请求失败(%s): %v", lang, err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Warnf("下载字幕失败(%s): %v", lang, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		logrus.Warnf("下载字幕失败(%s): HTTP %d", lang, resp.StatusCode)
		return
	}

	text := parseSRT(io.LimitReader(resp.Body, subtitleMaxBytes))
	if text == "" {
		logrus.Warnf("字幕内容为空(%s)", lang)
		return
	}

	v.SubtitleText = text
	v.SubtitleLang = lang
	logrus.Infof("字幕已取回: %s，%d 字", lang, utf8.RuneCountInString(text))
}
