package code

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestExtract_NormalizeVariants(t *testing.T) {
	v := domain.VideoFile{
		AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "cawd_895.mp4"),
		Base:    "cawd_895",
	}
	got, err := Extract(v)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if string(got) != "CAWD-895" {
		t.Fatalf("期望 CAWD-895，实际 %q", got)
	}
}

func TestExtract_AmbiguousInFilename(t *testing.T) {
	v := domain.VideoFile{
		AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "x.mp4"),
		Base:    "cawd-895 abcd_123",
	}
	_, err := Extract(v)

	var ue *UnmatchedError
	if !errors.As(err, &ue) || ue.Kind != "ambiguous" {
		t.Fatalf("期望 ambiguous，实际 err=%v", err)
	}
	if len(ue.Candidates) != 2 {
		t.Fatalf("期望 2 个候选，实际 %d", len(ue.Candidates))
	}
}

func TestExtract_AmbiguousBetweenFileAndDir(t *testing.T) {
	v := domain.VideoFile{
		AbsPath: filepath.Join(string(filepath.Separator), "tmp", "ABCD-123", "CAWD-895.mp4"),
		Base:    "CAWD-895",
	}
	_, err := Extract(v)

	var ue *UnmatchedError
	if !errors.As(err, &ue) || ue.Kind != "ambiguous" {
		t.Fatalf("期望 ambiguous，实际 err=%v", err)
	}
}

func TestExtract_NoMatch(t *testing.T) {
	v := domain.VideoFile{
		AbsPath: filepath.Join(string(filepath.Separator), "tmp", "x", "hello.mp4"),
		Base:    "hello",
	}
	_, err := Extract(v)

	var ue *UnmatchedError
	if !errors.As(err, &ue) || ue.Kind != "no_match" {
		t.Fatalf("期望 no_match，实际 err=%v", err)
	}
}
