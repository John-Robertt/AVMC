package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestReadOutState_ExistingSidecars(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")

	outDir := filepath.Join(root, "out", string(code))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	write(t, filepath.Join(outDir, string(code)+".nfo"))
	write(t, filepath.Join(outDir, "poster.jpg"))
	write(t, filepath.Join(outDir, "fanart.jpg"))

	st, err := ReadOutState(root, code)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if !st.HasNFO || !st.HasPoster || !st.HasFanart {
		t.Fatalf("期望 sidecar 都存在：%+v", st)
	}
}

func TestPlanItem_NoScrapeWhenSidecarsComplete(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")

	outDir := filepath.Join(root, "out", string(code))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	write(t, filepath.Join(outDir, string(code)+".nfo"))
	write(t, filepath.Join(outDir, "poster.jpg"))
	write(t, filepath.Join(outDir, "fanart.jpg"))

	st, err := ReadOutState(root, code)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	files := []domain.VideoFile{
		{AbsPath: filepath.Join(root, "in", "CAWD-895.mp4"), RelPath: "in/CAWD-895.mp4", Base: "CAWD-895", Ext: ".mp4"},
	}
	item := domain.WorkItem{Code: code, FileIdx: []int{0}}

	plan, err := PlanItem("javbus", files, item, st)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if plan.Need.NeedScrape {
		t.Fatalf("期望 NeedScrape=false，实际=%+v", plan.Need)
	}
}

func TestPlanItem_NameConflictDeterministic(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")

	outDir := filepath.Join(root, "out", string(code))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	// 目标目录已有同名与 __2，计划应生成 __3。
	write(t, filepath.Join(outDir, "A.mp4"))
	write(t, filepath.Join(outDir, "A__2.mp4"))

	st, err := ReadOutState(root, code)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	files := []domain.VideoFile{
		{AbsPath: filepath.Join(root, "in", "A.mp4"), RelPath: "in/A.mp4", Base: "A", Ext: ".mp4"},
	}
	item := domain.WorkItem{Code: code, FileIdx: []int{0}}

	plan, err := PlanItem("javbus", files, item, st)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(plan.Moves) != 1 {
		t.Fatalf("期望 1 条 move，实际 %d", len(plan.Moves))
	}
	wantDst := filepath.Join(outDir, "A__3.mp4")
	if plan.Moves[0].DstAbs != wantDst {
		t.Fatalf("期望 dst=%q，实际=%q", wantDst, plan.Moves[0].DstAbs)
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入文件失败：%v", err)
	}
}
