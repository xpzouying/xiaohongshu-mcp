package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 只验证子进程 JSON 协议，不需要安装 OCR 模型或访问网络。
func TestImageTextProcessProtocol(t *testing.T) {
	python := os.Getenv("XHS_OCR_TEST_PYTHON")
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil {
			t.Skip("设置 XHS_OCR_TEST_PYTHON 可运行子进程协议测试")
		}
	}
	for _, tc := range []struct {
		name   string
		script string
		want   string
	}{
		{"成功", "import json,sys\nr=json.load(sys.stdin)\nassert len(r['urls']) == 1\njson.dump({'images':[{'text':'HashMap','confidence':0.98}]},sys.stdout)", ""},
		{"无效JSON", "print('not json')", "无效结果"},
		{"引擎失败", "print('{\"error\":\"missing OCR dependency\"}')", "missing OCR dependency"},
		{"非零退出", "import sys\nprint('private diagnostic',file=sys.stderr)\nsys.exit(2)", "启动或执行失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := filepath.Join(t.TempDir(), "helper.py")
			require.NoError(t, os.WriteFile(script, []byte(tc.script), 0600))
			t.Setenv("XHS_OCR_PYTHON", python)
			t.Setenv("XHS_OCR_SCRIPT", script)
			images, err := runLocalOCR(context.Background(), []string{"https://a.xhscdn.com/1"})
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
				require.NotContains(t, err.Error(), "private diagnostic")
				return
			}
			require.NoError(t, err)
			require.Len(t, images, 1)
			require.Equal(t, "HashMap", images[0].Text)
		})
	}
	t.Run("取消", func(t *testing.T) {
		script := filepath.Join(t.TempDir(), "helper.py")
		require.NoError(t, os.WriteFile(script, []byte("import time\ntime.sleep(30)"), 0600))
		t.Setenv("XHS_OCR_PYTHON", python)
		t.Setenv("XHS_OCR_SCRIPT", script)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := runLocalOCR(ctx, []string{"https://a.xhscdn.com/1"})
		require.ErrorContains(t, err, "取消或超时")
	})
}
