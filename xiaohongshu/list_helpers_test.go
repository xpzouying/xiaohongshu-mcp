package xiaohongshu

import "testing"

func TestParseListCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  string
		want    int
		wantErr bool
	}{
		{name: "empty cursor", cursor: "", want: 0},
		{name: "zero cursor", cursor: "0", want: 0},
		{name: "positive cursor", cursor: "30", want: 30},
		{name: "negative cursor", cursor: "-1", wantErr: true},
		{name: "invalid cursor", cursor: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseListCursor(tt.cursor)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseListCursor(%q) expected error", tt.cursor)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseListCursor(%q) unexpected error: %v", tt.cursor, err)
			}

			if got != tt.want {
				t.Fatalf("parseListCursor(%q) = %d, want %d", tt.cursor, got, tt.want)
			}
		})
	}
}

func TestExtractFeedID(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{
			name: "plain explore url",
			link: "https://www.xiaohongshu.com/explore/69c6852a000000001a02cd0a",
			want: "69c6852a000000001a02cd0a",
		},
		{
			name: "url with query",
			link: "https://www.xiaohongshu.com/explore/69c90e93000000001d019c15?xsec_token=abc&xsec_source=pc_feed",
			want: "69c90e93000000001d019c15",
		},
		{
			name: "url with fragment",
			link: "https://www.xiaohongshu.com/explore/69c90e93000000001d019c15#comments",
			want: "69c90e93000000001d019c15",
		},
		{
			name: "bare id",
			link: "69c90e93000000001d019c15",
			want: "69c90e93000000001d019c15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFeedID(tt.link)
			if got != tt.want {
				t.Fatalf("extractFeedID(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}
