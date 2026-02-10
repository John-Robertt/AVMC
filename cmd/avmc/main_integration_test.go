package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestCLI_NoTTY_StdoutOnlyRunReportJSON(t *testing.T) {
	// 这个测试锁定对外契约：stdout 非 TTY 时只能输出一个 RunReport JSON（进度/配置必须走 stderr 或直接禁用）。
	root := t.TempDir()

	// 准备最小输入：一个视频文件。
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	// 准备 out 状态，避免 dry-run 触发真实 provider 抓取（NeedScrape=false）。
	outDir := filepath.Join(root, "out", "CAWD-895")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("创建 out 目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "CAWD-895.nfo"), []byte("n"), 0o644); err != nil {
		t.Fatalf("写入 nfo 失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "fanart.jpg"), []byte("f"), 0o644); err != nil {
		t.Fatalf("写入 fanart 失败：%v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("读取 cwd 失败：%v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))

	cmd := exec.Command("go", "run", "./cmd/avmc", "run", root)
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("命令执行失败：%v\nstderr=%s\nstdout=%s", err, stderr.String(), stdout.String())
	}

	// stdout 必须是单个 JSON。
	var rr domain.RunReport
	if err := json.Unmarshal(stdout.Bytes(), &rr); err != nil {
		t.Fatalf("stdout 不是合法的 RunReport JSON：%v\nstdout=%q", err, stdout.String())
	}
	// 进度/配置不应出现在 stdout。
	if strings.Contains(stdout.String(), "配置（生效）") || strings.Contains(stdout.String(), "进度:") {
		t.Fatalf("stdout 不应包含进度/配置输出：%q", stdout.String())
	}

	// stderr 至少应包含最终摘要行。
	if !strings.Contains(stderr.String(), "完成：processed=") {
		t.Fatalf("stderr 缺少完成摘要：%q", stderr.String())
	}
}

