package configs

// HeadlessMode 浏览器 headless 模式
type HeadlessMode string

const (
	HeadlessOff HeadlessMode = "false" // 有窗口（调试/登录用）
	HeadlessOld HeadlessMode = "true"  // 旧 headless（易被反爬检测）
	HeadlessNew HeadlessMode = "new"   // 新 headless（Chrome 112+，推荐）
)

var (
	headlessMode HeadlessMode = HeadlessNew
	binPath                   = ""
)

// InitHeadlessMode 设置 headless 模式（"new"/"true"/"false"）
func InitHeadlessMode(m string) {
	headlessMode = HeadlessMode(m)
}

// GetHeadlessMode 获取当前 headless 模式
func GetHeadlessMode() HeadlessMode {
	return headlessMode
}

func SetBinPath(b string) {
	binPath = b
}

func GetBinPath() string {
	return binPath
}
