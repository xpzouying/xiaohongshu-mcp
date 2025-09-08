package xiaohongshu

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	ImagePaths []string
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(60 * time.Second)

	// 导航到发布页面
	if err := pp.Navigate(urlOfPublic); err != nil {
		return nil, errors.Wrap(err, "导航到发布页面失败")
	}

	// 等待页面加载完成
	if err := pp.WaitLoad(); err != nil {
		return nil, errors.Wrap(err, "等待页面加载失败")
	}

	// 等待页面稳定，确保所有JavaScript完全加载
	if err := pp.WaitStable(5 * time.Second); err != nil {
		return nil, errors.Wrap(err, "等待页面稳定失败")
	}

	// 等待页面完全渲染，确保所有动态内容加载完成
	time.Sleep(3 * time.Second)

	// 等待关键页面状态加载完成
	for i := 0; i < 60; i++ { // 最多等待60秒
		// 检查页面是否包含必要的元素，表明页面已完全加载
		if ready, _, _ := pp.Has(`div.upload-content, div.creator-tab, .main-container`); ready {
			slog.Info("页面关键元素已加载，页面准备就绪")
			break
		}
		time.Sleep(1 * time.Second)
		slog.Info("等待页面完全加载", "进度", fmt.Sprintf("%d/60秒", i+1))
	}

	// 额外等待确保页面完全稳定
	time.Sleep(2 * time.Second)

	// 等待上传内容区域出现，使用重试机制
	uploadContentExists := false
	for i := 0; i < 10; i++ {
		if exists, _, _ := pp.Has(`div.upload-content`); exists {
			uploadContentExists = true
			break
		}
		time.Sleep(1 * time.Second)
		slog.Info("等待上传内容区域出现", "尝试", i+1)
	}

	if !uploadContentExists {
		return nil, errors.New("上传内容区域未找到")
	}

	slog.Info("等待上传内容区域出现成功")

	// 等待一段时间确保页面完全加载
	time.Sleep(10 * time.Second)

	// 尝试查找并点击"上传图文"按钮
	createElems, err := pp.Elements("div.creator-tab")
	if err != nil {
		slog.Warn("未找到creator-tab元素", "error", err)
		createElems = []*rod.Element{} // 设置为空切片以避免panic
	}
	slog.Info("找到creator-tab元素", "count", len(createElems))
	for _, elem := range createElems {
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

	if err := submitPublish(page, content.Title, content.Content); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)

	// 等待上传输入框出现，使用重试机制
	uploadInputFound := false
	var uploadInput *rod.Element
	for i := 0; i < 10; i++ {
		if elem, err := pp.Element(".upload-input"); err == nil {
			uploadInput = elem
			uploadInputFound = true
			slog.Info("找到上传输入框")
			break
		}
		time.Sleep(1 * time.Second)
		slog.Info("等待上传输入框", "尝试", i+1)
	}

	if !uploadInputFound {
		return errors.New("未找到上传输入框")
	}

	// 上传多个文件
	if err := uploadInput.SetFiles(imagesPaths); err != nil {
		return errors.Wrap(err, "设置文件失败")
	}

	// 等待上传完成
	time.Sleep(3 * time.Second)

	return nil
}

func submitPublish(page *rod.Page, title, content string) error {

	// 查找标题输入框
	titleElem, err := page.Element("div.d-input input")
	if err != nil {
		return errors.Wrap(err, "未找到标题输入框")
	}
	if err := titleElem.Input(title); err != nil {
		return errors.Wrap(err, "输入标题失败")
	}

	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		if err := contentElem.Input(content); err != nil {
			return errors.Wrap(err, "输入内容失败")
		}
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	// 查找发布按钮
	submitButton, err := page.Element("div.submit div.d-button-content")
	if err != nil {
		return errors.Wrap(err, "未找到发布按钮")
	}
	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击发布按钮失败")
	}

	time.Sleep(3 * time.Second)

	return nil
}

// 查找内容输入框 - 使用安全的方法处理两种样式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	// 先尝试第一种方式：ql-editor
	if elem, err := page.Element("div.ql-editor"); err == nil {
		slog.Info("找到ql-editor内容输入框")
		return elem, true
	}

	// 再尝试第二种方式：placeholder方式
	if elem, err := findTextboxByPlaceholder(page); err == nil {
		slog.Info("找到placeholder内容输入框")
		return elem, true
	}

	slog.Warn("所有方法都未找到内容输入框")
	return nil, false
}

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements, err := page.Elements("p")
	if err != nil {
		return nil, errors.Wrap(err, "未找到p元素")
	}
	if elements == nil {
		return nil, errors.New("未找到p元素")
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
