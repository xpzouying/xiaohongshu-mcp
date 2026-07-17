package xiaohongshu

import (
	"encoding/json"
	"testing"
)

func TestFeedDetailUnmarshalsVideoMediaStreams(t *testing.T) {
	var detail FeedDetail
	err := json.Unmarshal([]byte(`{"type":"video","video":{"capa":{"duration":12},"media":{"stream":{"h264":[{"masterUrl":"http://video/h264.mp4","backupUrls":["http://backup/h264.mp4"],"format":"mp4","codec":"h264","width":1080,"height":1920,"duration":12000,"quality":1,"default":true}],"h265":[{"masterUrl":"http://video/h265.mp4","codec":"h265"}]}}}}`), &detail)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Video == nil || detail.Video.Capa.Duration != 12 || len(detail.Video.Media.Stream.H264) != 1 || len(detail.Video.Media.Stream.H265) != 1 {
		t.Fatalf("video media was not preserved: %+v", detail.Video)
	}
	stream := detail.Video.Media.Stream.H264[0]
	if stream.MasterURL != "http://video/h264.mp4" || len(stream.BackupURLs) != 1 || stream.Format != "mp4" || stream.Codec != "h264" || stream.Width != 1080 || stream.Height != 1920 || stream.Duration != 12000 || stream.Quality != 1 || !stream.Default {
		t.Fatalf("unexpected H264 stream: %+v", stream)
	}
}
