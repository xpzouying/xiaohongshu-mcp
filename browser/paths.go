package browser

import "os/exec"

// lookPath 在 PATH 中查找可执行文件，抽成变量便于测试替换。
func lookPath(name string) (string, error) {
	return exec.LookPath(name)
}
