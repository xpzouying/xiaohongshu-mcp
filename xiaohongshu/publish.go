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
	pp := page.Timeout(300 * time.Second) // 延长超时时间

	logrus.Info("开始导航到发布页面")
	// 安全导航
	if err := pp.Navigate(urlOfPublic); err != nil {
		logrus.Errorf("导航失败: %v", err)
		return nil, errors.Wrap(err, "导航到发布页面失败")
	}

	logrus.Info("导航成功，等待页面加载")
	// 安全等待页面加载
	if err := pp.WaitLoad(); err != nil {
		logrus.Errorf("等待页面加载失败: %v", err)
		return nil, errors.Wrap(err, "等待页面加载失败")
	}

	logrus.Info("页面加载成功，开始等待60秒")
	// 强制等待60秒，让页面完全加载，并每10秒检查页面状态
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		if i%10 == 0 {
			logrus.Infof("等待页面加载: %d/60秒", i+1)
			// 检查页面状态，但不强制退出
			if info, err := pp.Info(); err != nil {
				logrus.Warnf("页面状态检查失败: %v", err)
			} else {
				logrus.Infof("页面状态正常: title=%s, url=%s", info.Title, info.URL)
			}
		}
	}
	logrus.Info("60秒等待完成")

	// 简化检查，不强制要求找到元素
	logrus.Info("尝试查找上传内容区域")
	if _, err := pp.Element("div.upload-content"); err == nil {
		logrus.Info("找到上传内容区域")
	} else {
		logrus.Warnf("未找到上传内容区域: %v", err)
	}

	// 尝试查找并点击"上传图文"按钮，但不强制要求成功
	logrus.Info("尝试查找上传图文按钮")
	if createElems, err := pp.Elements("div.creator-tab"); err == nil {
		for _, elem := range createElems {
			if text, err := elem.Text(); err == nil && (text == "发布笔记" || text == "上传图文") {
				if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
					logrus.Warnf("点击上传图文按钮失败: %v", err)
				} else {
					logrus.Info("成功点击上传图文按钮")
				}
				break
			}
		}
	} else {
		logrus.Warnf("查找上传图文按钮失败: %v", err)
	}

	logrus.Info("创建PublishAction完成，页面将保持打开")
	time.Sleep(2 * time.Second)

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

	// 安全等待上传输入框出现
	uploadInput, err := pp.Element(".upload-input")
	if err != nil {
		return errors.Wrap(err, "未找到上传输入框")
	}

	// 上传多个文件
	err = uploadInput.SetFiles(imagesPaths)
	if err != nil {
		return err
	}

	logrus.Infof("Uploaded %d images", len(imagesPaths))

	// 等待上传完成
	time.Sleep(3 * time.Second)

	return nil
}

func submitPublish(page *rod.Page, title, content string) error {

	// 安全查找标题输入框
	titleElem, err := page.Element("div.d-input input")
	if err != nil {
		return errors.Wrap(err, "未找到标题输入框")
	}
	if err := titleElem.Input(title); err != nil {
		return errors.Wrap(err, "输入标题失败")
	}

	logrus.Info("Input title success")
	time.Sleep(1 * time.Second)

	contentElem, found := getContentElement(page)
	if !found {
		return errors.New("没有找到内容输入框")
	}

	if err := contentElem.Input(content); err != nil {
		return errors.Wrap(err, "输入内容失败")
	}

	logrus.Info("Input content success")
	time.Sleep(1 * time.Second)

	// 安全查找发布按钮
	submitButton, err := page.Element("div.submit div.d-button-content")
	if err != nil {
		return errors.Wrap(err, "未找到发布按钮")
	}
	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击发布按钮失败")
	}

	logrus.Info("Submit button clicked")
	time.Sleep(3 * time.Second)

	return nil
}

// 查找内容输入框 - 使用两种方式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	// 先尝试第一种方式：ql-editor
	if elem, err := page.Element("div.ql-editor"); err == nil {
		logrus.Info("Found ql-editor content input")
		return elem, true
	}

	// 再尝试第二种方式：placeholder方式
	if elem, err := findTextboxByPlaceholder(page); err == nil {
		logrus.Info("Found placeholder content input")
		return elem, true
	}

	logrus.Warn("Content input not found with any method")
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
