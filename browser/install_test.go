package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCamoufoxPath_AppBundle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip(".app 布局仅在 macOS 解析")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "Camoufox.app", "Contents", "MacOS", "camoufox")
	require.NoError(t, os.MkdirAll(filepath.Dir(exe), 0o755))
	require.NoError(t, os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755))

	// 传 .app 包路径
	got, err := normalizeCamoufoxPath(filepath.Join(dir, "Camoufox.app"))
	require.NoError(t, err)
	assert.Equal(t, exe, got)

	// 传上层目录
	got, err = normalizeCamoufoxPath(dir)
	require.NoError(t, err)
	assert.Equal(t, exe, got)

	// 传可执行文件本身
	got, err = normalizeCamoufoxPath(exe)
	require.NoError(t, err)
	assert.Equal(t, exe, got)
}

func TestNormalizeCamoufoxPath_RejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	_, err := normalizeCamoufoxPath(filepath.Join(dir, "nonexistent"))
	assert.Error(t, err)

	empty := filepath.Join(dir, "emptydir")
	require.NoError(t, os.MkdirAll(empty, 0o755))
	_, err = normalizeCamoufoxPath(empty)
	assert.Error(t, err)
}

func TestCamoufoxExeInDir_NotExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows 不校验可执行位")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "Camoufox.app", "Contents", "MacOS", "camoufox")
	require.NoError(t, os.MkdirAll(filepath.Dir(exe), 0o755))
	require.NoError(t, os.WriteFile(exe, []byte("x"), 0o644)) // 无可执行位
	assert.Empty(t, camoufoxExeInDir(dir))
}
