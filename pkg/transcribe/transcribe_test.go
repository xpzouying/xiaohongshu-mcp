package transcribe

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := tmpDir + "/a.tmp"
	file2 := tmpDir + "/b.tmp"

	require.NoError(t, os.WriteFile(file1, []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("b"), 0o644))

	require.NoError(t, RemoveArtifacts([]string{file1, file2}))
	_, err1 := os.Stat(file1)
	_, err2 := os.Stat(file2)
	require.Error(t, err1)
	require.Error(t, err2)
}

func TestResolveAPIKey(t *testing.T) {
	cases := []struct {
		name          string
		provider      string
		custom        string
		dashEnv       string
		zhipuEnv      string
		bigModelEnv   string
		expectedKey   string
		expectFailure bool
	}{
		{
			name:        "custom key has highest priority",
			provider:    providerDashScope,
			custom:      "custom-key",
			dashEnv:     "dash-env",
			zhipuEnv:    "zhipu-env",
			bigModelEnv: "bigmodel-env",
			expectedKey: "custom-key",
		},
		{
			name:        "dashscope env used",
			provider:    providerDashScope,
			dashEnv:     "dash-env",
			expectedKey: "dash-env",
		},
		{
			name:        "glm env used",
			provider:    providerGLM,
			zhipuEnv:    "zhipu-env",
			bigModelEnv: "bigmodel-env",
			expectedKey: "zhipu-env",
		},
		{
			name:          "glm fallback bigmodel env",
			provider:      providerGLM,
			bigModelEnv:   "bigmodel-env",
			expectedKey:   "bigmodel-env",
			expectFailure: false,
		},
		{
			name:          "no key returns error",
			provider:      providerDashScope,
			expectFailure: true,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("DASHSCOPE_API_KEY", testCase.dashEnv)
			t.Setenv("ZHIPUAI_API_KEY", testCase.zhipuEnv)
			t.Setenv("BIGMODEL_API_KEY", testCase.bigModelEnv)

			key, err := resolveAPIKey(testCase.provider, testCase.custom)
			if testCase.expectFailure {
				require.Error(t, err)
				require.Empty(t, key)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expectedKey, key)
		})
	}
}

func TestResolveGLMModel(t *testing.T) {
	cases := []struct {
		name        string
		provider    string
		custom      string
		glmEnvModel string
		dashEnvMode string
		expectModel string
	}{
		{
			name:        "custom model wins",
			provider:    providerDashScope,
			custom:      "glm-custom",
			glmEnvModel: "glm-env",
			dashEnvMode: "qwen-env",
			expectModel: "glm-custom",
		},
		{
			name:        "dashscope env model used",
			provider:    providerDashScope,
			dashEnvMode: "qwen-env",
			expectModel: "qwen-env",
		},
		{
			name:        "glm env model used",
			provider:    providerGLM,
			glmEnvModel: "glm-env",
			expectModel: "glm-env",
		},
		{
			name:        "dashscope default model",
			provider:    providerDashScope,
			expectModel: defaultDashScopeModel,
		},
		{
			name:        "glm default model",
			provider:    providerGLM,
			expectModel: defaultGLMModel,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("GLM_VIDEO_MODEL", testCase.glmEnvModel)
			t.Setenv("DASHSCOPE_VIDEO_MODEL", testCase.dashEnvMode)
			got := resolveModel(testCase.provider, testCase.custom)
			require.Equal(t, testCase.expectModel, got)
		})
	}
}

func TestResolveProvider(t *testing.T) {
	t.Setenv("VIDEO_TRANSCRIBE_PROVIDER", "")
	got, err := resolveProvider("")
	require.NoError(t, err)
	require.Equal(t, providerDashScope, got)

	got, err = resolveProvider("glm")
	require.NoError(t, err)
	require.Equal(t, providerGLM, got)

	_, err = resolveProvider("unknown")
	require.Error(t, err)
}

func TestBuildProviderPayload_UsesVideoURL(t *testing.T) {
	videoURL := "http://sns-video-bd.xhscdn.com/stream/1/110/259/demo.mp4"
	prompt := "test prompt"

	for _, provider := range []string{providerDashScope, providerGLM} {
		payloadBytes, err := buildProviderPayload(provider, "test-model", videoURL, prompt)
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(payloadBytes, &payload))

		messages, ok := payload["messages"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, messages)
		message, ok := messages[0].(map[string]any)
		require.True(t, ok)
		content, ok := message["content"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, content)
		videoItem, ok := content[0].(map[string]any)
		require.True(t, ok)
		videoObj, ok := videoItem["video_url"].(map[string]any)
		require.True(t, ok)

		gotURL, ok := videoObj["url"].(string)
		require.True(t, ok)
		require.Equal(t, videoURL, gotURL)
	}
}

func TestParseStructuredTranscription(t *testing.T) {
	cases := []struct {
		name               string
		raw                string
		expectTranscript   string
		expectSRTContains  string
		expectParseSuccess bool
	}{
		{
			name:               "plain json",
			raw:                `{"transcript_text":"第一句","srt_text":"1\n00:00:00,000 --> 00:00:04,000\n第一句"}`,
			expectTranscript:   "第一句",
			expectSRTContains:  "00:00:00,000 --> 00:00:04,000",
			expectParseSuccess: true,
		},
		{
			name:               "markdown fenced json",
			raw:                "```json\n{\"transcript\":\"第二句\"}\n```",
			expectTranscript:   "第二句",
			expectParseSuccess: true,
		},
		{
			name:               "invalid json",
			raw:                "这是一段纯文本，不是JSON",
			expectParseSuccess: false,
		},
		{
			name:               "pseudo json with raw newlines",
			raw:                "```json\n{\n  \"transcript_text\": \"第一行\n第二行\",\n  \"srt_text\": \"1\\n00:00:00,000 --> 00:00:04,000\\n第一行\"\n}\n```",
			expectTranscript:   "第一行\n第二行",
			expectSRTContains:  "00:00:00,000 --> 00:00:04,000",
			expectParseSuccess: true,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			transcriptText, subtitleText, ok := parseStructuredTranscription(testCase.raw)
			require.Equal(t, testCase.expectParseSuccess, ok)
			if !ok {
				return
			}
			require.Equal(t, testCase.expectTranscript, transcriptText)
			if testCase.expectSRTContains != "" {
				require.Contains(t, subtitleText, testCase.expectSRTContains)
			}
		})
	}
}

func TestBuildFallbackSRT(t *testing.T) {
	subtitle := BuildFallbackSRT("第一句。\n第二句。")
	require.Contains(t, subtitle, "1\n00:00:00,000 --> 00:00:04,000")
	require.Contains(t, subtitle, "2\n00:00:04,000 --> 00:00:08,000")
	require.Contains(t, subtitle, "第一句。")
	require.Contains(t, subtitle, "第二句。")
}

func TestExtractMessageContent(t *testing.T) {
	content := []any{
		map[string]any{"text": "第一段"},
		"第二段",
	}
	result := extractMessageContent(content)
	require.Contains(t, result, "第一段")
	require.Contains(t, result, "第二段")
}

func TestParseTranscriptionResponse(t *testing.T) {
	mockResponse := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": `{"transcript_text":"你好","srt_text":"1\n00:00:00,000 --> 00:00:04,000\n你好"}`,
				},
			},
		},
	}
	raw, err := json.Marshal(mockResponse)
	require.NoError(t, err)

	transcriptText, subtitleText, err := parseTranscriptionResponse(raw)
	require.NoError(t, err)
	require.Equal(t, "你好", transcriptText)
	require.Contains(t, subtitleText, "00:00:00,000 --> 00:00:04,000")
}

func TestParseTranscriptionResponse_InvalidSRTFallback(t *testing.T) {
	mockResponse := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": `{"transcript_text":"你好世界","srt_text":"1 0.00 -- -- 你好世界"}`,
				},
			},
		},
	}
	raw, err := json.Marshal(mockResponse)
	require.NoError(t, err)

	_, subtitleText, err := parseTranscriptionResponse(raw)
	require.NoError(t, err)
	require.Contains(t, subtitleText, "00:00:00,000 --> 00:00:04,000")
	require.NotContains(t, subtitleText, "0.00 -- --")
}
