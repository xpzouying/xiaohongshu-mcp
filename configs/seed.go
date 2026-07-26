package configs

import (
	"crypto/rand"
	"math/big"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// maxSeed seed 的取值上限，够大以避免碰撞，又不至于溢出。
const maxSeed = 1 << 31

// ResolveFingerprintSeed 决定本次使用的 seed，优先级：
// 环境变量 XHS_FP_SEED > 会话文件里已存的 > 新生成一个并写回。
//
// 环境变量必须排第一：已经用它钉死画像的部署，不能被文件里的值顶掉。
// 生成后一定要写回，否则每次启动都是新的一套，等于没固定。
func ResolveFingerprintSeed(store cookies.Cookier) int {
	if seed := FingerprintSeedFromEnv(); seed > 0 {
		return seed
	}

	if seed := store.LoadSeed(); seed > 0 {
		return seed
	}

	seed := newSeed()
	if err := store.SaveSeed(seed); err != nil {
		// 存不下也照常启动：退化成本次随机，和改动前的行为一致
		logrus.Warnf("保存会话 seed 失败，本次使用临时值: %v", err)
	}
	return seed
}

// newSeed 生成一个新的 seed。用 crypto/rand 而非 math/rand，
// 避免进程启动时机相近的多个实例撞上同一个值。
func newSeed() int {
	n, err := rand.Int(rand.Reader, big.NewInt(maxSeed))
	if err != nil {
		return 1 // 极罕见：熵源不可用时给个确定值，好过 panic
	}
	return int(n.Int64()) + 1 // +1 保证为正，0 表示"未设置"
}
