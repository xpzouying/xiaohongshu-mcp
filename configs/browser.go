package configs

import "time"

var (
	useHeadless = true

	binPath = ""

	windowWidth  = 1920
	windowHeight = 1080

	minOpInterval = 3 * time.Second
)

func InitHeadless(h bool) {
	useHeadless = h
}

func IsHeadless() bool {
	return useHeadless
}

func SetBinPath(b string) {
	binPath = b
}

func GetBinPath() string {
	return binPath
}

func GetWindowSize() (int, int) {
	return windowWidth, windowHeight
}

func SetWindowSize(w, h int) {
	windowWidth = w
	windowHeight = h
}

func GetMinOpInterval() time.Duration {
	return minOpInterval
}

func SetMinOpInterval(d time.Duration) {
	if d < time.Second {
		d = time.Second
	}
	minOpInterval = d
}
