package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanVideos_ExcludeOutAndCache(t *testing.T) {
	root := t.TempDir()

	// 永久排除 out/cache。
	touch(t, filepath.Join(root, "out", "CAWD-895", "CAWD-895.mp4"))
	touch(t, filepath.Join(root, "cache", "x.mp4"))

	// 正常目录。
	touch(t, filepath.Join(root, "in", "CAWD-895.mp4"))
	touch(t, filepath.Join(root, "in", "ignore.txt"))

	got, err := ScanVideos(root, nil)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 个视频文件，实际 %d", len(got))
	}
	wantRel := filepath.Join("in", "CAWD-895.mp4")
	if got[0].RelPath != wantRel {
		t.Fatalf("期望 rel=%q，实际=%q", wantRel, got[0].RelPath)
	}
}

func TestScanVideos_ExcludeDirsFromConfig(t *testing.T) {
	root := t.TempDir()

	touch(t, filepath.Join(root, "temp", "A-01.mp4"))
	touch(t, filepath.Join(root, "ok", "B-02.mkv"))

	got, err := ScanVideos(root, []string{"temp"})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 个视频文件，实际 %d", len(got))
	}
	wantRel := filepath.Join("ok", "B-02.mkv")
	if got[0].RelPath != wantRel {
		t.Fatalf("期望 rel=%q，实际=%q", wantRel, got[0].RelPath)
	}
}

func TestScanVideos_ExtCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "X.MP4"))

	got, err := ScanVideos(root, nil)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 个视频文件，实际 %d", len(got))
	}
	if got[0].Ext != ".mp4" {
		t.Fatalf("期望 ext=.mp4，实际=%q", got[0].Ext)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入文件失败：%v", err)
	}
}
