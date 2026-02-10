package app

import (
	"path/filepath"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestGroupByCode_MergeSameCode(t *testing.T) {
	files := []domain.VideoFile{
		{AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "CAWD-895.mp4"), RelPath: "b.mp4", Base: "CAWD-895"},
		{AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "CAWD-895-2.mp4"), RelPath: "a.mp4", Base: "CAWD-895"},
	}

	items, unmatched, err := GroupByCode(files)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(unmatched) != 0 {
		t.Fatalf("不期望 unmatched：%v", unmatched)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 个 item，实际 %d", len(items))
	}
	if string(items[0].Code) != "CAWD-895" {
		t.Fatalf("期望 CAWD-895，实际 %q", items[0].Code)
	}
	// item 内必须按 RelPath 排序：a.mp4 在 b.mp4 之前。
	if len(items[0].FileIdx) != 2 || items[0].FileIdx[0] != 1 || items[0].FileIdx[1] != 0 {
		t.Fatalf("FileIdx 排序不稳定：%v", items[0].FileIdx)
	}
}

func TestGroupByCode_Unmatched(t *testing.T) {
	files := []domain.VideoFile{
		{AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "hello.mp4"), RelPath: "hello.mp4", Base: "hello"},
		{AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "CAWD-895.mp4"), RelPath: "CAWD-895.mp4", Base: "CAWD-895"},
	}

	items, unmatched, err := GroupByCode(files)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 个 item，实际 %d", len(items))
	}
	if len(unmatched) != 1 {
		t.Fatalf("期望 1 个 unmatched，实际 %d", len(unmatched))
	}
}
