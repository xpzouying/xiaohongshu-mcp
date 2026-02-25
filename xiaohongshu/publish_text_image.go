package xiaohongshu

import (
	"log/slog"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// generateTextImage 点击文字配图按钮，输入文字，生成图片，点击下一步
func generateTextImage(page *rod.Page, textContent string) error {
	// 步骤1: 找到并点击"文字配图"按钮
	if err := clickTextImageButton(page); err != nil {
		return errors.Wrap(err, "点击文字配图按钮失败")
	}
	slog.Info("已点击文字配图按钮")
	time.Sleep(2 * time.Second)

	// 步骤2: 在文字输入区域输入内容
	if err := inputTextImageContent(page, textContent); err != nil {
		return errors.Wrap(err, "输入文字配图内容失败")
	}
	slog.Info("已输入文字配图内容", "length", len(textContent))
	time.Sleep(500 * time.Millisecond)

	// 步骤3: 点击"生成图片"按钮
	if err := clickGenerateImageButton(page); err != nil {
		return errors.Wrap(err, "点击生成图片按钮失败")
	}
	slog.Info("已点击生成图片按钮")

	// 步骤4: 等待图片生成完成
	if err := waitForImageGeneration(page); err != nil {
		return errors.Wrap(err, "等待图片生成超时")
	}
	slog.Info("图片生成完成")

	// 步骤5: 点击"下一步"按钮进入标准发布页
	if err := clickNextStepButton(page); err != nil {
		return errors.Wrap(err, "点击下一步按钮失败")
	}
	slog.Info("已点击下一步，进入发布页面")
	time.Sleep(2 * time.Second)

	return nil
}

// clickTextImageButton 点击"文字配图"按钮
// 已确认的 DOM 结构:
//
//	<div class="image-upload-buttons">
//	  <button class="d-button ...">上传图片</button>
//	  <button class="d-button ...">
//	    <span class="text2image-content">
//	      <svg .../>
//	      <span class="text2image-text">文字配图</span>
//	    </span>
//	  </button>
//	</div>
func clickTextImageButton(page *rod.Page) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		// 使用精确的 CSS 选择器定位"文字配图"按钮
		el, err := page.Element("span.text2image-text")
		if err == nil && el != nil {
			if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
				logrus.Warnf("点击文字配图按钮失败: %v，重试", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("未找到文字配图按钮(span.text2image-text)")
}

// inputTextImageContent 在文字配图的文本输入区域输入内容
// 已确认的 DOM 结构: <div class="tiptap ProseMirror" contenteditable="true">
func inputTextImageContent(page *rod.Page, text string) error {
	// 优先查找 contenteditable 区域（文字配图使用 tiptap/ProseMirror 编辑器）
	inputElem, err := page.Timeout(10 * time.Second).Element("[contenteditable='true']")
	if err != nil {
		// 降级尝试 textarea
		inputElem, err = page.Timeout(5 * time.Second).Element("textarea")
		if err != nil {
			return errors.New("未找到文字配图的文本输入区域")
		}
	}

	if err := inputTextToEditor(inputElem, text); err != nil {
		return errors.Wrap(err, "输入文字内容失败")
	}

	return nil
}

// clickGenerateImageButton 点击"生成图片"按钮
func clickGenerateImageButton(page *rod.Page) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		el, err := page.ElementR("button, span", "生成图片")
		if err == nil && el != nil {
			if err := el.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return nil
			}
			logrus.Warnf("点击生成图片按钮失败: %v，重试", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("未找到生成图片按钮")
}

// waitForImageGeneration 等待图片生成完成
// 通过检测"下一步"按钮是否可用来判断图片是否生成完成
func waitForImageGeneration(page *rod.Page) error {
	maxWait := 60 * time.Second
	interval := 1 * time.Second
	start := time.Now()

	for time.Since(start) < maxWait {
		el, err := page.ElementR("button, span", "下一步")
		if err == nil && el != nil {
			visible, _ := el.Visible()
			if visible {
				disabled, _ := el.Attribute("disabled")
				if disabled == nil {
					slog.Info("检测到下一步按钮可用，图片生成完成")
					return nil
				}
			}
		}

		time.Sleep(interval)
	}

	return errors.New("等待图片生成超时(60s)")
}

// clickNextStepButton 点击"下一步"按钮
func clickNextStepButton(page *rod.Page) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		el, err := page.ElementR("button, span", "下一步")
		if err == nil && el != nil {
			if err := el.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return nil
			}
			logrus.Warnf("点击下一步按钮失败: %v，重试", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("未找到下一步按钮")
}
