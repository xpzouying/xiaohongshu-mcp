package xiaohongshu

import (
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// inputTextToEditorSafe 安全地向富文本编辑器输入文本（包括换行）
// 按 \n 拆分文本，逐行使用 Input 输入，行间模拟 Enter 键实现换行，避免 JS 注入
func inputTextToEditorSafe(page *rod.Page, elem *rod.Element, text string) error {
	// 安全检查：确保 elem 不为 nil
	if elem == nil {
		return errors.New("编辑器元素为 nil")
	}

	// 检查元素是否有效
	visible, err := elem.Visible()
	if err != nil || !visible {
		return errors.New("编辑器元素不可见或无效")
	}

	// 聚焦到编辑器
	if err := elem.Focus(); err != nil {
		return errors.Wrap(err, "聚焦到编辑器失败")
	}

	// 按换行符拆分文本，逐行输入，行间用 Enter 键模拟换行
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			if err := elem.Input(line); err != nil {
				return errors.Wrapf(err, "输入第%d行正文失败", i+1)
			}
			time.Sleep(100 * time.Millisecond)
		}

		// 除最后一行外，按 Enter 产生换行
		if i < len(lines)-1 {
			ka, err := elem.KeyActions()
			if err != nil {
				return errors.Wrap(err, "创建键盘操作失败")
			}
			if err := ka.Press(input.Enter).Do(); err != nil {
				return errors.Wrap(err, "按下回车键失败")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 给编辑器时间处理输入
	time.Sleep(500 * time.Millisecond)

	// 触发 input 事件确保内容被识别
	if _, err := elem.Eval(`() => {
		try {
			this.dispatchEvent(new Event('input', { bubbles: true }));
			this.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		} catch(e) {
			console.error('trigger events error:', e);
			return false;
		}
	}()`); err != nil {
		logrus.Warn("触发 input 事件失败，但内容可能已输入")
	}

	time.Sleep(300 * time.Millisecond) // 等待内容渲染和验证
	return nil
}
