package xiaohongshu

import (
	"context"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PublishLongArticleContent 发布长文内容
type PublishLongArticleContent struct {
	Title        string
	Content      string
	ContentIsMD  bool
	MarkdownBase string
	PostTitle    string
	PostContent  string
	Tags         []string
	ScheduleTime *time.Time // 定时发布时间，nil 表示立即发布
}

// NewPublishLongArticleAction 进入发布页并切换到"写长文"
func NewPublishLongArticleAction(page *rod.Page) (*PublishAction, error) {
	pp := page.Timeout(300 * time.Second)

	if err := pp.Navigate(urlOfPublic); err != nil {
		return nil, errors.Wrap(err, "导航到发布页面失败")
	}

	// 使用 WaitLoad 代替 WaitIdle（更宽松）
	if err := pp.WaitLoad(); err != nil {
		if !strings.Contains(err.Error(), "Execution context was destroyed") {
			logrus.Warnf("等待页面加载出现问题: %v，继续尝试", err)
		}
	}
	time.Sleep(2 * time.Second)

	if err := pp.WaitDOMStable(time.Second, 0.1); err != nil {
		logrus.Warnf("等待 DOM 稳定出现问题: %v，继续尝试", err)
	}
	time.Sleep(1 * time.Second)

	if err := mustClickPublishTab(pp, "写长文"); err != nil {
		// 兼容不同文案
		if err2 := mustClickPublishTab(pp, "长文"); err2 != nil {
			return nil, errors.Wrap(err, "切换到写长文失败")
		}
	}

	time.Sleep(1 * time.Second)

	editorPage, err := enterLongArticleEditorPage(pp)
	if err != nil {
		return nil, err
	}

	// 若打开了新的创作页，关闭旧页避免泄漏
	if editorPage != pp {
		_ = pp.Close()
	}

	return &PublishAction{page: editorPage}, nil
}

// PublishLongArticle 填写长文并提交发布
func (p *PublishAction) PublishLongArticle(ctx context.Context, content PublishLongArticleContent) error {
	if content.Title == "" {
		return errors.New("标题不能为空")
	}
	if content.Content == "" {
		return errors.New("正文不能为空")
	}

	page := p.page.Context(ctx)

	// 标题
	titleElem, ok := getLongArticleTitleElement(page)
	if !ok || titleElem == nil {
		return errors.New("没有找到标题输入框")
	}
	if err := setLongArticleTitle(page, titleElem, content.Title); err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond) // 等待页面渲染长度提示
	if err := checkTitleMaxLength(page); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// 正文 + 标签
	bodyElem, ok := getLongArticleBodyElement(page)
	if ok {
		if content.ContentIsMD {
			if err := renderLongArticleMarkdown(page, bodyElem, content.Content, content.MarkdownBase); err != nil {
				return err
			}
		} else {
			bodyElem.MustInput(content.Content)
		}

		// 标签：目前沿用旧逻辑 best-effort（长文编辑器可能不支持同样的标签输入）
		inputTags(bodyElem, content.Tags)
		_ = clickOneKeyFormat(page)
		if ok, _ := clickTemplateNextIfPresent(page); ok {
			// 进入图文发布页后，走 post_content 的逻辑
			if err := waitForPostContentEditor(page); err != nil {
				return err
			}
			postTitle := content.Title
			if strings.TrimSpace(content.PostTitle) != "" {
				postTitle = content.PostTitle
			}
			return submitPublish(page, postTitle, content.PostContent, content.Tags, content.ScheduleTime)
		}
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	// 长文可能没有同样的长度提示元素，这里只做 best-effort 的检查
	if err := checkContentMaxLength(page); err != nil {
		return err
	}

	// 处理定时发布
	if content.ScheduleTime != nil {
		if err := setSchedulePublish(page, *content.ScheduleTime); err != nil {
			return errors.Wrap(err, "设置定时发布失败")
		}
	}

	btn, err := waitForPublishButtonClickable(page)
	if err == nil && btn != nil {
		if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return errors.Wrap(err, "点击发布按钮失败")
		}
		time.Sleep(3 * time.Second)
		return nil
	}

	// fallback：直接点击发布按钮
	submitButton := page.MustElement(".publish-page-publish-btn button.bg-red")
	submitButton.MustClick()
	time.Sleep(3 * time.Second)
	return nil
}

func waitForPostContentEditor(page *rod.Page) error {
	// 生成图文卡片需要时间，放宽等待
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		// 标题输入
		if _, err := page.Timeout(1200 * time.Millisecond).Element("div.d-input input"); err == nil {
			return nil
		}
		// 发布按钮
		if _, err := page.Timeout(1200 * time.Millisecond).Element(".publish-page-publish-btn button.bg-red"); err == nil {
			return nil
		}
		// 图片编辑区（图文页常见）
		if _, err := page.Timeout(1200 * time.Millisecond).Element(".img-preview-area"); err == nil {
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return errors.New("进入图文发布页超时（等待生成图文卡片）")
}

func clickOneKeyFormat(page *rod.Page) error {
	if btn, err := waitForOneKeyFormatButton(page, 10*time.Second); err == nil && btn != nil {
		_ = btn.ScrollIntoView()
		_ = btn.Click(proto.InputMouseButtonLeft, 1)
		time.Sleep(800 * time.Millisecond)
		return nil
	}
	if span, err := page.Timeout(2 * time.Second).Element(".next-btn-text"); err == nil && span != nil {
		_ = span.ScrollIntoView()
		_ = span.Click(proto.InputMouseButtonLeft, 1)
		time.Sleep(800 * time.Millisecond)
		return nil
	}
	res, err := page.Timeout(2 * time.Second).Eval(`() => {
		const nodes = Array.from(document.querySelectorAll('button,span,div'));
		const hit = nodes.find(n => n && n.textContent && n.textContent.includes('一键排版'));
		if (!hit) return false;
		let el = hit;
		for (let i = 0; i < 4; i++) {
			if (el.tagName && el.tagName.toLowerCase() === 'button') break;
			el = el.parentElement;
			if (!el) break;
		}
		if (el && el.click) {
			el.click();
			return true;
		}
		return false;
	}`)
	if err != nil {
		return err
	}
	var ok bool
	_ = res.Value.Unmarshal(&ok)
	if ok {
		time.Sleep(800 * time.Millisecond)
	}
	return nil
}

func waitForOneKeyFormatButton(page *rod.Page, timeout time.Duration) (*rod.Element, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		btn, err := page.Timeout(800 * time.Millisecond).Element("button.next-btn")
		if err == nil && btn != nil {
			disabled, _ := btn.Attribute("disabled")
			if disabled == nil {
				return btn, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, errors.New("一键排版按钮不可用")
}

func clickTemplateNextIfPresent(page *rod.Page) (bool, error) {
	// 模板页“下一步”按钮
	if btn, err := waitForTemplateNextButton(page, 10*time.Second); err == nil && btn != nil {
		_ = btn.Click(proto.InputMouseButtonLeft, 1)
		time.Sleep(800 * time.Millisecond)
		return true, nil
	}
	// JS 兜底点击（避免被遮挡）
	if ok, _ := clickTemplateNextByJS(page); ok {
		time.Sleep(800 * time.Millisecond)
		return true, nil
	}
	// 兜底：文本匹配“下一步”
	res, err := page.Timeout(2 * time.Second).Eval(`() => {
		const nodes = Array.from(document.querySelectorAll('button,span,div'));
		const hit = nodes.find(n => n && n.textContent && n.textContent.includes('下一步'));
		if (!hit) return false;
		let el = hit;
		for (let i = 0; i < 4; i++) {
			if (el.tagName && el.tagName.toLowerCase() === 'button') break;
			el = el.parentElement;
			if (!el) break;
		}
		if (el && el.click) {
			el.click();
			return true;
		}
		return false;
	}`)
	if err != nil {
		return false, err
	}
	var ok bool
	_ = res.Value.Unmarshal(&ok)
	if ok {
		time.Sleep(800 * time.Millisecond)
	}
	return ok, nil
}

func waitForTemplateNextButton(page *rod.Page, timeout time.Duration) (*rod.Element, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		btn, err := page.Timeout(800 * time.Millisecond).Element("button.submit")
		if err == nil && btn != nil {
			disabled, _ := btn.Attribute("disabled")
			if disabled == nil {
				return btn, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, errors.New("未找到模板页下一步按钮")
}

func clickTemplateNextByJS(page *rod.Page) (bool, error) {
	res, err := page.Timeout(2 * time.Second).Eval(`() => {
		const btn = document.querySelector('button.submit');
		if (btn && !btn.disabled) { btn.click(); return true; }
		const nodes = Array.from(document.querySelectorAll('button,span,div'));
		const hit = nodes.find(n => n && n.textContent && n.textContent.includes('下一步'));
		if (!hit) return false;
		let el = hit;
		for (let i = 0; i < 4; i++) {
			if (el.tagName && el.tagName.toLowerCase() === 'button') break;
			el = el.parentElement;
			if (!el) break;
		}
		if (el && !el.disabled && el.click) { el.click(); return true; }
		return false;
	}`)
	if err != nil {
		return false, err
	}
	var ok bool
	_ = res.Value.Unmarshal(&ok)
	return ok, nil
}

func setLongArticleTitle(page *rod.Page, titleElem *rod.Element, title string) error {
	_ = titleElem.ScrollIntoView()
	_ = titleElem.Click(proto.InputMouseButtonLeft, 1)
	time.Sleep(150 * time.Millisecond)

	// 尝试清空（不同实现可能是 input 或 contenteditable）
	_, _ = titleElem.Eval(`() => {
		if (this.tagName === 'INPUT' || this.tagName === 'TEXTAREA') {
			this.value = '';
		} else if (this.isContentEditable) {
			this.innerText = '';
		}
		return true;
	}`)

	// 优先使用 Element.Input（对 input/textarea/contenteditable 都可能生效）
	if err := titleElem.Input(title); err == nil {
		return nil
	}

	page.MustInsertText(title)
	return nil
}

func enterLongArticleEditorPage(page *rod.Page) (*rod.Page, error) {
	// 可能已经在创作页
	if isLongArticleEditorReady(page) {
		return page, nil
	}

	beforePages, _ := page.Browser().Pages()
	beforeSet := make(map[*rod.Page]struct{}, len(beforePages))
	for _, p := range beforePages {
		beforeSet[p] = struct{}{}
	}

	// 优先点“新的创作”（你截图里的主入口）
	_ = clickNewCreationButton(page)
	return waitForEditorPage(page, beforeSet)
}

func clickNewCreationButton(page *rod.Page) bool {
	// 优先用精确 class（来自 HTML）
	if btn, err := page.Timeout(3 * time.Second).Element("button.new-btn"); err == nil && btn != nil {
		// 先尝试 JS 直接点击（更快）
		_, _ = page.Eval(`() => {
			const btn = document.querySelector('button.new-btn');
			if (btn) { btn.click(); return true; }
			return false;
		}`)

		if err := btn.Click(proto.InputMouseButtonLeft, 1); err == nil {
			time.Sleep(100 * time.Millisecond)
			return true
		}
	}

	// 兜底：按文本匹配
	btn, err := page.Timeout(3*time.Second).ElementR("button,[role=button],a,div", `^\s*新的创作\s*$`)
	if err != nil || btn == nil {
		return false
	}
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return false
	}
	time.Sleep(100 * time.Millisecond)
	return true
}

func waitForEditorPage(page *rod.Page, beforeSet map[*rod.Page]struct{}) (*rod.Page, error) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if isLongArticleEditorReady(page) {
			return page, nil
		}

		if el, err := page.Timeout(500 * time.Millisecond).Element("div.rich-editor-container"); err == nil && el != nil {
			return page, nil
		}

		pages, _ := page.Browser().Pages()
		for _, p := range pages {
			if _, ok := beforeSet[p]; ok {
				continue
			}

			// 新页面打开后，等待编辑器容器出现
			pp := p.Timeout(300 * time.Second)
			if el, err := pp.Timeout(800 * time.Millisecond).Element("div.rich-editor-container"); err == nil && el != nil {
				return pp, nil
			}
		}

		time.Sleep(80 * time.Millisecond)
	}

	// 输出一些可观测信息，方便定位选择器问题
	debugLogLongArticleEditorSignals(page)
	return nil, errors.New("进入长文创作页超时：未检测到可编辑的输入区域")
}

func debugLogLongArticleEditorSignals(page *rod.Page) {
	url := ""
	if info, err := page.Info(); err == nil && info != nil {
		url = info.URL
	}
	logrus.WithFields(logrus.Fields{
		"url": url,
	}).Warn("长文进入编辑器超时，输出信号用于排查")

	if el, ok := getLongArticleTitleElement(page); ok && el != nil {
		logrus.Warn("信号：已找到标题输入区域")
	}
	if el, ok := getLongArticleBodyElement(page); ok && el != nil {
		logrus.Warn("信号：已找到正文输入区域")
	}

	// 列出可见按钮（前 25 个）
	btns, err := page.Timeout(2 * time.Second).Elements("button,[role=button]")
	if err != nil || len(btns) == 0 {
		logrus.WithError(err).Warn("信号：未枚举到按钮")
		return
	}

	count := 0
	for _, b := range btns {
		if b == nil {
			continue
		}
		vis, err := b.Visible()
		if err == nil && !vis {
			continue
		}
		txt, _ := b.Text()
		title, _ := b.Attribute("title")
		aria, _ := b.Attribute("aria-label")

		fields := logrus.Fields{
			"text": strings.TrimSpace(txt),
		}
		if title != nil {
			fields["title"] = *title
		}
		if aria != nil {
			fields["aria"] = *aria
		}

		logrus.WithFields(fields).Warn("button")
		count++
		if count >= 25 {
			break
		}
	}
}

func isLongArticleEditorReady(page *rod.Page) bool {
	// 新版长文编辑器：标题/正文占位符 + 底部按钮
	if el, _ := getLongArticleTitleElement(page); el != nil {
		return true
	}
	if el, _ := getLongArticleBodyElement(page); el != nil {
		return true
	}
	if el, err := page.Timeout(2*time.Second).ElementR("button,div,span", `一键排版`); err == nil && el != nil {
		return true
	}
	if el, err := page.Timeout(2*time.Second).ElementR("button,div,span", `暂存离开`); err == nil && el != nil {
		return true
	}
	// 工具栏容器存在也视为进入
	if el, err := page.Timeout(2 * time.Second).Element("div.rich-editor-container .header .menu-items-container"); err == nil && el != nil {
		return true
	}

	return false
}

func clickEnterCreateIfExists(page *rod.Page) bool {
	// 常见入口文案（按优先级）
	texts := []string{
		"开始创作",
		"新的创作",
		"新建创作",
		"进入创作",
		"立即创作",
		"去创作",
		"开始写作",
		"开始写长文",
		"去写长文",
	}

	// 常见可点击元素
	selectors := []string{
		"button",
		"a",
		"[role=button]",
		"div.creator-tab",
	}

	for _, sel := range selectors {
		elems, err := page.Elements(sel)
		if err != nil || len(elems) == 0 {
			continue
		}

		for _, el := range elems {
			if el == nil {
				continue
			}
			vis, err := el.Visible()
			if err == nil && !vis {
				continue
			}

			txt, err := el.Text()
			if err != nil {
				continue
			}

			for _, t := range texts {
				if txt == "" {
					continue
				}
				if strings.Contains(txt, t) {
					// 尽量用 Click，失败就跳过
					if err := el.Click(proto.InputMouseButtonLeft, 1); err == nil {
						time.Sleep(800 * time.Millisecond)
						return true
					}
				}
			}
		}
	}

	return false
}

func getLongArticleContentElement(page *rod.Page) (*rod.Element, bool) {
	elem, err := findTextboxByPlaceholderList(page, []string{
		"输入正文",
		"输入文章",
		"请输入正文",
		"请输入文章",
	})
	if err != nil || elem == nil {
		return nil, false
	}
	return elem, true
}

func findTextboxByPlaceholderList(page *rod.Page, placeholders []string) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	for _, ph := range placeholders {
		placeholderElem := findPlaceholderElement(elements, ph)
		if placeholderElem == nil {
			continue
		}

		textboxElem := findTextboxParent(placeholderElem)
		if textboxElem != nil {
			return textboxElem, nil
		}
	}

	return nil, errors.New("no textbox parent found")
}
