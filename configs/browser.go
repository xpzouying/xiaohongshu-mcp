package configs

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

var (
	useHeadless = true

	fingerprintSeed = 0

	proxy = ""

	// userDataDir 持久 Chrome profile 目录；空字符串表示不启用（保持原有每次新建浏览器行为）
	userDataDir = ""
)

func InitHeadless(h bool) {
	useHeadless = h
}

// IsHeadless 是否无头模式。
func IsHeadless() bool {
	return useHeadless
}

func SetFingerprintSeed(s int) {
	fingerprintSeed = s
}

func FingerprintSeed() int {
	return fingerprintSeed
}

// FingerprintSeedFromEnv 从 XHS_FP_SEED 环境变量解析固定 seed。
// 未设或非法返回 0（回退随机）。env 读取集中在配置层，浏览器工厂只收 Option。
func FingerprintSeedFromEnv() int {
	s := os.Getenv("XHS_FP_SEED")
	if s == "" {
		return 0
	}
	seed, err := strconv.Atoi(s)
	if err != nil || seed <= 0 {
		logrus.Warnf("invalid XHS_FP_SEED=%q, ignored (fallback to random seed)", s)
		return 0
	}
	return seed
}

func SetProxy(p string) {
	proxy = p
}

func Proxy() string {
	return proxy
}

// ProxyFromEnv 从 XHS_PROXY 环境变量读取代理地址。env 读取集中在配置层。
func ProxyFromEnv() string {
	return os.Getenv("XHS_PROXY")
}

// SetUserDataDir 设置持久 Chrome profile 目录
func SetUserDataDir(dir string) {
	userDataDir = dir
}

// UserDataDir 获取持久 Chrome profile 目录
func UserDataDir() string {
	return userDataDir
}

// UserDataDirFromEnv 从环境变量 XHS_USER_DATA_DIR 读取持久 profile 路径
func UserDataDirFromEnv() string {
	return os.Getenv("XHS_USER_DATA_DIR")
}
