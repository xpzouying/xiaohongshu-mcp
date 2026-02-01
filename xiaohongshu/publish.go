package xiaohongshu

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	Tags       []string
	ImagePaths []string
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(600 * time.Second)

	pp.MustNavigate(urlOfPublic).MustWaitIdle().MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	if err := mustClickPublishTab(page, "上传图文"); err != nil {
		logrus.Errorf("点击上传图文 TAB 失败: %v", err)
		return nil, err
	}

	time.Sleep(1 * time.Second)

	return &PublishAction{
		page: pp,
	}, nil
}

func (p *PublishAction) Publish(ctx context.Context, content PublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	// 直接使用p.page,它已经有600秒的超时
	// 不要使用p.page.Context(ctx),因为ctx可能已经过期或被取消
	page := p.page

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	tags := content.Tags
	if len(tags) >= 10 {
		logrus.Warnf("标签数量超过10，截取前10个标签")
		tags = tags[:10]
	}

	logrus.Infof("发布内容: title=%s, images=%v, tags=%v", content.Title, len(content.ImagePaths), tags)

	// 传递带超时的page对象
	if err := submitPublish(page, content.Title, content.Content, tags); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func (p *PublishAction) SaveAndExit(ctx context.Context, content PublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	tags := content.Tags
	if len(tags) >= 10 {
		logrus.Warnf("标签数量超过10，截取前10个标签")
		tags = tags[:10]
	}

	logrus.Infof("暂存内容: title=%s, images=%v, tags=%v", content.Title, len(content.ImagePaths), tags)

	// 填写内容
	if err := fillPublishContent(page, content.Title, content.Content, tags); err != nil {
		return errors.Wrap(err, "填写发布内容失败")
	}

	// 点击暂存离开
	// 按钮class: custom-button cancelBtn
	cancelBtn, err := page.Element(".cancelBtn")
	if err != nil {
		// 尝试更精确的选择器
		cancelBtn, err = page.Element("button.cancelBtn")
		if err != nil {
			return errors.Wrap(err, "未找到暂存离开按钮")
		}
	}

	logrus.Info("点击暂存离开按钮")
	if err := cancelBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击暂存离开按钮失败")
	}

	// 验证是否成功：通常会跳转回首页或者创作中心可能
	// 或者检查 URL 变化
	time.Sleep(2 * time.Second)

	return nil
}

func removePopCover(page *rod.Page) {

	// 先移除弹窗封面
	has, elem, err := page.Has("div.d-popover")
	if err != nil {
		return
	}
	if has {
		elem.MustRemove()
	}

	// 兜底：点击一下空位置吧
	clickEmptyPosition(page)
}

func clickEmptyPosition(page *rod.Page) {
	x := 380 + rand.Intn(100)
	y := 20 + rand.Intn(60)
	page.Mouse.MustMoveTo(float64(x), float64(y)).MustClick(proto.InputMouseButtonLeft)
}

func mustClickPublishTab(page *rod.Page, tabname string) error {
	page.MustElement(`div.upload-content`).MustWaitVisible()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		tab, blocked, err := getTabElement(page, tabname)
		if err != nil {
			logrus.Warnf("获取发布 TAB 元素失败: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if tab == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if blocked {
			logrus.Info("发布 TAB 被遮挡，尝试移除遮挡")
			removePopCover(page)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if err := tab.Click(proto.InputMouseButtonLeft, 1); err != nil {
			logrus.Warnf("点击发布 TAB 失败: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		return nil
	}

	return errors.Errorf("没有找到发布 TAB - %s", tabname)
}

func getTabElement(page *rod.Page, tabname string) (*rod.Element, bool, error) {
	elems, err := page.Elements("div.creator-tab")
	if err != nil {
		return nil, false, err
	}

	for _, elem := range elems {
		if !isElementVisible(elem) {
			continue
		}

		text, err := elem.Text()
		if err != nil {
			logrus.Debugf("获取发布 TAB 文本失败: %v", err)
			continue
		}

		if strings.TrimSpace(text) != tabname {
			continue
		}

		blocked, err := isElementBlocked(elem)
		if err != nil {
			return nil, false, err
		}

		return elem, blocked, nil
	}

	return nil, false, nil
}

func isElementBlocked(elem *rod.Element) (bool, error) {
	result, err := elem.Eval(`() => {
		const rect = this.getBoundingClientRect();
		if (rect.width === 0 || rect.height === 0) {
			return true;
		}
		const x = rect.left + rect.width / 2;
		const y = rect.top + rect.height / 2;
		const target = document.elementFromPoint(x, y);
		return !(target === this || this.contains(target));
	}`)
	if err != nil {
		return false, err
	}

	return result.Value.Bool(), nil
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	// 验证文件路径有效性
	validPaths := make([]string, 0, len(imagesPaths))
	for _, path := range imagesPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			logrus.Warnf("图片文件不存在: %s", path)
			continue
		}
		validPaths = append(validPaths, path)

		logrus.Infof("获取有效图片：%s", path)
	}

	// 只为查找上传输入框和设置文件使用90秒超时
	// 不要将这个超时的page传递给waitForUploadComplete
	pp := page.Timeout(90 * time.Second)

	// 等待上传输入框出现
	uploadInput := pp.MustElement(".upload-input")
	logrus.Info("找到上传输入框，准备设置文件")

	// 上传多个文件
	uploadInput.MustSetFiles(validPaths...)
	logrus.Infof("已设置%d个文件到上传输入框", len(validPaths))

	// 等待一下，让浏览器开始处理文件
	time.Sleep(2 * time.Second)

	// 传递原始的page对象(没有90秒超时限制),让waitForUploadComplete自己管理超时
	return waitForUploadComplete(page, len(validPaths))
}

// waitForUploadComplete 等待并验证上传完成
func waitForUploadComplete(page *rod.Page, expectedCount int) error {
	maxWaitTime := 120 * time.Second
	checkInterval := 500 * time.Millisecond
	start := time.Now()

	// 创建一个新的超时上下文,避免使用已过期的上下文
	// 这很重要,因为传入的page可能已经有一个过期的超时
	pageWithTimeout := page.Timeout(maxWaitTime)

	slog.Info("开始等待图片上传完成", "expected_count", expectedCount)

	// 如果图片数量超过6张，需要先点击"展开"按钮才能看到所有图片
	// 这个操作只需要执行一次
	if expectedCount > 6 {
		// 1. 首先检查是否有"封面优化建议"弹窗遮挡
		// 根据用户提供的HTML: <div class="title">【封面优化建议】功能上线啦</div> ... <button>立即体验</button>
		// 尝试点击"立即体验"或者关闭按钮
		modalSelectors := []string{
			"div.container .title", // 包含"封面优化建议"的容器
		}

		for _, selector := range modalSelectors {
			if elems, err := pageWithTimeout.Elements(selector); err == nil {
				for _, elem := range elems {
					if text, _ := elem.Text(); strings.Contains(text, "封面优化建议") {
						slog.Info("检测到'封面优化建议'弹窗，尝试关闭")
						// 尝试点击"立即体验"按钮
						if btn, err := pageWithTimeout.Element("div.container button.d-button"); err == nil {
							slog.Info("点击'立即体验'以关闭弹窗")
							btn.Click(proto.InputMouseButtonLeft, 1)
							time.Sleep(1 * time.Second)
						}
					}
				}
			}
		}

		// 2. 尝试点击展开按钮
		// 根据实际DOM结构，展开按钮在 .img-list-footer .footer-btn
		expandSelectors := []string{
			".img-list-footer .footer-btn",
			".img-list-footer",
		}

		expanded := false
		// 最多尝试3次点击展开
		for i := 0; i < 3; i++ {
			if expanded {
				break
			}

			for _, selector := range expandSelectors {
				expandBtn, err := pageWithTimeout.Element(selector)
				if err == nil && expandBtn != nil {
					// 检查按钮是否可见
					if visible, _ := expandBtn.Visible(); visible {
						slog.Info("找到展开按钮，尝试点击", "selector", selector, "attempt", i+1)

						// 尝试滚动到按钮位置，防止被遮挡
						expandBtn.ScrollIntoView()
						time.Sleep(500 * time.Millisecond)

						// 尝试点击
						if err := expandBtn.Click(proto.InputMouseButtonLeft, 1); err == nil {
							slog.Info("点击动作执行成功，等待验证")
							time.Sleep(1 * time.Second)

							// 验证点击是否生效：检查按钮是否消失，或者是否变成了"收起"状态
							// 这里简单判断：如果按钮不再可见，或者文本变了，或者第7张图片可见了，就认为成功

							// 检查第7张图片是否可见（如果有的话）
							if images, _ := pageWithTimeout.Elements(".pr"); len(images) > 6 {
								if vis, _ := images[6].Visible(); vis {
									slog.Info("验证成功：第7张图片已可见")
									expanded = true
									break
								}
							}

							// 如果按钮本身消失了，也算成功
							if vis, _ := expandBtn.Visible(); !vis {
								slog.Info("验证成功：展开按钮已消失")
								expanded = true
								break
							}

							slog.Warn("点击后未检测到展开效果，准备重试")
						} else {
							slog.Warn("点击操作失败", "error", err)
						}
					} else {
						// 如果按钮不可见，可能已经展开了
						slog.Info("展开按钮不可见，假设已展开")
						expanded = true
						break
					}
				}
			}
			time.Sleep(1 * time.Second)
		}

		if !expanded {
			slog.Warn("多次尝试后仍未确认展开成功，继续尝试检测图片")
		}
	}

	for time.Since(start) < maxWaitTime {

		// 根据实际DOM结构,使用正确的选择器
		// 首先等待至少一个图片元素出现
		var uploadedImages []*rod.Element
		var err error

		// 等待第一个图片元素出现(最多等待5秒,因为上传需要时间)
		firstImageFound := false
		waitStart := time.Now()
		attemptCount := 0
		for time.Since(waitStart) < 5*time.Second {
			attemptCount++
			uploadedImages, err = pageWithTimeout.Elements(".pr")
			if err == nil && len(uploadedImages) > 0 {
				slog.Info("找到图片元素", "selector", ".pr", "count", len(uploadedImages), "elapsed", time.Since(waitStart).String())
				firstImageFound = true
				break
			}
			time.Sleep(200 * time.Millisecond)
		}

		if !firstImageFound {
			slog.Debug("主选择器未找到图片,尝试备选选择器", "attempts", attemptCount)
			// 如果.pr找不到,尝试其他选择器
			imageSelectors := []string{
				".img-preview-area .pr",
				".flex-list .pr",
				"div[data-draggable='true']", // 根据HTML,所有图片都有这个属性
			}

			for _, selector := range imageSelectors {
				uploadedImages, err = pageWithTimeout.Elements(selector)
				if err == nil && len(uploadedImages) > 0 {
					slog.Info("使用备选选择器找到图片", "selector", selector, "count", len(uploadedImages))
					firstImageFound = true
					break
				}
			}
		}

		if !firstImageFound {
			elapsed := time.Since(start)
			slog.Warn("未找到已上传图片,所有选择器都无效", "elapsed", elapsed.String(), "attempts", attemptCount)

			// 输出页面HTML用于调试
			if html, err := pageWithTimeout.HTML(); err == nil {
				slog.Debug("页面HTML快照(前2000字符)", "html", html[:min(2000, len(html))])
			}
		} else {
			currentCount := len(uploadedImages)
			slog.Info("检测到图片元素", "current_count", currentCount, "expected_count", expectedCount)

			if currentCount >= expectedCount {
				// 严格检查：确保每张图片都已经真正上传完成（有src属性，且不是loading状态）
				allUploaded := true
				for i, img := range uploadedImages {
					// 检查是否有 img 标签
					imgTag, err := img.Element("img")
					if err != nil {
						slog.Warn("图片元素中未找到img标签", "index", i)
						allUploaded = false
						break
					}

					// 检查 src 属性
					src, err := imgTag.Attribute("src")
					if err != nil || src == nil || *src == "" {
						slog.Warn("图片src为空，可能还在上传中", "index", i)
						allUploaded = false
						break
					}

					// 检查是否是 blob 开头（通常是预览图）或者 http 开头
					if !strings.HasPrefix(*src, "blob:") && !strings.HasPrefix(*src, "http") && !strings.HasPrefix(*src, "data:") {
						slog.Warn("图片src格式异常", "index", i, "src", *src)
						// 这里不一定算失败，但值得记录
					}

					// 检查是否有 loading 遮罩 (根据之前的HTML，上传完成的图片有 .mask，但没有 loading 相关的类)
					// 如果有 .uploading 或者类似的类，应该在这里检查
					// 目前假设只要有 img 且 src 正常就算成功
				}

				if allUploaded {
					// 进一步验证：
					// 1. 检查页面上是否有"上传中"、"处理中"等提示
					// 2. 检查发布按钮是否处于可点击状态

					// 检查是否有正在上传的提示
					hasUploadingText := false
					uploadingKeywords := []string{"上传中", "处理中", "准备中"}
					for _, keyword := range uploadingKeywords {
						// 使用XPath查找包含特定文本的元素
						// //*[contains(text(), '上传中')]
						xpath := fmt.Sprintf("//*[contains(text(), '%s')]", keyword)
						if elems, err := pageWithTimeout.ElementsX(xpath); err == nil && len(elems) > 0 {
							for _, elem := range elems {
								if vis, _ := elem.Visible(); vis {
									slog.Info("检测到上传状态提示", "text", keyword)
									hasUploadingText = true
									break
								}
							}
						}
						if hasUploadingText {
							break
						}
					}

					if hasUploadingText {
						slog.Info("检测到上传/处理中提示，继续等待...")
						time.Sleep(1 * time.Second)
						continue
					}

					// 检查发布按钮状态
					// 发布按钮通常在 div.submit
					submitBtn, err := pageWithTimeout.Element("div.submit")
					if err == nil {
						// 检查是否有 disabled 类或者属性
						// 小红书的发布按钮在不可用时通常会有 disabled 类或者样式改变
						// 这里打印一下class属性帮助调试
						if classAttr, err := submitBtn.Attribute("class"); err == nil && classAttr != nil {
							slog.Debug("发布按钮Class", "class", *classAttr)
							if strings.Contains(*classAttr, "disabled") {
								slog.Info("发布按钮处于禁用状态，可能还在上传中...")
								time.Sleep(1 * time.Second)
								continue
							}
						}
					}

					// 稳定性检查：确保状态持续一段时间
					// 如果是刚检测到完成，不要立即返回，而是多等几秒确认
					slog.Info("初步检测到上传完成，进行稳定性等待(3秒)...", "count", currentCount)
					time.Sleep(3 * time.Second)

					// 再次检查图片数量，确保没有变化
					finalImages, _ := pageWithTimeout.Elements(".pr")
					if len(finalImages) >= expectedCount {
						slog.Info("所有图片上传并验证完成(稳定性检查通过)", "count", len(finalImages))
						return nil
					} else {
						slog.Warn("稳定性检查未通过，图片数量发生变化", "before", currentCount, "after", len(finalImages))
					}
				} else {
					slog.Info("部分图片尚未上传完成，继续等待...")
				}
			}
		}

		time.Sleep(checkInterval)
	}

	return errors.New("上传超时，请检查网络连接和图片大小")
}

func submitPublish(page *rod.Page, title, content string, tags []string) error {

	if err := fillPublishContent(page, title, content, tags); err != nil {
		return errors.Wrap(err, "填写发布内容失败")
	}

	time.Sleep(1 * time.Second)

	// 提交逻辑改为带重试的机制
	// 因为点击发布时可能会提示"图片正在上传中"
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		slog.Info("尝试点击发布按钮", "attempt", i+1)

		submitButton := page.MustElement("div.submit div.d-button-content")
		if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return errors.Wrap(err, "点击发布按钮失败")
		}

		// 点击后等待检查是否有错误提示
		time.Sleep(2 * time.Second)

		// 检查页面是否有错误提示
		// 常见的错误提示关键词
		errorKeywords := []string{
			"图片正在上传",
			"图片上传中",
			"上传未完成",
			"请等待上传",
			"上传失败",
		}

		hasError := false
		for _, keyword := range errorKeywords {
			// 使用XPath查找包含特定文本的元素
			xpath := fmt.Sprintf("//*[contains(text(), '%s')]", keyword)
			if elems, err := page.ElementsX(xpath); err == nil && len(elems) > 0 {
				for _, elem := range elems {
					if vis, _ := elem.Visible(); vis {
						slog.Warn("检测到发布错误提示", "text", keyword)
						hasError = true
						break
					}
				}
			}
			if hasError {
				break
			}
		}

		if hasError {
			slog.Info("等待图片上传完成，5秒后重试...")
			time.Sleep(5 * time.Second)
			continue
		}

		// 如果没有错误提示，假设发布成功
		// 这里添加更严格的成功检查
		slog.Info("未检测到错误提示，检查是否发布成功")

		// 检查是否有成功提示
		successKeywords := []string{"发布成功", "发布完成"}
		isSuccess := false
		for _, keyword := range successKeywords {
			xpath := fmt.Sprintf("//*[contains(text(), '%s')]", keyword)
			if elems, err := page.ElementsX(xpath); err == nil && len(elems) > 0 {
				for _, elem := range elems {
					if vis, _ := elem.Visible(); vis {
						slog.Info("检测到发布成功提示", "text", keyword)
						isSuccess = true
						break
					}
				}
			}
			if isSuccess {
				break
			}
		}

		if isSuccess {
			break
		}

		// 检查URL是否发生变化（通常发布成功后会跳转）
		// 或者检查发布按钮是否消失
		if _, err := page.Element("div.submit div.d-button-content"); err != nil {
			slog.Info("发布按钮已消失，推测发布成功")
			break
		}

		slog.Info("未检测到明确成功信号，但也没有错误，继续流程")
		break
	}

	time.Sleep(3 * time.Second)

	return nil
}

func fillPublishContent(page *rod.Page, title, content string, tags []string) error {
	titleElem := page.MustElement("div.d-input input")
	titleElem.MustInput(title)

	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		contentElem.MustInput(content)

		// 传递page对象以确保使用正确的超时设置
		inputTags(page, contentElem, tags)

	} else {
		return errors.New("没有找到内容输入框")
	}
	return nil
}

// 查找内容输入框 - 使用Race方法处理两种样式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	var foundElement *rod.Element
	var found bool

	page.Race().
		Element("div.ql-editor").MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		ElementFunc(func(page *rod.Page) (*rod.Element, error) {
			return findTextboxByPlaceholder(page)
		}).MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		MustDo()

	if found {
		return foundElement, true
	}

	slog.Warn("no content element found by any method")
	return nil, false
}

func inputTags(page *rod.Page, contentElem *rod.Element, tags []string) {
	if len(tags) == 0 {
		return
	}

	time.Sleep(1 * time.Second)

	for i := 0; i < 20; i++ {
		contentElem.MustKeyActions().
			Type(input.ArrowDown).
			MustDo()
		time.Sleep(10 * time.Millisecond)
	}

	contentElem.MustKeyActions().
		Press(input.Enter).
		Press(input.Enter).
		MustDo()

	time.Sleep(1 * time.Second)

	for _, tag := range tags {
		tag = strings.TrimLeft(tag, "#")
		inputTag(page, contentElem, tag)
	}
}

func inputTag(page *rod.Page, contentElem *rod.Element, tag string) {
	contentElem.MustInput("#")
	time.Sleep(200 * time.Millisecond)

	for _, char := range tag {
		contentElem.MustInput(string(char))
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	// 使用传入的page对象，确保使用正确的超时设置
	topicContainer, err := page.Element("#creator-editor-topic-container")
	if err == nil && topicContainer != nil {
		firstItem, err := topicContainer.Element(".item")
		if err == nil && firstItem != nil {
			firstItem.MustClick()
			slog.Info("成功点击标签联想选项", "tag", tag)
			time.Sleep(200 * time.Millisecond)
		} else {
			slog.Warn("未找到标签联想选项，直接输入空格", "tag", tag)
			// 如果没有找到联想选项，输入空格结束
			contentElem.MustInput(" ")
		}
	} else {
		slog.Warn("未找到标签联想下拉框，直接输入空格", "tag", tag)
		// 如果没有找到下拉框，输入空格结束
		contentElem.MustInput(" ")
	}

	time.Sleep(500 * time.Millisecond) // 等待标签处理完成
}

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	// 查找包含指定placeholder的元素
	placeholderElem := findPlaceholderElement(elements, "输入正文描述")
	if placeholderElem == nil {
		return nil, errors.New("no placeholder element found")
	}

	// 向上查找textbox父元素
	textboxElem := findTextboxParent(placeholderElem)
	if textboxElem == nil {
		return nil, errors.New("no textbox parent found")
	}

	return textboxElem, nil
}

func findPlaceholderElement(elements []*rod.Element, searchText string) *rod.Element {
	for _, elem := range elements {
		placeholder, err := elem.Attribute("data-placeholder")
		if err != nil || placeholder == nil {
			continue
		}

		if strings.Contains(*placeholder, searchText) {
			return elem
		}
	}
	return nil
}

func findTextboxParent(elem *rod.Element) *rod.Element {
	currentElem := elem
	for i := 0; i < 5; i++ {
		parent, err := currentElem.Parent()
		if err != nil {
			break
		}

		role, err := parent.Attribute("role")
		if err != nil || role == nil {
			currentElem = parent
			continue
		}

		if *role == "textbox" {
			return parent
		}

		currentElem = parent
	}
	return nil
}

// isElementVisible 检查元素是否可见
func isElementVisible(elem *rod.Element) bool {

	// 检查是否有隐藏样式
	style, err := elem.Attribute("style")
	if err == nil && style != nil {
		styleStr := *style

		if strings.Contains(styleStr, "left: -9999px") ||
			strings.Contains(styleStr, "top: -9999px") ||
			strings.Contains(styleStr, "position: absolute; left: -9999px") ||
			strings.Contains(styleStr, "display: none") ||
			strings.Contains(styleStr, "visibility: hidden") {
			return false
		}
	}

	visible, err := elem.Visible()
	if err != nil {
		slog.Warn("无法获取元素可见性", "error", err)
		return true
	}

	return visible
}
