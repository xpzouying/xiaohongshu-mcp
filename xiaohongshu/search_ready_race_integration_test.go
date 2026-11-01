//go:build integration

package xiaohongshu

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
)

// 只延迟浏览器返回快照，不替换等待逻辑或伪造页面读取结果。
type delayedSearchSnapshotClient struct {
	rod.CDPClient
	oldState            string
	oldResponseConsumed <-chan struct{}
	snapshotPaused      chan struct{}
	releaseSnapshot     chan struct{}
	freshRead           chan struct{}
	delayed             atomic.Bool
	reread              atomic.Bool
}

func (c *delayedSearchSnapshotClient) Call(ctx context.Context, sessionID, method string, params interface{}) ([]byte, error) {
	if method != "Runtime.callFunctionOn" {
		return c.CDPClient.Call(ctx, sessionID, method, params)
	}
	select {
	case <-c.oldResponseConsumed:
	default:
		return c.CDPClient.Call(ctx, sessionID, method, params)
	}
	if !c.delayed.CompareAndSwap(false, true) {
		if c.reread.CompareAndSwap(false, true) {
			close(c.freshRead)
		}
		return c.CDPClient.Call(ctx, sessionID, method, params)
	}
	setState := func(state string) error {
		feeds := "[]"
		if state == "success" {
			feeds = `[{id: 'old-result', modelType: 'note'}]`
		}
		_, err := c.CDPClient.Call(ctx, sessionID, "Runtime.evaluate", proto.RuntimeEvaluate{
			Expression: "(() => {const s = window.__INITIAL_STATE__.search; s.state._value = '" + state + "'; s.feeds._value = " + feeds + ";})()",
		})
		return err
	}
	if err := setState(c.oldState); err != nil {
		return nil, err
	}
	result, err := c.CDPClient.Call(ctx, sessionID, method, params)
	if err != nil {
		return nil, err
	}
	if err := setState("loading"); err != nil {
		return nil, err
	}
	close(c.snapshotPaused)
	select {
	case <-c.releaseSnapshot:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSearchReadyRechecksSnapshotAfterRequestReset(t *testing.T) {
	cases := []struct {
		name     string
		response searchResponse
		state    string
	}{
		{"旧空结果", searchResponse{empty: true}, "error"},
		{"旧失败", searchResponse{err: fmt.Errorf("旧搜索失败")}, "error"},
		{"旧非空结果", searchResponse{}, "success"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testSearchReadyRechecksSnapshot(t, tc.response, tc.state)
		})
	}
}

func testSearchReadyRechecksSnapshot(t *testing.T, oldResponse searchResponse, oldState string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	oldConsumed := make(chan struct{})
	client := &delayedSearchSnapshotClient{
		oldState:            oldState,
		oldResponseConsumed: oldConsumed,
		snapshotPaused:      make(chan struct{}),
		releaseSnapshot:     make(chan struct{}),
		freshRead:           make(chan struct{}),
	}
	b := newSearchFixtureBrowser(t, func(inner rod.CDPClient) rod.CDPClient {
		client.CDPClient = inner
		return client
	})
	page := b.MustPage().Context(ctx)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetDocumentContent(`<script>
window.__INITIAL_STATE__ = {search: {
    searchContext: {keyword: 'race'}, feeds: {_value: []}, state: {_value: 'loading'}
}};
</script>`)
	responses := make(chan searchResponse, 1)
	responses <- oldResponse
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for len(responses) != 0 {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
		close(oldConsumed)
	}()
	result := make(chan error, 1)
	go func() { result <- waitSearchReady(page, "race", responses) }()

	select {
	case <-client.snapshotPaused:
	case <-ctx.Done():
		t.Fatal("未到达旧响应对应的页面快照")
	}
	// 页面读取期间新请求开始，通知先入队，再让旧快照返回。
	responses <- searchResponse{}
	close(client.releaseSnapshot)
	select {
	case err := <-result:
		t.Fatalf("新请求仍在 loading，却沿用旧结果结束等待: %v", err)
	case <-client.freshRead:
	case <-ctx.Done():
		t.Fatal("收到新请求通知后没有重新读取页面状态")
	}
	page.MustEval(`() => {
		const s = window.__INITIAL_STATE__.search;
		s.feeds._value = [{id: 'new-result', modelType: 'note'}];
		s.state._value = 'success';
	}`)
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("新搜索数据已就绪，等待仍未结束")
	}
}
