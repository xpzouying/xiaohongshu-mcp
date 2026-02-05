package xiaohongshu

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type longArticleBlockType int

const (
	blockParagraph longArticleBlockType = iota
	blockH1
	blockH2
	blockOrderedList
	blockUnorderedList
	blockQuote
	blockImage
)

type longArticleInline struct {
	Text      string
	Highlight bool
	Emoji     string // Unicode emoji
}

type longArticleBlock struct {
	Type    longArticleBlockType
	Inlines []longArticleInline
	Items   [][]longArticleInline // list items
	Image   string                // local path
}

func getLongArticleBodyElement(page *rod.Page) (*rod.Element, bool) {
	// 新版编辑器正文：tiptap ProseMirror
	if el, err := page.Timeout(2 * time.Second).Element("div.tiptap.ProseMirror"); err == nil && el != nil {
		return el, true
	}

	// 先复用旧逻辑（可能编辑器仍使用 ql-editor）
	if el, ok := getContentElement(page); ok {
		return el, true
	}
	if el, ok := getLongArticleContentElement(page); ok {
		return el, true
	}

	// 长文编辑器常见占位符
	el, err := findEditableByPlaceholder(page, []string{
		"粘贴到这里或输入文字",
		"粘贴到这里或输入文字…",
		"输入正文",
		"请输入正文",
	})
	if err == nil && el != nil {
		return el, true
	}

	// 有些实现占位符是普通文本节点
	for _, t := range []string{"粘贴到这里或输入文字", "粘贴到这里或输入文字…"} {
		if e, err := findEditableByText(page, t); err == nil && e != nil {
			return e, true
		}
	}

	return nil, false
}

func getLongArticleTitleElement(page *rod.Page) (*rod.Element, bool) {
	// 新版编辑器标题：textarea placeholder="输入标题"
	if el, err := page.Timeout(2 * time.Second).Element("textarea.d-text[placeholder='输入标题']"); err == nil && el != nil {
		return el, true
	}

	// 旧版发布页的标题 input
	if has, el, _ := page.Has("div.d-input input"); has && el != nil {
		return el, true
	}

	// 新版长文编辑器：标题通常是 contenteditable，placeholder 为“输入标题”
	if el, err := findEditableByPlaceholder(page, []string{"输入标题"}); err == nil && el != nil {
		return el, true
	}

	// 占位符是普通文本节点
	if el, err := findEditableByText(page, "输入标题"); err == nil && el != nil {
		return el, true
	}

	// 兜底：找包含“输入标题”的可编辑区域
	el, err := page.Timeout(2*time.Second).ElementR("[contenteditable='true'],[role='textbox']", "输入标题")
	if err == nil && el != nil {
		return el, true
	}

	return nil, false
}

func findEditableByPlaceholder(page *rod.Page, placeholders []string) (*rod.Element, error) {
	// 通过 data-placeholder 搜索（不限定 tag）
	elements, err := page.Elements("[data-placeholder]")
	if err != nil || len(elements) == 0 {
		return nil, errors.New("no elements with data-placeholder found")
	}

	for _, el := range elements {
		if el == nil {
			continue
		}
		ph, err := el.Attribute("data-placeholder")
		if err != nil || ph == nil {
			continue
		}
		for _, p := range placeholders {
			if strings.Contains(*ph, p) {
				// 往上找 contenteditable 或 role=textbox
				if editable := findEditableParent(el); editable != nil {
					return editable, nil
				}
				return el, nil
			}
		}
	}

	return nil, errors.New("no placeholder element found")
}

func findEditableParent(el *rod.Element) *rod.Element {
	// 元素本身已可编辑
	if role, _ := el.Attribute("role"); role != nil && *role == "textbox" {
		return el
	}
	if ce, _ := el.Attribute("contenteditable"); ce != nil && (*ce == "true" || *ce == "plaintext-only") {
		return el
	}

	cur := el
	for i := 0; i < 8; i++ {
		parent, err := cur.Parent()
		if err != nil || parent == nil {
			return nil
		}

		if role, _ := parent.Attribute("role"); role != nil && *role == "textbox" {
			return parent
		}
		if ce, _ := parent.Attribute("contenteditable"); ce != nil && (*ce == "true" || *ce == "plaintext-only") {
			return parent
		}
		cur = parent
	}
	return nil
}

func findEditableByText(page *rod.Page, text string) (*rod.Element, error) {
	// 注意：ElementR 的正则是匹配元素文本
	el, err := page.Timeout(2*time.Second).ElementR("div,span,p", regexp.QuoteMeta(text))
	if err != nil || el == nil {
		return nil, errors.New("text element not found: " + text)
	}
	editable := findEditableParent(el)
	if editable != nil {
		return editable, nil
	}
	return nil, errors.New("editable parent not found for: " + text)
}

func renderLongArticleMarkdown(page *rod.Page, body *rod.Element, md string, baseDir string) error {
	blocks, err := parseLongArticleMarkdown(md)
	if err != nil {
		return err
	}
	if len(blocks) == 0 {
		return errors.New("markdown 内容为空")
	}

	toolbar, err := getLongArticleToolbarRoot(page)
	if err != nil {
		return err
	}

	logrus.Infof("长文 Markdown 渲染：blocks=%d", len(blocks))

	// 聚焦正文区域
	focusLongArticleBody(body)

	for _, b := range blocks {
		switch b.Type {
		case blockH1:
			waitEditorIdle(page, body)
			focusLongArticleBody(body)
			if err := typeInlines(page, toolbar, body, b.Inlines); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			if err := clickHeadingButton(toolbar, 1); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			moveCursorToLineEnd(page)
			waitEditorIdle(page, body)
			pressEnter(page, 1)
		case blockH2:
			waitEditorIdle(page, body)
			focusLongArticleBody(body)
			if err := typeInlines(page, toolbar, body, b.Inlines); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			if err := clickHeadingButton(toolbar, 2); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			moveCursorToLineEnd(page)
			waitEditorIdle(page, body)
			pressEnter(page, 1)
		case blockQuote:
			waitEditorIdle(page, body)
			if err := clickToolbarByKind(toolbar, toolbarQuote); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			if err := typeInlines(page, toolbar, body, b.Inlines); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			pressEnter(page, 1)
			waitEditorIdle(page, body)
			_ = clickToolbarByKind(toolbar, toolbarQuote) // best-effort 关闭引用
		case blockOrderedList:
			waitEditorIdle(page, body)
			if err := clickToolbarByKind(toolbar, toolbarOrderedList); err != nil {
				return err
			}
			for _, item := range b.Items {
				waitEditorIdle(page, body)
				if err := typeInlines(page, toolbar, body, item); err != nil {
					return err
				}
				waitEditorIdle(page, body)
				pressEnter(page, 1)
			}
			waitEditorIdle(page, body)
			pressEnter(page, 2) // 退出列表
			waitEditorIdle(page, body)
			_ = clickToolbarByKind(toolbar, toolbarOrderedList)
		case blockUnorderedList:
			waitEditorIdle(page, body)
			if err := clickToolbarByKind(toolbar, toolbarUnorderedList); err != nil {
				return err
			}
			for _, item := range b.Items {
				waitEditorIdle(page, body)
				if err := typeInlines(page, toolbar, body, item); err != nil {
					return err
				}
				waitEditorIdle(page, body)
				pressEnter(page, 1)
			}
			waitEditorIdle(page, body)
			pressEnter(page, 2)
			waitEditorIdle(page, body)
			_ = clickToolbarByKind(toolbar, toolbarUnorderedList)
		case blockImage:
			imgPath := b.Image
			if imgPath == "" {
				continue
			}
			if baseDir != "" && !filepath.IsAbs(imgPath) {
				imgPath = filepath.Join(baseDir, imgPath)
			}
			waitEditorIdle(page, body)
			if err := insertImage(page, toolbar, imgPath); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			pressEnter(page, 1)
		default:
			waitEditorIdle(page, body)
			if err := typeInlines(page, toolbar, body, b.Inlines); err != nil {
				return err
			}
			waitEditorIdle(page, body)
			pressEnter(page, 1)
		}
	}

	return nil
}

func pressEnter(page *rod.Page, n int) {
	for i := 0; i < n; i++ {
		page.Keyboard.MustType(input.Enter)
		time.Sleep(80 * time.Millisecond)
	}
}

type toolbarKind int

const (
	toolbarOrderedList toolbarKind = iota
	toolbarUnorderedList
	toolbarQuote
	toolbarHighlight
	toolbarImage
	toolbarEmoji
)

func typeInlines(page *rod.Page, toolbar *rod.Element, body *rod.Element, inlines []longArticleInline) error {
	for _, inl := range inlines {
		if inl.Emoji != "" {
			waitEditorIdle(page, body)
			if err := insertEmoji(page, toolbar, inl.Emoji); err != nil {
				return err
			}
			continue
		}

		if inl.Highlight {
			waitEditorIdle(page, body)
			_ = clickToolbarByKind(toolbar, toolbarHighlight)
		}

		if inl.Text != "" {
			waitEditorIdle(page, body)
			focusLongArticleBody(body)
			page.MustInsertText(inl.Text)
		}

		if inl.Highlight {
			waitEditorIdle(page, body)
			_ = clickToolbarByKind(toolbar, toolbarHighlight)
		}
	}
	return nil
}

func focusLongArticleBody(body *rod.Element) {
	_ = body.ScrollIntoView()
	_ = body.Click(proto.InputMouseButtonLeft, 1)
	time.Sleep(120 * time.Millisecond)
}

func moveCursorToLineEnd(page *rod.Page) {
	// 新版编辑器点击标题按钮后光标会回到行首，需移到行尾再换行
	page.Keyboard.MustType(input.End)
	time.Sleep(60 * time.Millisecond)
}

type editorSelectionState struct {
	InEditor bool `json:"inEditor"`
	Anchor   int  `json:"anchor"`
	Focus    int  `json:"focus"`
	TextLen  int  `json:"textLen"`
}

func waitEditorIdle(page *rod.Page, body *rod.Element) {
	_ = body
	deadline := time.Now().Add(1200 * time.Millisecond)
	var last editorSelectionState
	stableCount := 0

	for time.Now().Before(deadline) {
		state := editorSelectionState{InEditor: false, Anchor: -1, Focus: -1, TextLen: -1}
		res, err := page.Timeout(800 * time.Millisecond).Eval(`() => {
			const sel = window.getSelection && window.getSelection();
			const active = document.activeElement;
			const inEditor = !!(active && active.closest && active.closest('.rich-editor-content'));
			let anchor = -1, focus = -1, textLen = -1;
			if (sel && sel.anchorNode) {
				anchor = sel.anchorOffset;
				focus = sel.focusOffset;
				const node = sel.anchorNode;
				if (node && node.nodeType === 3) textLen = node.textContent ? node.textContent.length : 0;
			}
			return { inEditor, anchor, focus, textLen };
		}`)
		if err == nil {
			err = res.Value.Unmarshal(&state)
		}
		if err != nil {
			time.Sleep(120 * time.Millisecond)
			continue
		}

		if state.InEditor &&
			state.Anchor == last.Anchor &&
			state.Focus == last.Focus &&
			state.TextLen == last.TextLen &&
			state.InEditor == last.InEditor {
			stableCount++
		} else {
			stableCount = 0
		}
		last = state

		if stableCount >= 2 {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
}

func parseLongArticleMarkdown(md string) ([]longArticleBlock, error) {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	lines := strings.Split(md, "\n")

	var blocks []longArticleBlock

	var pendingPara []longArticleInline
	flushPara := func() {
		if len(pendingPara) == 0 {
			return
		}
		blocks = append(blocks, longArticleBlock{Type: blockParagraph, Inlines: pendingPara})
		pendingPara = nil
	}

	i := 0
	for i < len(lines) {
		line := strings.TrimRight(lines[i], " \t")
		if strings.TrimSpace(line) == "" {
			flushPara()
			i++
			continue
		}

		// Image (single line)
		if m := imageLineRe().FindStringSubmatch(strings.TrimSpace(line)); len(m) == 2 {
			flushPara()
			blocks = append(blocks, longArticleBlock{Type: blockImage, Image: m[1]})
			i++
			continue
		}

		// Headings
		if strings.HasPrefix(line, "# ") {
			flushPara()
			blocks = append(blocks, longArticleBlock{Type: blockH1, Inlines: parseInlines(strings.TrimSpace(strings.TrimPrefix(line, "# ")))})
			i++
			continue
		}
		if strings.HasPrefix(line, "## ") {
			flushPara()
			blocks = append(blocks, longArticleBlock{Type: blockH2, Inlines: parseInlines(strings.TrimSpace(strings.TrimPrefix(line, "## ")))})
			i++
			continue
		}

		// Blockquote (single line)
		if strings.HasPrefix(strings.TrimSpace(line), "> ") {
			flushPara()
			blocks = append(blocks, longArticleBlock{Type: blockQuote, Inlines: parseInlines(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "> ")))})
			i++
			continue
		}

		// Lists
		if isOrderedListLine(line) {
			flushPara()
			var items [][]longArticleInline
			for i < len(lines) && isOrderedListLine(strings.TrimRight(lines[i], " \t")) {
				item := orderedListItemText(strings.TrimRight(lines[i], " \t"))
				items = append(items, parseInlines(item))
				i++
			}
			blocks = append(blocks, longArticleBlock{Type: blockOrderedList, Items: items})
			continue
		}
		if isUnorderedListLine(line) {
			flushPara()
			var items [][]longArticleInline
			for i < len(lines) && isUnorderedListLine(strings.TrimRight(lines[i], " \t")) {
				item := strings.TrimSpace(lines[i])[2:]
				items = append(items, parseInlines(item))
				i++
			}
			blocks = append(blocks, longArticleBlock{Type: blockUnorderedList, Items: items})
			continue
		}

		// Paragraph line (join with space)
		if len(pendingPara) > 0 {
			pendingPara = append(pendingPara, longArticleInline{Text: "\n"})
		}
		pendingPara = append(pendingPara, parseInlines(line)...)
		i++
	}
	flushPara()

	return blocks, nil
}

func parseInlines(text string) []longArticleInline {
	if text == "" {
		return nil
	}

	// highlight: ==text==
	var out []longArticleInline
	for len(text) > 0 {
		start := strings.Index(text, "==")
		if start == -1 {
			out = append(out, parseEmojiInline(text)...)
			break
		}
		end := strings.Index(text[start+2:], "==")
		if end == -1 {
			out = append(out, parseEmojiInline(text)...)
			break
		}
		end = start + 2 + end

		if start > 0 {
			out = append(out, parseEmojiInline(text[:start])...)
		}
		hl := text[start+2 : end]
		if strings.TrimSpace(hl) != "" {
			out = append(out, parseEmojiInlineWithHighlight(hl, true)...)
		}
		text = text[end+2:]
	}
	return out
}

func parseEmojiInline(text string) []longArticleInline {
	return parseEmojiInlineWithHighlight(text, false)
}

func parseEmojiInlineWithHighlight(text string, highlight bool) []longArticleInline {
	if text == "" {
		return nil
	}

	// :smile: style
	re := regexp.MustCompile(`:([a-zA-Z0-9_+-]+):`)
	locs := re.FindAllStringSubmatchIndex(text, -1)
	if len(locs) == 0 {
		return []longArticleInline{{Text: text, Highlight: highlight}}
	}

	var out []longArticleInline
	cursor := 0
	for _, loc := range locs {
		if loc[0] > cursor {
			out = append(out, longArticleInline{Text: text[cursor:loc[0]], Highlight: highlight})
		}
		code := text[loc[2]:loc[3]]
		if em, ok := emojiMap()[strings.ToLower(code)]; ok {
			out = append(out, longArticleInline{Emoji: em, Highlight: highlight})
		} else {
			out = append(out, longArticleInline{Text: text[loc[0]:loc[1]], Highlight: highlight})
		}
		cursor = loc[1]
	}
	if cursor < len(text) {
		out = append(out, longArticleInline{Text: text[cursor:], Highlight: highlight})
	}
	return out
}

func emojiMap() map[string]string {
	return map[string]string{
		"smile":      "😄",
		"grin":       "😁",
		"joy":        "😂",
		"wink":       "😉",
		"heart":      "❤️",
		"thumbsup":   "👍",
		"fire":       "🔥",
		"star":       "⭐",
		"clap":       "👏",
		"thinking":   "🤔",
		"cry":        "😢",
		"sunglasses": "😎",
	}
}

var reOrderedList = regexp.MustCompile(`^\s*\d+\.\s+`)

func isOrderedListLine(line string) bool {
	return reOrderedList.MatchString(line)
}

func orderedListItemText(line string) string {
	return reOrderedList.ReplaceAllString(line, "")
}

func isUnorderedListLine(line string) bool {
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ ")
}

var cachedImageLineRe *regexp.Regexp

func imageLineRe() *regexp.Regexp {
	if cachedImageLineRe != nil {
		return cachedImageLineRe
	}
	cachedImageLineRe = regexp.MustCompile(`^!\[[^\]]*]\(([^)]+)\)\s*$`)
	return cachedImageLineRe
}

func getLongArticleToolbarRoot(page *rod.Page) (*rod.Element, error) {
	// 新版编辑器：工具栏容器
	root, err := page.Timeout(15 * time.Second).Element("div.rich-editor-container .header .menu-items-container")
	if err == nil && root != nil {
		return root, nil
	}

	return nil, errors.New("未能定位长文工具栏容器")
}

func clickToolbarButton(toolbar *rod.Element, text string, fallbackOffsetFromH2 int) error {
	btns, err := toolbar.Elements("button.menu-item,button,[role=button]")
	if err != nil {
		return err
	}

	// text match
	if strings.TrimSpace(text) != "" {
		for _, b := range btns {
			if b == nil {
				continue
			}
			t, _ := b.Text()
			if strings.TrimSpace(t) == text {
				return b.Click(proto.InputMouseButtonLeft, 1)
			}
		}
	}

	if fallbackOffsetFromH2 > 0 {
		// 新版编辑器按钮顺序（仅按钮）：0撤销 1重做 2H1 3H2 4有序 5无序 6引用 7高亮 8图片 9表情
		indexMap := map[int]int{
			1: 4, // ordered
			2: 5, // unordered
			3: 6, // quote
			4: 7, // highlight
			5: 8, // image
			6: 9, // emoji
		}
		if idx, ok := indexMap[fallbackOffsetFromH2]; ok && idx < len(btns) {
			return btns[idx].Click(proto.InputMouseButtonLeft, 1)
		}
	}

	return fmt.Errorf("未找到工具栏按钮: %s", text)
}

func clickHeadingButton(toolbar *rod.Element, level int) error {
	btns, err := toolbar.Elements("button.menu-item,button,[role=button]")
	if err != nil {
		return err
	}

	indexMap := map[int]int{
		1: 2, // H1
		2: 3, // H2
	}
	idx, ok := indexMap[level]
	if !ok || idx >= len(btns) {
		return fmt.Errorf("未找到标题按钮: H%d", level)
	}

	return btns[idx].Click(proto.InputMouseButtonLeft, 1)
}

func clickToolbarByKind(toolbar *rod.Element, kind toolbarKind) error {
	// 先按关键词匹配（title/aria-label/文本）
	keywords := map[toolbarKind][]string{
		toolbarOrderedList:   {"有序", "编号", "序号"},
		toolbarUnorderedList: {"无序", "项目符号"},
		toolbarQuote:         {"引用"},
		toolbarHighlight:     {"高亮", "标记", "荧光"},
		toolbarImage:         {"图片", "插入图片"},
		toolbarEmoji:         {"表情", "emoji", "emoj"},
	}
	if btn, err := findToolbarButton(toolbar, keywords[kind]); err == nil && btn != nil {
		return btn.Click(proto.InputMouseButtonLeft, 1)
	}

	// fallback：按 H2 偏移（基于你截图的顺序：H2 后依次为 有序/无序/引用/高亮/图片/表情）
	offset := map[toolbarKind]int{
		toolbarOrderedList:   1,
		toolbarUnorderedList: 2,
		toolbarQuote:         3,
		toolbarHighlight:     4,
		toolbarImage:         5,
		toolbarEmoji:         6,
	}[kind]
	return clickToolbarButton(toolbar, "", offset)
}

func findToolbarButton(toolbar *rod.Element, keywords []string) (*rod.Element, error) {
	btns, err := toolbar.Elements("button.menu-item,button,[role=button]")
	if err != nil {
		return nil, err
	}

	for _, b := range btns {
		if b == nil {
			continue
		}
		vis, err := b.Visible()
		if err == nil && !vis {
			continue
		}

		text, _ := b.Text()
		title, _ := b.Attribute("title")
		aria, _ := b.Attribute("aria-label")

		hay := strings.ToLower(strings.TrimSpace(text))
		if title != nil {
			hay += " " + strings.ToLower(*title)
		}
		if aria != nil {
			hay += " " + strings.ToLower(*aria)
		}

		for _, kw := range keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(hay, strings.ToLower(kw)) {
				return b, nil
			}
		}
	}

	return nil, &rod.ElementNotFoundError{}
}

func insertImage(page *rod.Page, toolbar *rod.Element, imagePath string) error {
	logrus.Infof("插入图片: %s", imagePath)

	// 优先：使用系统文件选择器事件（稳定）
	if handle, err := page.Timeout(8 * time.Second).HandleFileDialog(); err == nil {
		if err := clickToolbarByKind(toolbar, toolbarImage); err != nil {
			return err
		}
		if err := handle([]string{imagePath}); err == nil {
			closeSystemFileDialog(page)
			time.Sleep(2 * time.Second)
			return nil
		}
	}

	// fallback：尝试直接找到 input[type=file]
	if inputEl, err := findFileInputAcrossFrames(page, 800*time.Millisecond); err == nil && inputEl != nil {
		inputEl.MustSetFiles(imagePath)
		time.Sleep(2 * time.Second)
		return nil
	}

	closeSystemFileDialog(page)
	return errors.New("未找到图片上传 input")
}

func waitForFileInput(page *rod.Page, timeout time.Duration) (*rod.Element, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		el, err := page.Timeout(800 * time.Millisecond).Element("input[type='file']")
		if err == nil && el != nil {
			return el, nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return nil, errors.New("input[type=file] not found")
}

func findFileInputAcrossFrames(page *rod.Page, timeout time.Duration) (*rod.Element, error) {
	if el, err := page.Timeout(timeout).Element("input[type='file']"); err == nil && el != nil {
		return el, nil
	}
	iframes, err := page.Elements("iframe")
	if err != nil {
		return nil, err
	}
	for _, f := range iframes {
		fp, err := f.Frame()
		if err != nil || fp == nil {
			continue
		}
		if el, err := fp.Timeout(timeout).Element("input[type='file']"); err == nil && el != nil {
			return el, nil
		}
	}
	return nil, errors.New("input[type=file] not found")
}

func closeSystemFileDialog(page *rod.Page) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		page.Keyboard.MustType(input.Escape)
	}
}

func insertEmoji(page *rod.Page, toolbar *rod.Element, emoji string) error {
	if err := clickToolbarByKind(toolbar, toolbarEmoji); err != nil {
		return err
	}

	// 在弹层里找 emoji 字符
	selector := "button,span,div"
	el, err := page.Timeout(5*time.Second).ElementR(selector, regexp.QuoteMeta(emoji))
	if err != nil {
		// 关闭弹层避免影响后续输入
		page.Keyboard.MustType(input.Escape)
		return errors.Wrapf(err, "未找到表情: %s", emoji)
	}

	_ = el.Click(proto.InputMouseButtonLeft, 1)
	time.Sleep(200 * time.Millisecond)
	page.Keyboard.MustType(input.Escape)
	return nil
}
