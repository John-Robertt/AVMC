package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEffective_ConfigNotFound(t *testing.T) {
	cwd := t.TempDir()

	_, err := LoadEffective(cwd, CLIArgs{})
	if Code(err) != ErrCodeNotFound {
		t.Fatalf("期望 %q，实际 err=%v (code=%q)", ErrCodeNotFound, err, Code(err))
	}
}

func TestLoadEffective_ConfigMissingPath(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "avmc.json"), []byte(`{"provider":"javdb"}`))

	_, err := LoadEffective(cwd, CLIArgs{})
	if Code(err) != ErrCodeMissingPath {
		t.Fatalf("期望 %q，实际 err=%v (code=%q)", ErrCodeMissingPath, err, Code(err))
	}
}

func TestLoadEffective_ApplyCLIOverride(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "avmc.json"), []byte(`{"path":"videos","apply":true}`))

	eff, err := LoadEffective(cwd, CLIArgs{
		Apply:    false,
		ApplySet: true, // --apply=false
	})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if eff.Apply != false {
		t.Fatalf("期望 apply=false，实际=%v", eff.Apply)
	}

	wantPath := filepath.Join(cwd, "videos")
	if eff.Path != wantPath {
		t.Fatalf("期望 path=%q，实际=%q", wantPath, eff.Path)
	}
}

func TestLoadEffective_ProviderMergeOrder(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "avmc.json"), []byte(`{"path":"p","provider":"javdb"}`))

	// CLI 未指定 provider，则应使用配置文件中的 javdb。
	eff, err := LoadEffective(cwd, CLIArgs{})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if eff.Provider != "javdb" {
		t.Fatalf("期望 provider=javdb，实际=%q", eff.Provider)
	}

	// CLI 显式指定，则覆盖配置文件。
	eff2, err := LoadEffective(cwd, CLIArgs{
		Provider:    "javbus",
		ProviderSet: true,
	})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if eff2.Provider != "javbus" {
		t.Fatalf("期望 provider=javbus，实际=%q", eff2.Provider)
	}
}

func TestLoadEffective_CLIPath_ConfigOptional(t *testing.T) {
	cwd := t.TempDir()
	root := filepath.Join(cwd, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}

	eff, err := LoadEffective(cwd, CLIArgs{
		Path: root,
	})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if eff.Path != root {
		t.Fatalf("期望 path=%q，实际=%q", root, eff.Path)
	}
	if eff.Provider != DefaultProvider {
		t.Fatalf("期望 provider=%q，实际=%q", DefaultProvider, eff.Provider)
	}
}

func TestLoadEffective_InvalidProvider(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "avmc.json"), []byte(`{"path":"p","provider":"nope"}`))

	_, err := LoadEffective(cwd, CLIArgs{})
	if Code(err) != ErrCodeInvalid {
		t.Fatalf("期望 %q，实际 err=%v (code=%q)", ErrCodeInvalid, err, Code(err))
	}
}

func TestLoadEffective_CLIPath_InvalidConfig(t *testing.T) {
	cwd := t.TempDir()
	root := filepath.Join(cwd, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	writeFile(t, filepath.Join(root, "avmc.json"), []byte(`{`))

	_, err := LoadEffective(cwd, CLIArgs{Path: root})
	if Code(err) != ErrCodeInvalid {
		t.Fatalf("期望 %q，实际 err=%v (code=%q)", ErrCodeInvalid, err, Code(err))
	}
}

func TestLoadEffective_ImageProxyRequiresProxyURL(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "avmc.json"), []byte(`{"path":"p","image_proxy":true}`))

	_, err := LoadEffective(cwd, CLIArgs{})
	if Code(err) != ErrCodeInvalid {
		t.Fatalf("期望 %q，实际 err=%v (code=%q)", ErrCodeInvalid, err, Code(err))
	}
}

func TestLoadEffective_InvalidProxyURL(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "avmc.json"), []byte(`{"path":"p","proxy":{"url":"http://[::1"}}`))

	_, err := LoadEffective(cwd, CLIArgs{})
	if Code(err) != ErrCodeInvalid {
		t.Fatalf("期望 %q，实际 err=%v (code=%q)", ErrCodeInvalid, err, Code(err))
	}
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("写入文件失败 %q：%v", path, err)
	}
}
