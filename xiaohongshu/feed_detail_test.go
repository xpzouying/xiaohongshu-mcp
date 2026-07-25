package xiaohongshu

import (
	"context"
	"errors"
	"testing"

	"github.com/avast/retry-go/v4"
)

func TestIsPermanentAccessError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error is retryable",
			err:  nil,
			want: false,
		},
		{
			name: "deleted note is permanent",
			err:  errors.New("笔记不可访问: 该笔记已被删除"),
			want: true,
		},
		{
			name: "private note is permanent",
			err:  errors.New("笔记不可访问: 私密笔记"),
			want: true,
		},
		{
			name: "user restriction is permanent",
			err:  errors.New("笔记不可访问: 因用户设置，你无法查看"),
			want: true,
		},
		{
			name: "risk control page is transient",
			err:  errors.New("笔记不可访问: Sorry, This Page Isn't Available Right Now."),
			want: false,
		},
		{
			name: "unknown access error is transient",
			err:  errors.New("笔记不可访问: some unknown error"),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isPermanentAccessError(tt.err); got != tt.want {
				t.Fatalf("isPermanentAccessError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryFeedDetailNavigationRetriesTransientErrors(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := retryFeedDetailNavigation(
		context.Background(),
		func() error {
			attempts++
			if attempts < 3 {
				return errors.New("笔记不可访问: Sorry, This Page Isn't Available Right Now.")
			}
			return nil
		},
		retry.Delay(0),
		retry.DelayType(retry.FixedDelay),
	)

	if err != nil {
		t.Fatalf("retryFeedDetailNavigation() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryFeedDetailNavigationStopsOnPermanentError(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := retryFeedDetailNavigation(
		context.Background(),
		func() error {
			attempts++
			return errors.New("笔记不可访问: 该笔记已被删除")
		},
		retry.Delay(0),
		retry.DelayType(retry.FixedDelay),
	)

	if err == nil {
		t.Fatal("retryFeedDetailNavigation() error = nil, want permanent access error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
