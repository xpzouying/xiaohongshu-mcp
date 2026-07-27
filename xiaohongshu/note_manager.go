package xiaohongshu

import (
	"context"
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
	// 找到所有删除按钮，每个按钮对应一条笔记
	buttons, err := page.Elements(`span.control.data-del`)
	if err != nil {
		return nil, fmt.Errorf("查找笔记列表失败: %w", err)
	}

	if len(buttons) == 0 {
		logrus.Warn("笔记列表为空")
		return nil, nil
	}

	notes := make([]NoteSummary, 0, len(buttons))
	for idx, btn := range buttons {
		note := NoteSummary{Index: idx + 1}

		// 向上找到最近的笔记卡片容器
		card, err := findNoteCard(btn)
		if err != nil {
			logrus.Warnf("第 %d 条笔记未找到卡片容器: %v", idx+1, err)
			notes = append(notes, note)
			continue
		}

		// 提取 note_id（优先从 data 属性读取）
		for _, attr := range []string{"data-note-id", "data-id", "data-noteid"} {
			if val, err := card.Attribute(attr); err == nil && val != nil && *val != "" {
				note.NoteID = *val
				break
			}
		}
		// 若 data 属性中没有，尝试从 data-impression 中解析
		if note.NoteID == "" {
			if impression, err := card.Attribute("data-impression"); err == nil && impression != nil {
				note.NoteID = extractNoteIDFromImpression(*impression)
			}
		}

		// 提取标题
		note.Title = elemText(card, ".title")

		// 提取状态
		note.Status = elemText(card, ".permission_msg")

		// 提取更新时间
		note.UpdatedAt = elemText(card, ".time")

		// 提取链接
		if link, err := card.Element("a[href]"); err == nil && link != nil {
			if href, err := link.Attribute("href"); err == nil && href != nil {
				note.URL = *href
			}
		}

		notes = append(notes, note)
	}

	return notes, nil
}

// findNoteCard 从删除按钮向上查找最近的笔记卡片容器
// go-rod 没有原生的 closest API，通过「打标记 → 定位 → 清除标记」的方式实现
func findNoteCard(btn *rod.Element) (*rod.Element, error) {
	return findNoteCardByJS(btn)
}

// findNoteCardByJS 通过 JS 找到笔记卡片，返回对应的 rod.Element
// go-rod 的 Eval 不传参数，JS 中用 this 引用当前元素
func findNoteCardByJS(btn *rod.Element) (*rod.Element, error) {
	// 给卡片元素打上临时标记，再用 go-rod 定位
	const marker = "__rod_note_card__"
	_, err := btn.Eval(fmt.Sprintf(`() => {
		const card = this.closest('.note') || this.closest('[data-note-id]') || this.closest('[data-id]') || this.closest('[data-noteid]');
		if (card) { card.setAttribute('%s', '1'); }
		return card !== null;
	}`, marker))
	if err != nil {
		return nil, fmt.Errorf("标记笔记卡片失败: %w", err)
	}

	card, err := btn.Page().Element(fmt.Sprintf("[%s]", marker))
	if err != nil {
		return nil, fmt.Errorf("定位笔记卡片失败: %w", err)
	}

	// 清除临时标记
	_, _ = card.Eval(fmt.Sprintf(`() => this.removeAttribute('%s')`, marker))

	return card, nil
}

// elemText 在指定容器内查找子元素并返回其文本，找不到时返回空字符串
func elemText(parent *rod.Element, selector string) string {
	el, err := parent.Element(selector)
	if err != nil || el == nil {
		return ""
	}
	text, err := el.Text()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

// extractNoteIDFromImpression 从 data-impression 属性值中解析 noteId
func extractNoteIDFromImpression(impression string) string {
	const prefix = `noteId":"`
	idx := strings.Index(impression, prefix)
	if idx == -1 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(impression[start:], `"`)
	if end == -1 {
		return ""
	}
	return impression[start : start+end]
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
