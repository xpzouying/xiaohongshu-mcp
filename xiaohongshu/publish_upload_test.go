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

func TestAcceptsImage(t *testing.T) {
	cases := []struct {
		name   string
		accept string
		want   bool
	}{
		{"图片扩展名", ".png,.webp", true},
		{"大写扩展名", ".JPG", true},
		{"MIME 通配", "image/*", true},
		{"非图片扩展名", ".xyz,.abc", false},
		{"非图片 MIME", "video/mp4", false},
		{"空值", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, acceptsImage(c.accept))
		})
	}
}

func TestRetryImageUploadInputWaitsForReplacement(t *testing.T) {
	want := &rod.Element{}
	lookups := 0
	sets := 0

	err := retryImageUploadInput(
		context.Background(),
		time.Second,
		func() (*rod.Element, error) {
			lookups++
			if lookups < 3 {
				return nil, nil
			}
			return want, nil
		},
		func(got *rod.Element) error {
			assert.Same(t, want, got)
			sets++
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 3, lookups)
	assert.Equal(t, 1, sets)
}

func TestRetryImageUploadInputReacquiresDetachedElement(t *testing.T) {
	lookups := 0
	sets := 0

	err := retryImageUploadInput(
		context.Background(),
		time.Second,
		func() (*rod.Element, error) {
			lookups++
			return &rod.Element{}, nil
		},
		func(*rod.Element) error {
			sets++
			if sets == 1 {
				return errors.New("node detached")
			}
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 2, lookups)
	assert.Equal(t, 2, sets)
}

func TestRetryImageUploadInputTimesOut(t *testing.T) {
	started := time.Now()
	err := retryImageUploadInput(
		context.Background(),
		20*time.Millisecond,
		func() (*rod.Element, error) { return nil, nil },
		func(*rod.Element) error { return nil },
	)

	assert.ErrorContains(t, err, "等待图片上传输入框超时")
	assert.Less(t, time.Since(started), time.Second)
}

func TestRetryImageUploadInputHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := retryImageUploadInput(
		ctx,
		time.Hour,
		func() (*rod.Element, error) { return nil, nil },
		func(*rod.Element) error { return nil },
	)

	assert.ErrorIs(t, err, context.Canceled)
}
