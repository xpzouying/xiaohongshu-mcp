package xiaohongshu

import (
	"context"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

func normalizeTopicName(name string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(name), "#"))
}

func matchesTopicCandidate(label, name string) bool {
	// 阅读量等信息可能在后续行；只接受名称完全一致的候选。
	firstLine := strings.SplitN(strings.TrimSpace(label), "\n", 2)[0]
	firstLine = strings.TrimSpace(strings.TrimSuffix(firstLine, "新建话题"))
	return normalizeTopicName(firstLine) == name
}

func editorTopicCount(editor *rod.Element, name string) (int, error) {
	// 只接受明确的话题类型标记，普通链接和只读节点都不能证明插入成功。
	nodes, err := editor.Elements(`[data-type="topic"]`)
	if err != nil {
		return 0, errors.Wrap(err, "读取编辑器话题节点失败")
	}
	count := 0
	for _, node := range nodes {
		children, err := node.Elements(`[data-type="topic"]`)
		if err != nil {
			return 0, err
		}
		if len(children) != 0 {
			continue
		}
		text, err := node.Text()
		if err != nil {
			return 0, err
		}
		text = strings.TrimSuffix(strings.TrimSpace(text), "#")
		text = strings.TrimSuffix(text, "[话题]")
		if normalizeTopicName(text) == name {
			count++
		}
	}
	return count, nil
}

func waitEditorTopic(ctx context.Context, editor *rod.Element, name string, before int) error {
	editor = editor.Context(ctx).Timeout(5 * time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := editorTopicCount(editor, name)
		if err != nil {
			return err
		}
		if count > before {
			return nil
		}
		if err := waitTopicPoll(ctx); err != nil {
			return err
		}
	}
	return errors.Errorf("话题[%s]点击后未生成可确认的话题节点，已停止发布", name)
}

func waitTopicPoll(ctx context.Context) error {
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
