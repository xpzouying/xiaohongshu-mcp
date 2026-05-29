package xiaohongshu

import (
	"encoding/json"
	"testing"
)

func TestParseVideoJSON_H264Stream(t *testing.T) {
	input := `{
		"capa": {"duration": 120},
		"media": {
			"stream": {
				"h264": [{"masterUrl": "http://cdn.example.com/h264.mp4", "width": 720, "height": 1280, "avgBitrate": 2000000}],
				"h265": [{"masterUrl": "http://cdn.example.com/h265.mp4", "width": 1080, "height": 1920, "avgBitrate": 3000000}]
			}
		}
	}`

	result := parseVideoJSON(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Duration != 120 {
		t.Errorf("duration: got %d, want 120", result.Duration)
	}
	if result.URL != "http://cdn.example.com/h264.mp4" {
		t.Errorf("url: got %s, want h264 URL (preferred codec)", result.URL)
	}
	if len(result.Media.Stream["h264"]) != 1 {
		t.Errorf("h264 streams: got %d, want 1", len(result.Media.Stream["h264"]))
	}
	if len(result.Media.Stream["h265"]) != 1 {
		t.Errorf("h265 streams: got %d, want 1", len(result.Media.Stream["h265"]))
	}
}

func TestParseVideoJSON_OnlyH265(t *testing.T) {
	input := `{
		"capa": {"duration": 60},
		"media": {
			"stream": {
				"h265": [{"masterUrl": "http://cdn.example.com/h265.mp4", "width": 1080, "height": 1920, "avgBitrate": 3000000}]
			}
		}
	}`

	result := parseVideoJSON(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.URL != "http://cdn.example.com/h265.mp4" {
		t.Errorf("url: got %s, want h265 URL (fallback)", result.URL)
	}
}

func TestParseVideoJSON_OriginVideoKey(t *testing.T) {
	input := `{
		"consumer": {"originVideoKey": "abc123/video.mp4"},
		"capa": {"duration": 30},
		"media": {"stream": {}}
	}`

	result := parseVideoJSON(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	expected := xhsVideoCDNBase + "abc123/video.mp4"
	if result.URL != expected {
		t.Errorf("url: got %s, want %s", result.URL, expected)
	}
}

func TestParseVideoJSON_BackupURLFallback(t *testing.T) {
	input := `{
		"capa": {"duration": 10},
		"media": {
			"stream": {
				"h264": [{"masterUrl": "", "backupUrls": ["http://backup.example.com/video.mp4"], "width": 720, "height": 1280}]
			}
		}
	}`

	result := parseVideoJSON(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.URL != "http://backup.example.com/video.mp4" {
		t.Errorf("url: got %s, want backup URL", result.URL)
	}
}

func TestParseVideoJSON_EmptyJSON(t *testing.T) {
	result := parseVideoJSON("{}")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.URL != "" {
		t.Errorf("url: got %s, want empty", result.URL)
	}
}

func TestParseVideoJSON_InvalidJSON(t *testing.T) {
	result := parseVideoJSON("not json")
	// Should fallback to parseVideoURLFromRawJSON which also fails, return nil
	if result != nil && result.URL != "" {
		t.Errorf("expected nil or empty url for invalid json, got %s", result.URL)
	}
}

func TestParseVideoURLFromRawJSON(t *testing.T) {
	input := `{
		"capa": {"duration": 45},
		"media": {
			"stream": {
				"h264": [{"masterUrl": "http://cdn.example.com/raw.mp4"}]
			}
		}
	}`

	result := parseVideoURLFromRawJSON(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.URL != "http://cdn.example.com/raw.mp4" {
		t.Errorf("url: got %s, want raw URL", result.URL)
	}
	if result.Duration != 45 {
		t.Errorf("duration: got %d, want 45", result.Duration)
	}
}

func TestPickStreamURL(t *testing.T) {
	tests := []struct {
		name   string
		stream VideoStream
		want   string
	}{
		{"master url", VideoStream{MasterURL: "http://master.mp4"}, "http://master.mp4"},
		{"backup url", VideoStream{BackupURLs: []string{"http://backup.mp4"}}, "http://backup.mp4"},
		{"both", VideoStream{MasterURL: "http://master.mp4", BackupURLs: []string{"http://backup.mp4"}}, "http://master.mp4"},
		{"empty", VideoStream{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickStreamURL(tt.stream)
			if got != tt.want {
				t.Errorf("pickStreamURL: got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestVideoStreamJSON(t *testing.T) {
	// Verify VideoStream can round-trip JSON correctly
	input := `{"masterUrl":"http://example.com/v.mp4","backupUrls":["http://backup.com/v.mp4"],"width":1080,"height":1920,"avgBitrate":3000000}`
	var vs VideoStream
	if err := json.Unmarshal([]byte(input), &vs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if vs.MasterURL != "http://example.com/v.mp4" {
		t.Errorf("MasterURL: got %s", vs.MasterURL)
	}
	if vs.Width != 1080 || vs.Height != 1920 {
		t.Errorf("dimensions: got %dx%d", vs.Width, vs.Height)
	}
	if vs.AvgBitrate != 3000000 {
		t.Errorf("bitrate: got %d", vs.AvgBitrate)
	}
}
