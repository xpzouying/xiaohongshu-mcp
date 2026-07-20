package humanize

import (
	"context"
	"time"
)

// Delay 按 action 对应的分布采样一个时延并阻塞等待，期间可被 ctx 取消。
func Delay(ctx context.Context, action Action) {
	dist, ok := defaultProvider.Timing()[action]
	if !ok {
		dist = defaultProvider.Timing()[AfterClick] // 未知动作回退到一个保守默认
	}

	t := time.NewTimer(dist.Sample())
	defer t.Stop()

	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
