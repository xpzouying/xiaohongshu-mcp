package xiaohongshu

import (
	"os"
	"strings"
)

func getBaseURL() string {
	if url := os.Getenv("XHS_BASE_URL"); url != "" {
		return strings.TrimRight(url, "/")
	}
	return "https://www.xiaohongshu.com"
}
