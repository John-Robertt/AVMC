package cache

import (
	"errors"
	"os"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestStore_ReadWriteProviderCache(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")

	s := New(root, false)
	if err := s.WriteProviderHTML("javbus", code, []byte("<html/>")); err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	b, ok, err := s.ReadProviderHTML("javbus", code)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if !ok {
		t.Fatalf("期望命中缓存，但 ok=false")
	}
	if string(b) != "<html/>" {
		t.Fatalf("内容不一致：%q", string(b))
	}

	path, err := s.ProviderHTMLPath("javbus", code)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("期望文件存在，但 Stat 失败：%v", err)
	}
}

func TestStore_ReadOnlyRejectWrite(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")

	s := New(root, true)
	err := s.WriteProviderJSON("javdb", code, []byte(`{"ok":true}`))
	if !errors.Is(err, ErrReadOnly) {
		t.Fatalf("期望 ErrReadOnly，实际：%v", err)
	}

	path, err := s.ProviderJSONPath("javdb", code)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("期望文件不存在，但 Stat err=%v", err)
	}
}
