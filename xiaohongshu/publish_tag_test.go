package xiaohongshu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectTagSuggestionReturnsAfterClick(t *testing.T) {
	want := &rod.Element{}
	calls := 0

	got, err := selectTagSuggestion(context.Background(), time.Second, func(context.Context) (*rod.Element, error) {
		calls++
		if calls < 2 {
			return nil, nil
		}
		return want, nil
	}, func(got *rod.Element) error {
		assert.NotNil(t, got)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, got)
	assert.Equal(t, 2, calls)
}

func TestSelectTagSuggestionRetriesDetachedItem(t *testing.T) {
	want := &rod.Element{}
	clicks := 0

	got, err := selectTagSuggestion(context.Background(), time.Second, func(context.Context) (*rod.Element, error) {
		return want, nil
	}, func(*rod.Element) error {
		clicks++
		if clicks == 1 {
			return errors.New("node detached")
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, got)
	assert.Equal(t, 2, clicks)
}

func TestSelectTagSuggestionTimesOutWithoutError(t *testing.T) {
	started := time.Now()

	got, err := selectTagSuggestion(context.Background(), 20*time.Millisecond, func(context.Context) (*rod.Element, error) {
		return nil, nil
	}, func(*rod.Element) error {
		return nil
	})

	require.NoError(t, err)
	assert.False(t, got)
	assert.Less(t, time.Since(started), time.Second)
}

func TestSelectTagSuggestionTreatsLookupDeadlineAsNoMatch(t *testing.T) {
	got, err := selectTagSuggestion(context.Background(), 20*time.Millisecond, func(ctx context.Context) (*rod.Element, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}, func(*rod.Element) error {
		return nil
	})

	require.NoError(t, err)
	assert.False(t, got)
}

func TestSelectTagSuggestionReturnsLookupError(t *testing.T) {
	want := errors.New("lookup failed")

	got, err := selectTagSuggestion(context.Background(), time.Second, func(context.Context) (*rod.Element, error) {
		return nil, want
	}, func(*rod.Element) error {
		return nil
	})

	assert.False(t, got)
	assert.ErrorIs(t, err, want)
}

func TestSelectTagSuggestionHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := selectTagSuggestion(ctx, time.Second, func(context.Context) (*rod.Element, error) {
		return nil, nil
	}, func(*rod.Element) error {
		return nil
	})

	assert.False(t, got)
	assert.ErrorIs(t, err, context.Canceled)
}
