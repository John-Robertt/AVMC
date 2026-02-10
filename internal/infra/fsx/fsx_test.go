package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic_SuccessAndNoTempLeft(t *testing.T) {
	dir := t.TempDir()

	if err := WriteFileAtomic(dir, "a.txt", []byte("hello")); err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("内容不一致：%q", string(b))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir 失败：%v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".a.txt.tmp-") {
			t.Fatalf("临时文件未清理：%q", e.Name())
		}
	}
}

func TestWriteFileAtomic_RenameFail_CleanupTemp(t *testing.T) {
	dir := t.TempDir()

	old := renameFunc
	renameFunc = func(oldpath, newpath string) error {
		return os.ErrPermission
	}
	defer func() { renameFunc = old }()

	err := WriteFileAtomic(dir, "a.txt", []byte("hello"))
	if err == nil {
		t.Fatalf("期望失败，但得到 nil")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir 失败：%v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".a.txt.tmp-") {
			t.Fatalf("临时文件未清理：%q", e.Name())
		}
		if e.Name() == "a.txt" {
			t.Fatalf("不应写出最终文件：%q", e.Name())
		}
	}
}

func TestWriteFileAtomicNoOverwrite_TargetConflictDir(t *testing.T) {
	dir := t.TempDir()

	// 目标路径是目录：应返回 PathTypeConflictError，而不是 os.ErrExist。
	if err := os.Mkdir(filepath.Join(dir, "a.txt"), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}

	err := WriteFileAtomicNoOverwrite(dir, "a.txt", []byte("hello"))
	if err == nil {
		t.Fatalf("期望错误，但得到 nil")
	}
	if !IsPathTypeConflict(err) {
		t.Fatalf("期望 PathTypeConflictError，实际：%T %v", err, err)
	}
}
