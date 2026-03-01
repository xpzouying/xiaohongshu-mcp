package xiaohongshu

import (
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// inputTextToEditorSafe 安全地向富文本编辑器输入文本（包括换行）
// 按 \n 拆分文本，逐行使用 InsertText 在光标处插入，行间模拟 Enter 键实现换行
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

	// 点击编辑器确保获得焦点和光标
	if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
		// 点击失败时尝试 Focus
		if err := elem.Focus(); err != nil {
			return errors.Wrap(err, "聚焦到编辑器失败")
		}
	}
	time.Sleep(200 * time.Millisecond)

	// 将字面量 \n（两个字符：反斜杠+n）替换为真正的换行符，
	// AI 客户端通过 MCP/JSON 传入的文本可能包含字面量 \n
	text = strings.ReplaceAll(text, `\n`, "\n")

	// 按换行符拆分文本，逐行输入，行间用 Enter 键模拟换行
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			// 使用 InsertText 在光标位置插入文本，不会覆盖已有内容
			if err := page.InsertText(line); err != nil {
				return errors.Wrapf(err, "输入第%d行正文失败", i+1)
			}
			time.Sleep(100 * time.Millisecond)
		}

		// 除最后一行外，按 Enter 产生换行
		if i < len(lines)-1 {
			if err := page.Keyboard.Press(input.Enter); err != nil {
				return errors.Wrap(err, "按下回车键失败")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 正文末尾追加一个换行，与后续标签区分
	if err := page.Keyboard.Press(input.Enter); err != nil {
		return errors.Wrap(err, "正文末尾按下回车键失败")
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
