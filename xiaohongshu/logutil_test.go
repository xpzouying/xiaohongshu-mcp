package xiaohongshu

import (
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	in := "https://www.xiaohongshu.com/explore/abc?xsec_token=SECRET_TOKEN_VALUE&xsec_source=pc_feed"
	out := RedactURL(in)
	if out == in {
		t.Fatalf("expected redaction, got same string")
	}
	if strings.Contains(out, "SECRET_TOKEN_VALUE") {
		t.Fatalf("token leaked: %s", out)
	}
	if !strings.Contains(out, "xsec_token") {
		t.Fatalf("expected xsec_token key kept: %s", out)
	}
}

func TestRedactToken(t *testing.T) {
	if RedactToken("short") != "***" {
		t.Fatal("short token")
	}
	got := RedactToken("abcdefghijklmnop")
	if strings.Contains(got, "efghijkl") {
		t.Fatalf("middle should be hidden: %s", got)
	}
}
