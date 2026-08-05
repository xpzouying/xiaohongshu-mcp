package browser

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// camouConfigChunkSize CAMOU_CONFIG 单块长度，与 camoufox pythonlib 的
// update/launch_options 分块策略一致，避免超长环境变量在某些平台被截断。
const camouConfigChunkSize = 2048

// buildCamouConfig 由固定 seed 推导一份确定性的 Camoufox 指纹配置。
//
// Camoufox 在 C++ 层直接消费 navigator.* / screen.* 等键做指纹注入，
// 无需 go-rod 时代的 stealth JS 注入，也无需事后用 CDP 覆盖 UA/Client Hints。
// seed 固定 → 同一账号每次启动得到同一份指纹画像（与 cookies.json 里的 seed 绑定）。
func buildCamouConfig(seed int, language string) map[string]any {
	uaOS, platform := hostUAProfile()

	cfg := map[string]any{
		// 语言 / 区域：只读小红书内容，固定 zh-CN
		"navigator.language":  language,
		"navigator.languages": []string{language, primaryLangOf(language)},
		"locale":              language,

		// 平台指纹
		"navigator.userAgent": firefoxUserAgent(uaOS),
		"navigator.platform":  platform,

		// 屏幕：常见桌面分辨率，避免无头默认的怪异尺寸
		"screen.width":       1920,
		"screen.height":      1080,
		"screen.availWidth":  1920,
		"screen.availHeight": 1040,
		"window.outerWidth":  1920,
		"window.outerHeight": 1040,
	}

	// seed>0 时把种子也作为配置键传入，Camoufox 据此稳定派生 canvas/webgl 噪声。
	if seed > 0 {
		cfg["seed"] = seed
	}
	return cfg
}

// camouEnv 把指纹配置序列化并按 CAMOU_CONFIG_<n> 分块，返回注入子进程的环境变量。
// 这是 Camoufox 读取配置的唯一通道（C++ 侧拼接所有 CAMOU_CONFIG_* 后再解析）。
func camouEnv(cfg map[string]any) (map[string]string, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal camou config failed: %w", err)
	}
	env := map[string]string{}
	s := string(raw)
	if len(s) <= camouConfigChunkSize {
		env["CAMOU_CONFIG_1"] = s
		return env, nil
	}
	for i, n := 0, 1; i < len(s); i, n = i+camouConfigChunkSize, n+1 {
		end := i + camouConfigChunkSize
		if end > len(s) {
			end = len(s)
		}
		env[fmt.Sprintf("CAMOU_CONFIG_%d", n)] = s[i:end]
	}
	return env, nil
}

// hostUAProfile 返回与宿主机一致的 UA OS 标识与 navigator.platform 取值。
func hostUAProfile() (uaOS string, platform string) {
	switch runtime.GOOS {
	case "darwin":
		return "macos", "MacIntel"
	case "windows":
		return "windows", "Win32"
	default:
		return "linux", "Linux x86_64"
	}
}

// firefoxUserAgent 生成与当前 Camoufox（Firefox 152）主版本一致的 UA 字符串。
// 主版本取自 CamoufoxVersion 常量，避免 UA 与真实内核错位形成指纹矛盾。
func firefoxUserAgent(uaOS string) string {
	const ffMajor = "152"
	switch uaOS {
	case "macos":
		return fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:%s.0) Gecko/20100101 Firefox/%s.0", ffMajor, ffMajor)
	case "windows":
		return fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:%s.0) Gecko/20100101 Firefox/%s.0", ffMajor, ffMajor)
	default:
		return fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64; rv:%s.0) Gecko/20100101 Firefox/%s.0", ffMajor, ffMajor)
	}
}

func primaryLangOf(language string) string {
	for i := 0; i < len(language); i++ {
		if language[i] == '-' {
			return language[:i]
		}
	}
	return language
}
