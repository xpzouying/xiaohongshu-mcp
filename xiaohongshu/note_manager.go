package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

type NoteManagerAction struct {
	page *rod.Page
}

type NoteSummary struct {
	Index     int    `json:"index"`
	NoteID    string `json:"note_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	URL       string `json:"url,omitempty"`
}

type DeleteNoteOptions struct {
	Keyword string
	NoteID  string
	Title   string
	Index   int
}

const (
	urlOfNoteManager = "https://creator.xiaohongshu.com/new/note-manager?source=official"
)

func NewNoteManagerAction(page *rod.Page) (*NoteManagerAction, error) {
	pp := page.Timeout(300 * time.Second)

	// 使用更稳健的导航和等待策略
	if err := pp.Navigate(urlOfNoteManager); err != nil {
		return nil, fmt.Errorf("导航到笔记管理页面失败: %w", err)
	}

	if err := pp.WaitLoad(); err != nil {
		logrus.Warnf("等待笔记管理页面加载出现问题: %v，继续尝试", err)
	}
	time.Sleep(1 * time.Second)

	if err := pp.WaitDOMStable(time.Second, 0.1); err != nil {
		logrus.Warnf("等待笔记管理页面 DOM 稳定出现问题: %v，继续尝试", err)
	}
	time.Sleep(1 * time.Second)

	if err := waitNoteManagerReady(pp); err != nil {
		logrus.Warnf("等待笔记管理页面可交互失败: %v", err)
	}
	time.Sleep(1 * time.Second)

	return &NoteManagerAction{page: pp}, nil
}

func DeleteNoteAction(page *rod.Page) *NoteManagerAction {
	action, err := NewNoteManagerAction(page)
	if err != nil {
		logrus.Warnf("初始化笔记管理页面失败: %v", err)
		return &NoteManagerAction{page: page.Timeout(300 * time.Second)}
	}
	return action
}

func (n *NoteManagerAction) ListNotes(ctx context.Context, keyword string) ([]NoteSummary, error) {
	page := n.page.Context(ctx)

	if keyword != "" {
		if err := applyNoteSearch(page, keyword); err != nil {
			return nil, err
		}
	}

	time.Sleep(1 * time.Second)

	notes, err := readNoteSummaries(page)
	if err != nil {
		return nil, err
	}

	return notes, nil
}

func (n *NoteManagerAction) DeleteNote(ctx context.Context, opts DeleteNoteOptions) (NoteSummary, []NoteSummary, error) {
	page := n.page.Context(ctx)

	notes, err := n.ListNotes(ctx, opts.Keyword)
	if err != nil {
		return NoteSummary{}, nil, err
	}

	targetIndex, target, err := pickTargetNote(notes, opts)
	if err != nil {
		return NoteSummary{}, notes, err
	}

	if err := clickDeleteButton(page, targetIndex); err != nil {
		return NoteSummary{}, notes, err
	}

	if err := confirmNoteDelete(page); err != nil {
		return NoteSummary{}, notes, err
	}

	page.MustWaitStable()
	time.Sleep(1 * time.Second)

	return target, notes, nil
}

func applyNoteSearch(page *rod.Page, keyword string) error {
	searchInput, err := findNoteSearchInput(page)
	if err != nil {
		return err
	}

	if err := searchInput.SelectAllText(); err != nil {
		logrus.Warnf("选择搜索框文本失败: %v", err)
	}

	if err := searchInput.Input(keyword); err != nil {
		return fmt.Errorf("输入搜索关键词失败: %w", err)
	}

	ka, err := searchInput.KeyActions()
	if err != nil {
		return fmt.Errorf("创建搜索输入键盘操作失败: %w", err)
	}
	if err := ka.Press(input.Enter).Do(); err != nil {
		return fmt.Errorf("触发搜索失败: %w", err)
	}

	time.Sleep(1 * time.Second)
	page.MustWaitStable()

	return nil
}

func findNoteSearchInput(page *rod.Page) (*rod.Element, error) {
	selectors := []string{
		`input.d-text[placeholder*="搜索已发布笔记"]`,
		`input.d-text[placeholder*="搜索"]`,
		`input.d-text[placeholder*="笔记"]`,
		`input.d-text[placeholder*="标题"]`,
		`input.d-text[placeholder*="关键词"]`,
		`input[placeholder*="搜索"]`,
		`input[type="search"]`,
	}

	for _, selector := range selectors {
		elem, err := page.Element(selector)
		if err == nil && elem != nil {
			return elem, nil
		}
	}

	return nil, fmt.Errorf("未找到笔记管理页面搜索框")
}

func waitNoteManagerReady(page *rod.Page) error {
	selector := `input.d-text[placeholder*="搜索"]`
	elem := page.MustElement(selector)
	if err := elem.WaitVisible(); err != nil {
		return fmt.Errorf("等待笔记管理搜索框出现失败: %w", err)
	}
	return nil
}

func readNoteSummaries(page *rod.Page) ([]NoteSummary, error) {
	result := page.MustEval(`() => {
		const buttons = Array.from(document.querySelectorAll('span.control.data-del'));
		const notes = buttons.map((btn, idx) => {
			const card = btn.closest('.note') || btn.closest('[data-note-id],[data-id],[data-noteid]');
			const getText = (el) => el ? el.textContent.trim() : '';
			let noteId = '';
			let title = '';
			let status = '';
			let updatedAt = '';
			let url = '';

			if (card) {
				noteId = card.getAttribute('data-note-id') || card.getAttribute('data-id') || card.getAttribute('data-noteid') || '';
				if (!noteId) {
					const impression = card.getAttribute('data-impression') || '';
					const match = impression.match(/noteId\"\s*:\"([^\"]+)/);
					if (match && match[1]) {
						noteId = match[1];
					}
				}
				const titleEl = card.querySelector('.title');
				title = getText(titleEl);
				const statusEl = card.querySelector('.permission_msg');
				status = getText(statusEl);
				const timeEl = card.querySelector('.time');
				updatedAt = getText(timeEl);
				const linkEl = card.querySelector('a[href]');
				if (linkEl && linkEl.href) {
					url = linkEl.href;
				}
			}

			return {
				index: idx + 1,
				note_id: noteId,
				title,
				status,
				updated_at: updatedAt,
				url,
			};
		});
		return JSON.stringify(notes);
	}`).String()

	if result == "" {
		return nil, fmt.Errorf("未获取到笔记列表")
	}

	var notes []NoteSummary
	if err := json.Unmarshal([]byte(result), &notes); err != nil {
		return nil, fmt.Errorf("解析笔记列表失败: %w", err)
	}

	if len(notes) == 0 {
		logrus.Warn("笔记列表为空")
	}

	return notes, nil
}

func pickTargetNote(notes []NoteSummary, opts DeleteNoteOptions) (int, NoteSummary, error) {
	if len(notes) == 0 {
		return 0, NoteSummary{}, fmt.Errorf("笔记列表为空，无法删除")
	}

	if opts.NoteID != "" {
		for _, note := range notes {
			if note.NoteID == opts.NoteID {
				return note.Index, note, nil
			}
		}
		return 0, NoteSummary{}, fmt.Errorf("未找到匹配 note_id=%s 的笔记", opts.NoteID)
	}

	if opts.Title != "" {
		for _, note := range notes {
			if strings.Contains(note.Title, opts.Title) {
				return note.Index, note, nil
			}
		}
		return 0, NoteSummary{}, fmt.Errorf("未找到标题包含 %s 的笔记", opts.Title)
	}

	if opts.Index > 0 {
		for _, note := range notes {
			if note.Index == opts.Index {
				return note.Index, note, nil
			}
		}
		return 0, NoteSummary{}, fmt.Errorf("未找到 index=%d 的笔记", opts.Index)
	}

	return 0, NoteSummary{}, fmt.Errorf("缺少删除目标，请提供 note_id、title 或 index")
}

func clickDeleteButton(page *rod.Page, targetIndex int) error {
	buttons, err := page.Elements(`span.control.data-del`)
	if err != nil {
		return fmt.Errorf("查找删除按钮失败: %w", err)
	}

	if targetIndex <= 0 || targetIndex > len(buttons) {
		return fmt.Errorf("删除目标 index=%d 超出范围，总数=%d", targetIndex, len(buttons))
	}

	if err := buttons[targetIndex-1].Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击删除按钮失败: %w", err)
	}

	return nil
}

func confirmNoteDelete(page *rod.Page) error {
	time.Sleep(800 * time.Millisecond)

	_, modal, err := page.Has("div[role='dialog'], div.d-modal, div.modal")
	if err != nil {
		logrus.Warnf("检查删除确认弹窗失败: %v", err)
		return nil
	}
	if modal == nil {
		return nil
	}

	confirmBtn, err := modal.Element("button.confirm-btn")
	if err == nil && confirmBtn != nil {
		if err := confirmBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return fmt.Errorf("点击删除确认按钮失败: %w", err)
		}
		return nil
	}

	buttons, err := modal.Elements("button")
	if err != nil {
		buttons, err = page.Elements("button")
		if err != nil {
			return fmt.Errorf("查找确认删除按钮失败: %w", err)
		}
	}

	for _, btn := range buttons {
		text, err := btn.Text()
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "确定" || text == "确认" || strings.Contains(text, "确定") {
			if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
				return fmt.Errorf("点击删除确认按钮失败: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("未找到删除确认按钮")
}
