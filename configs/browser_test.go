package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFingerprintSeedFromEnv 校验 XHS_FP_SEED 解析：非法/非正数一律回退 0（随机）。
func TestFingerprintSeedFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "未设返回0", env: "", want: 0},
		{name: "合法seed", env: "98759", want: 98759},
		{name: "非数字回退0", env: "abc", want: 0},
		{name: "零回退0", env: "0", want: 0},
		{name: "负数回退0", env: "-5", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XHS_FP_SEED", tt.env)
			assert.Equal(t, tt.want, FingerprintSeedFromEnv())
		})
	}
}

func TestProxyFromEnv(t *testing.T) {
	t.Run("未设为空", func(t *testing.T) {
		t.Setenv("XHS_PROXY", "")
		assert.Equal(t, "", ProxyFromEnv())
	})
	t.Run("读取代理地址", func(t *testing.T) {
		t.Setenv("XHS_PROXY", "socks5://127.0.0.1:1080")
		assert.Equal(t, "socks5://127.0.0.1:1080", ProxyFromEnv())
	})
}

// TestSetGet 校验 seed/proxy 的存取；用后恢复全局状态避免污染其他测试。
func TestSetGet(t *testing.T) {
	origSeed, origProxy := FingerprintSeed(), Proxy()
	t.Cleanup(func() { SetFingerprintSeed(origSeed); SetProxy(origProxy) })

	SetFingerprintSeed(12345)
	assert.Equal(t, 12345, FingerprintSeed())

	SetProxy("http://proxy:3128")
	assert.Equal(t, "http://proxy:3128", Proxy())
}
