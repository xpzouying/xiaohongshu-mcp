package xiaohongshu

import (
	"context"
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
	Title        string
	Content      string
	Tags         []string
	ImagePaths   []string
	ScheduleTime *time.Time // 定时发布时间，nil 表示立即发布
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(300 * time.Second)

	// 使用更稳健的导航和等待策略
	if err := pp.Navigate(urlOfPublic); err != nil {
		return nil, errors.Wrap(err, "导航到发布页面失败")
	}

	// 等待页面加载，使用 WaitLoad 代替 WaitIdle（更宽松）
	if err := pp.WaitLoad(); err != nil {
		logrus.Warnf("等待页面加载出现问题: %v，继续尝试", err)
	}
	time.Sleep(2 * time.Second)

	// 等待页面稳定
	if err := pp.WaitDOMStable(time.Second, 0.1); err != nil {
		logrus.Warnf("等待 DOM 稳定出现问题: %v，继续尝试", err)
	}
	time.Sleep(1 * time.Second)

	if err := mustClickPublishTab(pp, "上传图文"); err != nil {
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

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	tags := content.Tags
	if len(tags) >= 10 {
		logrus.Warnf("标签数量超过10，截取前10个标签")
		tags = tags[:10]
	}

	logrus.Infof("发布内容: title=%s, images=%v, tags=%v, schedule=%v", content.Title, len(content.ImagePaths), tags, content.ScheduleTime)

	if err := submitPublish(page, content.Title, content.Content, tags, content.ScheduleTime); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

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

	// 逐张上传：每张上传后等待预览出现，再上传下一张
	for i, path := range validPaths {
		selector := `input[type="file"]`
		if i == 0 {
			selector = ".upload-input"
		}

		uploadInput, err := page.Element(selector)
		if err != nil {
			return errors.Wrapf(err, "查找上传输入框失败(第%d张)", i+1)
		}
		if err := uploadInput.SetFiles([]string{path}); err != nil {
			return errors.Wrapf(err, "上传第%d张图片失败", i+1)
		}

		slog.Info("图片已提交上传", "index", i+1, "path", path)

		// 等待当前图片上传完成（预览元素数量达到 i+1），最多等 60 秒
		if err := waitForUploadComplete(page, i+1); err != nil {
			return errors.Wrapf(err, "第%d张图片上传超时", i+1)
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

// waitForUploadComplete 等待第 expectedCount 张图片上传完成，最多等 60 秒
func waitForUploadComplete(page *rod.Page, expectedCount int) error {
	maxWaitTime := 60 * time.Second
	checkInterval := 500 * time.Millisecond
	start := time.Now()
	lastLogCount := expectedCount - 1

	for time.Since(start) < maxWaitTime {
		uploadedImages, err := page.Elements(".img-preview-area .pr")
		if err != nil {
			time.Sleep(checkInterval)
			continue
		}

		currentCount := len(uploadedImages)
		// 数量变化时才打印，避免刷屏
		if currentCount != lastLogCount {
			slog.Info("等待图片上传", "current", currentCount, "expected", expectedCount)
			lastLogCount = currentCount
		}
		if currentCount >= expectedCount {
			slog.Info("图片上传完成", "count", currentCount)
			return nil
		}

		time.Sleep(checkInterval)
	}

	return errors.Errorf("第%d张图片上传超时(60s)，请检查网络连接和图片大小", expectedCount)
}

func submitPublish(page *rod.Page, title, content string, tags []string, scheduleTime *time.Time) error {
	titleElem, err := page.Element("div.d-input input")
	if err != nil {
		return errors.Wrap(err, "查找标题输入框失败")
	}
	if err := titleElem.Input(title); err != nil {
		return errors.Wrap(err, "输入标题失败")
	}

	// 检查标题长度
	time.Sleep(500 * time.Millisecond)
	if err := checkTitleMaxLength(page); err != nil {
		return err
	}
	slog.Info("检查标题长度：通过")

	time.Sleep(1 * time.Second)

	contentElem, ok := getContentElement(page)
	if !ok {
		return errors.New("没有找到内容输入框")
	}
	if err := contentElem.Input(content); err != nil {
		return errors.Wrap(err, "输入正文失败")
	}
	if err := inputTags(contentElem, tags); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// 检查正文长度
	if err := checkContentMaxLength(page); err != nil {
		return err
	}
	slog.Info("检查正文长度：通过")

	// 处理定时发布
	if scheduleTime != nil {
		if err := setSchedulePublish(page, *scheduleTime); err != nil {
			return errors.Wrap(err, "设置定时发布失败")
		}
		slog.Info("定时发布设置完成", "schedule_time", scheduleTime.Format("2006-01-02 15:04"))
	}

	submitButton, err := page.Element(".publish-page-publish-btn button.bg-red")
	if err != nil {
		return errors.Wrap(err, "查找发布按钮失败")
	}
	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击发布按钮失败")
	}

	time.Sleep(3 * time.Second)
	return nil
}

// 检查标题是否超过最大长度
func checkTitleMaxLength(page *rod.Page) error {
	has, elem, err := page.Has(`div.title-container div.max_suffix`)
	if err != nil {
		return errors.Wrap(err, "检查标题长度元素失败")
	}

	// 元素不存在，说明标题没超长
	if !has {
		return nil
	}

	// 元素存在，说明标题超长
	titleLength, err := elem.Text()
	if err != nil {
		return errors.Wrap(err, "获取标题长度文本失败")
	}

	return makeMaxLengthError(titleLength)
}

func checkContentMaxLength(page *rod.Page) error {
	has, elem, err := page.Has(`div.edit-container div.length-error`)
	if err != nil {
		return errors.Wrap(err, "检查正文长度元素失败")
	}

	// 元素不存在，说明正文没超长
	if !has {
		return nil
	}

	// 元素存在，说明正文超长
	contentLength, err := elem.Text()
	if err != nil {
		return errors.Wrap(err, "获取正文长度文本失败")
	}

	return makeMaxLengthError(contentLength)
}

func makeMaxLengthError(elemText string) error {
	parts := strings.Split(elemText, "/")
	if len(parts) != 2 {
		return errors.Errorf("长度超过限制: %s", elemText)
	}

	currLen, maxLen := parts[0], parts[1]

	return errors.Errorf("当前输入长度为%s，最大长度为%s", currLen, maxLen)
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

func inputTags(contentElem *rod.Element, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	time.Sleep(1 * time.Second)

	for i := 0; i < 20; i++ {
		ka, err := contentElem.KeyActions()
		if err != nil {
			return errors.Wrap(err, "创建键盘操作失败")
		}
		if err := ka.Type(input.ArrowDown).Do(); err != nil {
			return errors.Wrap(err, "按下方向键失败")
		}
		time.Sleep(10 * time.Millisecond)
	}

	ka, err := contentElem.KeyActions()
	if err != nil {
		return errors.Wrap(err, "创建键盘操作失败")
	}
	if err := ka.Press(input.Enter).Press(input.Enter).Do(); err != nil {
		return errors.Wrap(err, "按下回车键失败")
	}

	time.Sleep(1 * time.Second)

	for _, tag := range tags {
		tag = strings.TrimLeft(tag, "#")
		if err := inputTag(contentElem, tag); err != nil {
			return errors.Wrapf(err, "输入标签[%s]失败", tag)
		}
	}
	return nil
}

func inputTag(contentElem *rod.Element, tag string) error {
	if err := contentElem.Input("#"); err != nil {
		return errors.Wrap(err, "输入#失败")
	}
	time.Sleep(200 * time.Millisecond)

	for _, char := range tag {
		if err := contentElem.Input(string(char)); err != nil {
			return errors.Wrapf(err, "输入字符[%c]失败", char)
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	page := contentElem.Page()
	topicContainer, err := page.Element("#creator-editor-topic-container")
	if err != nil || topicContainer == nil {
		slog.Warn("未找到标签联想下拉框，直接输入空格", "tag", tag)
		return contentElem.Input(" ")
	}

	firstItem, err := topicContainer.Element(".item")
	if err != nil || firstItem == nil {
		slog.Warn("未找到标签联想选项，直接输入空格", "tag", tag)
		return contentElem.Input(" ")
	}

	if err := firstItem.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击标签联想选项失败")
	}
	slog.Info("成功点击标签联想选项", "tag", tag)
	time.Sleep(200 * time.Millisecond)

	time.Sleep(500 * time.Millisecond) // 等待标签处理完成
	return nil
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

// setSchedulePublish 设置定时发布时间
func setSchedulePublish(page *rod.Page, t time.Time) error {
	// 1. 点击定时发布开关
	if err := clickScheduleSwitch(page); err != nil {
		return err
	}
	time.Sleep(800 * time.Millisecond)

	// 2. 设置日期时间
	if err := setDateTime(page, t); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)

	return nil
}

// clickScheduleSwitch 点击定时发布开关
func clickScheduleSwitch(page *rod.Page) error {
	switchElem, err := page.Element(".post-time-wrapper .d-switch")
	if err != nil {
		return errors.Wrap(err, "查找定时发布开关失败")
	}

	if err := switchElem.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击定时发布开关失败")
	}
	slog.Info("已点击定时发布开关")
	return nil
}

// setDateTime 设置日期时间
func setDateTime(page *rod.Page, t time.Time) error {
	dateTimeStr := t.Format("2006-01-02 15:04")

	input, err := page.Element(".date-picker-container input")
	if err != nil {
		return errors.Wrap(err, "查找日期时间输入框失败")
	}

	if err := input.SelectAllText(); err != nil {
		return errors.Wrap(err, "选择日期时间文本失败")
	}
	if err := input.Input(dateTimeStr); err != nil {
		return errors.Wrap(err, "输入日期时间失败")
	}
	slog.Info("已设置日期时间", "datetime", dateTimeStr)

	return nil
}
