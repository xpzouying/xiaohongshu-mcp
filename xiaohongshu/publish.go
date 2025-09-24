package xiaohongshu

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	Tags       []string
	ImagePaths []string
}

// ScheduledPublishImageContent 定时发布图文内容
type ScheduledPublishImageContent struct {
	Title       string
	Content     string
	Tags        []string
	ImagePaths  []string
	PublishTime *time.Time // 定时发布时间，nil表示立即发布
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(60 * time.Second)

	if err := pp.Navigate(urlOfPublic); err != nil {
		return nil, errors.Wrap(err, "导航到发布页面失败")
	}

	uploadContentElem, err := pp.Element(`div.upload-content`)
	if err != nil {
		return nil, errors.Wrap(err, "找不到上传内容元素")
	}

	if err := uploadContentElem.WaitVisible(); err != nil {
		return nil, errors.Wrap(err, "等待上传内容可见失败")
	}
	slog.Info("wait for upload-content visible success")

	// 等待一段时间确保页面完全加载
	time.Sleep(1 * time.Second)

	createElems, err := pp.Elements("div.creator-tab")
	if err != nil {
		return nil, errors.Wrap(err, "找不到创建标签元素")
	}

	// 过滤掉隐藏的元素
	var visibleElems []*rod.Element
	for _, elem := range createElems {
		if isElementVisible(elem) {
			visibleElems = append(visibleElems, elem)
		}
	}

	if len(visibleElems) == 0 {
		return nil, errors.New("没有找到上传图文元素")
	}

	for _, elem := range visibleElems {
		text, err := elem.Text()
		if err != nil {
			slog.Error("获取元素文本失败", "error", err)
			continue
		}

		if text == "上传图文" {
			if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
				slog.Error("点击元素失败", "error", err)
				continue
			}
			break
		}
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

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitPublish(page, content.Title, content.Content, content.Tags); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func (p *PublishAction) PublishScheduled(ctx context.Context, content ScheduledPublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitScheduledPublish(page, content.Title, content.Content, content.Tags, content.PublishTime); err != nil {
		return errors.Wrap(err, "小红书定时发布失败")
	}

	return nil
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)

	// 验证文件路径有效性
	for _, path := range imagesPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.Wrapf(err, "图片文件不存在: %s", path)
		}
	}

	// 等待上传输入框出现
	uploadInput, err := pp.Element(".upload-input")
	if err != nil {
		return errors.Wrap(err, "找不到上传输入框")
	}

	// 上传多个文件
	if err := uploadInput.SetFiles(imagesPaths); err != nil {
		return errors.Wrap(err, "设置上传文件失败")
	}

	// 等待并验证上传完成
	return waitForUploadComplete(pp, len(imagesPaths))
}

// waitForUploadComplete 等待并验证上传完成
func waitForUploadComplete(page *rod.Page, expectedCount int) error {
	maxWaitTime := 60 * time.Second
	checkInterval := 500 * time.Millisecond
	start := time.Now()

	slog.Info("开始等待图片上传完成", "expected_count", expectedCount)

	for time.Since(start) < maxWaitTime {
		// 使用具体的pr类名检查已上传的图片
		uploadedImages, err := page.Elements(".img-preview-area .pr")

		slog.Info("uploadedImages", "uploadedImages", uploadedImages)

		if err == nil {
			currentCount := len(uploadedImages)
			slog.Info("检测到已上传图片", "current_count", currentCount, "expected_count", expectedCount)
			if currentCount >= expectedCount {
				slog.Info("所有图片上传完成", "count", currentCount)
				return nil
			}
		} else {
			slog.Debug("未找到已上传图片元素")
		}

		time.Sleep(checkInterval)
	}

	return errors.New("上传超时，请检查网络连接和图片大小")
}

func submitPublish(page *rod.Page, title, content string, tags []string) error {
	// 使用更长的超时时间
	pp := page.Timeout(30 * time.Second)

	titleElem, err := pp.Element("div.d-input input")
	if err != nil {
		return errors.Wrap(err, "找不到标题输入框")
	}

	if err := titleElem.Input(title); err != nil {
		return errors.Wrap(err, "输入标题失败")
	}

	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		if err := contentElem.Input(content); err != nil {
			return errors.Wrap(err, "输入内容失败")
		}
		inputTags(contentElem, tags)
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	submitButton, err := pp.Element("div.submit button.d-button")
	if err != nil {
		return errors.Wrap(err, "找不到提交按钮")
	}

	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击提交按钮失败")
	}

	time.Sleep(3 * time.Second)

	return nil
}

func submitScheduledPublish(page *rod.Page, title, content string, tags []string, publishTime *time.Time) error {
	// 使用更长的超时时间
	pp := page.Timeout(30 * time.Second)

	titleElem, err := pp.Element("div.d-input input")
	if err != nil {
		return errors.Wrap(err, "找不到标题输入框")
	}

	if err := titleElem.Input(title); err != nil {
		return errors.Wrap(err, "输入标题失败")
	}

	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		if err := contentElem.Input(content); err != nil {
			return errors.Wrap(err, "输入内容失败")
		}
		inputTags(contentElem, tags)
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	// 总是执行定时发布设置，即使 publishTime 为 nil（使用默认时间）
	var timeToSet time.Time
	if publishTime != nil {
		timeToSet = *publishTime
	} else {
		// 使用默认时间（当前时间+1小时）
		timeToSet = time.Now().Add(1 * time.Hour)
		slog.Info("使用默认定时发布时间", "default_time", timeToSet.Format("2006-01-02 15:04"))
	}

	if err := setScheduledPublishTime(page, timeToSet); err != nil {
		return errors.Wrap(err, "设置定时发布时间失败")
	}

	time.Sleep(2 * time.Second) // 等待界面更新

	// submitScheduledPublish 函数专用于定时发布，总是寻找"定时发布"按钮
	var submitButton *rod.Element

	// 使用更精确的选择器寻找定时发布按钮
	submitButton, err = pp.Element("button.publishBtn")
	if err != nil {
		slog.Warn("找不到publishBtn按钮，尝试通过文本查找")
		// 备用方案：通过span文本内容查找
		submitButton, err = pp.Element("button:has(span:contains('定时发布'))")
		if err != nil {
			slog.Warn("通过文本也找不到，尝试第一个红色按钮")
			// 最后备用方案：红色按钮
			submitButton, err = pp.Element("button.red.publishBtn, button.d-button.red")
			if err != nil {
				return errors.Wrap(err, "找不到定时发布按钮")
			}
		}
	}
	slog.Info("找到定时发布按钮")

	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击提交按钮失败")
	}

	time.Sleep(3 * time.Second)

	return nil
}

func setScheduledPublishTime(page *rod.Page, publishTime time.Time) error {
	slog.Info("设置定时发布选项")

	// 使用合理的超时时间
	pp := page.Timeout(10 * time.Second)

	// 查找定时发布的 radio button - 根据精确的HTML结构更新选择器
	// 使用精确的选择器直接找到定时发布的label
	timingLabel, err := pp.Element("label.el-radio:has(input[value='1']) .el-radio__label:contains('定时发布')")
	if err != nil {
		// 备选方案：通过input的value找到对应的label
		timingLabel, err = pp.Element("input.el-radio__original[value='1'][type='radio']")
		if err == nil {
			// 找到input后，获取其父级label
			if parent1, err1 := timingLabel.Parent(); err1 == nil {
				if parentLabel, parentErr := parent1.Parent(); parentErr == nil {
					timingLabel = parentLabel
					slog.Info("通过input找到父级label")
				}
			}
		}
	}

	if err != nil {
		return errors.Wrap(err, "找不到定时发布选项")
	}

	// 检查是否已经选中
	if hasClass, classErr := timingLabel.Eval("() => this.classList.contains('is-checked')"); classErr == nil && hasClass != nil && hasClass.Value.Bool() {
		slog.Info("定时发布选项已经被选中")
	} else {
		slog.Info("点击定时发布选项")
		// 直接点击label元素
		if err := timingLabel.Click(proto.InputMouseButtonLeft, 1); err != nil {
			slog.Warn("直接点击失败，尝试JavaScript点击", "error", err)
			if _, jsErr := timingLabel.Eval("() => this.click()"); jsErr != nil {
				return errors.Wrap(err, "点击定时发布选项失败")
			}
			slog.Info("JavaScript点击成功")
		} else {
			slog.Info("直接点击成功")
		}
	}

	// 等待一下让界面响应，确保定时发布选项生效
	time.Sleep(1 * time.Second)

	// 等待date-picker出现（这表明定时发布被正确选中）
	datePicker, datePickerErr := pp.Timeout(5 * time.Second).Element("div.date-picker")
	if datePickerErr == nil {
		slog.Info("date-picker出现，定时发布选择成功")

		// 获取并打印定时发布的时间信息
		timeInput, timeErr := datePicker.Element("input.el-input__inner")
		if timeErr == nil {
			if timeValue, valueErr := timeInput.Attribute("value"); valueErr == nil && timeValue != nil && *timeValue != "" {
				slog.Info("当前设置的定时发布时间", "time", *timeValue)
			} else if placeholder, placeholderErr := timeInput.Attribute("placeholder"); placeholderErr == nil && placeholder != nil {
				slog.Info("定时发布时间输入框为空，将使用系统默认时间", "placeholder", *placeholder)
			}
		} else {
			slog.Warn("无法找到定时发布时间输入框", "error", timeErr)
		}
	} else {
		slog.Warn("未找到date-picker，定时发布可能未成功选中", "error", datePickerErr)
	}

	// 再等待一下让界面完全稳定
	time.Sleep(1 * time.Second)

	slog.Info("定时发布选项设置完成，使用系统默认时间")
	return nil
}

// 查找内容输入框 - 使用多种方法处理不同的页面样式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	// 使用timeout来避免无限等待
	pp := page.Timeout(10 * time.Second)

	// 方法1: 查找 contenteditable 的 div (新版界面)
	if elem, err := pp.Element("div[contenteditable='true']"); err == nil {
		slog.Info("找到 contenteditable div 内容输入框")
		return elem, true
	}

	// 方法2: 查找 TipTap 编辑器 (新版界面)
	if elem, err := pp.Element("div.tiptap.ProseMirror"); err == nil {
		slog.Info("找到 TipTap 内容编辑器")
		return elem, true
	}

	// 方法3: 查找传统的 ql-editor (旧版界面)
	if elem, err := pp.Element("div.ql-editor"); err == nil {
		slog.Info("找到 ql-editor 内容输入框")
		return elem, true
	}

	// 方法4: 通过placeholder查找 (备用方法)
	if elem, err := findTextboxByPlaceholder(pp); err == nil {
		slog.Info("通过 placeholder 找到内容输入框")
		return elem, true
	}

	slog.Warn("所有方法都无法找到内容输入框")
	return nil, false
}

func inputTags(contentElem *rod.Element, tags []string) {
	if len(tags) == 0 {
		return
	}

	time.Sleep(1 * time.Second)

	for i := 0; i < 20; i++ {
		keyActions, err := contentElem.KeyActions()
		if err != nil {
			slog.Warn("获取KeyActions失败", "error", err)
			break
		}
		if err := keyActions.Type(input.ArrowDown).Do(); err != nil {
			slog.Warn("移动到内容末尾失败", "error", err)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	keyActions, err := contentElem.KeyActions()
	if err != nil {
		slog.Warn("获取KeyActions失败", "error", err)
	} else {
		if err := keyActions.Press(input.Enter).Press(input.Enter).Do(); err != nil {
			slog.Warn("添加换行失败", "error", err)
		}
	}

	time.Sleep(1 * time.Second)

	for _, tag := range tags {
		tag = strings.TrimLeft(tag, "#")
		inputTag(contentElem, tag)
	}
}

func inputTag(contentElem *rod.Element, tag string) {
	if err := contentElem.Input("#"); err != nil {
		slog.Warn("输入#符号失败", "tag", tag, "error", err)
		return
	}
	time.Sleep(200 * time.Millisecond)

	for _, char := range tag {
		if err := contentElem.Input(string(char)); err != nil {
			slog.Warn("输入标签字符失败", "tag", tag, "char", string(char), "error", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	page := contentElem.Page()
	topicContainer, err := page.Element("#creator-editor-topic-container")
	if err == nil && topicContainer != nil {
		firstItem, err := topicContainer.Element(".item")
		if err == nil && firstItem != nil {
			if err := firstItem.Click(proto.InputMouseButtonLeft, 1); err != nil {
				slog.Warn("点击标签联想选项失败", "tag", tag, "error", err)
			} else {
				slog.Info("成功点击标签联想选项", "tag", tag)
				time.Sleep(200 * time.Millisecond)
				return
			}
		} else {
			slog.Warn("未找到标签联想选项，直接输入空格", "tag", tag)
		}

		// 如果没有找到联想选项或点击失败，输入空格结束
		if err := contentElem.Input(" "); err != nil {
			slog.Warn("输入空格失败", "tag", tag, "error", err)
		}
	} else {
		slog.Warn("未找到标签联想下拉框，直接输入空格", "tag", tag)
		// 如果没有找到下拉框，输入空格结束
		if err := contentElem.Input(" "); err != nil {
			slog.Warn("输入空格失败", "tag", tag, "error", err)
		}
	}

	time.Sleep(500 * time.Millisecond) // 等待标签处理完成
}

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	// 使用带超时的页面实例
	pp := page.Timeout(10 * time.Second)
	elements, err := pp.Elements("p")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find p elements")
	}
	if len(elements) == 0 {
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
