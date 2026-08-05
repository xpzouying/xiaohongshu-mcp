package browser

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCamouConfig_Deterministic(t *testing.T) {
	a := buildCamouConfig(12345, "zh-CN")
	b := buildCamouConfig(12345, "zh-CN")
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	assert.JSONEq(t, string(ra), string(rb), "同一 seed 应得到同一指纹配置")

	assert.Equal(t, 12345, a["seed"])
	assert.Equal(t, "zh-CN", a["navigator.language"])
	assert.Equal(t, "zh-CN", a["locale"])
	assert.Contains(t, a["navigator.userAgent"], "Firefox/152")
}

func TestBuildCamouConfig_NoSeedOmitsKey(t *testing.T) {
	cfg := buildCamouConfig(0, "zh-CN")
	_, ok := cfg["seed"]
	assert.False(t, ok, "seed<=0 不应写入配置")
}

func TestCamouEnv_ShortConfigSingleChunk(t *testing.T) {
	env, err := camouEnv(map[string]any{"a": "b"})
	require.NoError(t, err)
	require.Len(t, env, 1)
	assert.Contains(t, env, "CAMOU_CONFIG_1")
	assert.JSONEq(t, `{"a":"b"}`, env["CAMOU_CONFIG_1"])
}

func TestCamouEnv_ChunksAndReassembles(t *testing.T) {
	// 构造一份超过单块大小的配置
	big := strings.Repeat("x", camouConfigChunkSize*3+17)
	env, err := camouEnv(map[string]any{"pad": big})
	require.NoError(t, err)
	require.Greater(t, len(env), 1, "应被拆成多块")

	// 按编号顺序拼接应还原出合法 JSON，且 pad 字段完整
	var sb strings.Builder
	for i := 1; i <= len(env); i++ {
		chunk, ok := env[jsonKey(i)]
		require.True(t, ok, "缺少块 %d", i)
		sb.WriteString(chunk)
	}
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(sb.String()), &decoded))
	assert.Equal(t, big, decoded["pad"])
}

func jsonKey(i int) string {
	return "CAMOU_CONFIG_" + strconv.Itoa(i)
}
