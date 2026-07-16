package xiaohongshu

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	xhserrors "github.com/xpzouying/xiaohongshu-mcp/errors"
)

func TestSearch(t *testing.T) {

	t.Skip("SKIP: 测试发布")

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer func() {
		_ = page.Close()
	}()

	action := NewSearchAction(page)

	feeds, err := action.Search(context.Background(), "Kimi")
	require.NoError(t, err)
	require.NotEmpty(t, feeds, "feeds should not be empty")

	fmt.Printf("成功获取到 %d 个 Feed\n", len(feeds))

	for _, feed := range feeds {
		fmt.Printf("Feed ID: %s\n", feed.ID)
		fmt.Printf("Feed Title: %s\n", feed.NoteCard.DisplayTitle)
	}
}

func TestSearchWithFilters(t *testing.T) {
	if os.Getenv("XHS_RUN_NETWORK_TESTS") != "1" {
		t.Skip("skip external network test; set XHS_RUN_NETWORK_TESTS=1 to enable")
	}

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer func() {
		_ = page.Close()
	}()

	action := NewSearchAction(page)

	// 使用新的 FilterOption 结构
	filter := FilterOption{
		NoteType:    "图文",
		PublishTime: "一天内",
	}

	feeds, err := action.Search(context.Background(), "dn432", filter)
	require.NoError(t, err)
	require.NotEmpty(t, feeds, "feeds should not be empty")

	fmt.Printf("成功获取到 %d 个筛选后的 Feed\n", len(feeds))

	for _, feed := range feeds {
		fmt.Printf("Feed ID: %s\n", feed.ID)
		fmt.Printf("Feed Title: %s\n", feed.NoteCard.DisplayTitle)
	}
}

func TestFilterValidation(t *testing.T) {
	// 测试有效的筛选选项转换
	validFilter := FilterOption{
		NoteType:    "图文",
		PublishTime: "一天内",
	}
	internalFilters, err := convertToInternalFilters(validFilter)
	require.NoError(t, err)
	require.Len(t, internalFilters, 2)

	// 验证转换后的内部筛选选项
	for _, filter := range internalFilters {
		err := validateInternalFilterOption(filter)
		require.NoError(t, err)
	}

	// 测试无效的筛选值
	invalidFilter := FilterOption{
		NoteType: "不存在的类型",
	}
	_, err = convertToInternalFilters(invalidFilter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "未找到文本")

	// 测试所有有效的筛选选项
	allFilters := FilterOption{
		SortBy:      "最新",
		NoteType:    "视频",
		PublishTime: "一周内",
		SearchScope: "已关注",
		Location:    "同城",
	}
	internalFilters, err = convertToInternalFilters(allFilters)
	require.NoError(t, err)
	require.Len(t, internalFilters, 5)
}

func TestDefaultFiltersAreSkipped(t *testing.T) {
	filters, err := convertToInternalFilters(FilterOption{
		SortBy: "综合", NoteType: "不限", PublishTime: "不限", SearchScope: "不限", Location: "不限",
	})
	require.NoError(t, err)
	require.Empty(t, filters)

	filters, err = convertToInternalFilters(FilterOption{SortBy: "综合", NoteType: "图文"})
	require.NoError(t, err)
	require.Len(t, filters, 1)
	require.Equal(t, "图文", filters[0].Text)
}

func TestSearchCanceledContextDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	action := &SearchAction{timeout: time.Second}
	require.NotPanics(t, func() {
		_, err := action.Search(ctx, "测试")
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
		var searchErr *SearchError
		require.True(t, stderrors.As(err, &searchErr))
		require.Equal(t, "SEARCH_CANCELED", searchErr.Code)
	})
}

func TestSearchTimeoutIsStructured(t *testing.T) {
	action := (&SearchAction{}).withTimeout(time.Nanosecond)
	time.Sleep(time.Millisecond)

	require.NotPanics(t, func() {
		_, err := action.Search(context.Background(), "测试")
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		var searchErr *SearchError
		require.True(t, stderrors.As(err, &searchErr))
		require.Equal(t, "SEARCH_TIMEOUT", searchErr.Code)
	})
}

func TestSearchRetriesInitialTimeoutOnce(t *testing.T) {
	attempts := 0
	action := &SearchAction{
		timeout:  time.Second,
		navigate: func(context.Context, string) error { return nil },
		reload:   func(context.Context) error { return nil },
		waitResults: func(context.Context, string) (searchSnapshot, error) {
			attempts++
			if attempts == 1 {
				return searchSnapshot{}, &SearchError{Code: "SEARCH_TIMEOUT", Stage: "wait_initial_state", Err: context.DeadlineExceeded}
			}
			return searchSnapshot{Feeds: []Feed{{ID: "retry-ok"}}, Signature: "retry-ok"}, nil
		},
	}

	feeds, err := action.Search(context.Background(), "Kimi")
	require.NoError(t, err)
	require.Equal(t, "retry-ok", feeds[0].ID)
	require.Equal(t, 2, attempts)
}

func TestSearchBoundsInitialAttemptForFastRetry(t *testing.T) {
	var firstAttemptBudget time.Duration
	attempts := 0
	action := &SearchAction{
		timeout:  25 * time.Second,
		navigate: func(context.Context, string) error { return nil },
		reload:   func(context.Context) error { return nil },
		waitResults: func(ctx context.Context, _ string) (searchSnapshot, error) {
			attempts++
			if attempts == 1 {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				firstAttemptBudget = time.Until(deadline)
				return searchSnapshot{}, &SearchError{Code: "SEARCH_TIMEOUT", Stage: "wait_initial_state", Err: context.DeadlineExceeded}
			}
			return searchSnapshot{Feeds: []Feed{{ID: "retry-ok"}}, Signature: "retry-ok"}, nil
		},
	}

	_, err := action.Search(context.Background(), "美食")
	require.NoError(t, err)
	require.LessOrEqual(t, firstAttemptBudget, 10*time.Second)
}

func TestSearchRetriesPrematureEmptyOnce(t *testing.T) {
	attempts := 0
	reloads := 0
	action := &SearchAction{
		timeout:  time.Second,
		navigate: func(context.Context, string) error { return nil },
		reload: func(context.Context) error {
			reloads++
			return nil
		},
		waitResults: func(context.Context, string) (searchSnapshot, error) {
			attempts++
			if attempts == 1 {
				return searchSnapshot{}, xhserrors.ErrNoFeeds
			}
			return searchSnapshot{Feeds: []Feed{{ID: "after-empty"}}, Signature: "after-empty"}, nil
		},
	}

	feeds, err := action.Search(context.Background(), "Kimi")
	require.NoError(t, err)
	require.Equal(t, "after-empty", feeds[0].ID)
	require.Equal(t, 1, reloads)
}

func TestSearchAcceptsExplicitEmptyState(t *testing.T) {
	attempts := 0
	action := &SearchAction{
		timeout:  time.Second,
		navigate: func(context.Context, string) error { return nil },
		reload:   func(context.Context) error { return nil },
		waitResults: func(context.Context, string) (searchSnapshot, error) {
			attempts++
			return searchSnapshot{NoResult: true}, nil
		},
	}

	feeds, err := action.Search(context.Background(), "不存在的关键词")
	require.NoError(t, err)
	require.Empty(t, feeds)
	require.Equal(t, 2, attempts)
}

func TestSearchWaitsForFilteredSignatureChange(t *testing.T) {
	previousSignatures := make([]string, 0, 2)
	action := &SearchAction{
		timeout:  time.Second,
		navigate: func(context.Context, string) error { return nil },
		waitResults: func(_ context.Context, previous string) (searchSnapshot, error) {
			previousSignatures = append(previousSignatures, previous)
			if previous == "" {
				return searchSnapshot{Feeds: []Feed{{ID: "before"}}, Signature: "before"}, nil
			}
			return searchSnapshot{Feeds: []Feed{{ID: "after"}}, Signature: "after"}, nil
		},
		applyFilters: func(context.Context, []internalFilterOption) error { return nil },
	}

	feeds, err := action.Search(context.Background(), "Kimi", FilterOption{NoteType: "图文"})
	require.NoError(t, err)
	require.Equal(t, "after", feeds[0].ID)
	require.Equal(t, []string{"", "before"}, previousSignatures)
}

func TestSearchCancellationDuringWaitDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	action := &SearchAction{
		timeout:  time.Second,
		navigate: func(context.Context, string) error { return nil },
		waitResults: func(ctx context.Context, _ string) (searchSnapshot, error) {
			cancel()
			<-ctx.Done()
			return searchSnapshot{}, ctx.Err()
		},
	}

	require.NotPanics(t, func() {
		_, err := action.Search(ctx, "Kimi")
		require.ErrorIs(t, err, context.Canceled)
	})
}
