package xiaohongshu

import "testing"

func TestChooseFilterTagSkipsAlreadyActiveDuplicate(t *testing.T) {
	tags := []filterTagState{
		{Text: "综合", Active: true},
		{Text: "综合", Active: true},
		{Text: "最新", Active: false},
	}

	index, shouldClick, err := chooseFilterTag(tags, "综合")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldClick {
		t.Fatalf("expected no click for already active tag")
	}
	if index != 0 {
		t.Fatalf("expected active index 0, got %d", index)
	}
}

func TestChooseFilterTagReturnsInactiveMatchWhenTargetNotActive(t *testing.T) {
	tags := []filterTagState{
		{Text: "不限", Active: true},
		{Text: "视频", Active: false, Display: "block"},
		{Text: "视频", Active: false, Display: "flex"},
		{Text: "图文", Active: false},
	}

	index, shouldClick, err := chooseFilterTag(tags, "图文")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldClick {
		t.Fatalf("expected click for inactive target")
	}
	if index != 3 {
		t.Fatalf("expected index 3, got %d", index)
	}
}

func TestChooseFilterTagPrefersInteractiveDuplicateWhenTargetNotActive(t *testing.T) {
	tags := []filterTagState{
		{Text: "不限", Active: true, Display: "flex"},
		{Text: "一周内", Active: false, Display: "block"},
		{Text: "一周内", Active: false, Display: "flex"},
	}

	index, shouldClick, err := chooseFilterTag(tags, "一周内")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldClick {
		t.Fatalf("expected click for inactive target")
	}
	if index != 2 {
		t.Fatalf("expected interactive duplicate index 2, got %d", index)
	}
}
