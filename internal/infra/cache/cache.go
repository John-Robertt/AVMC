package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/infra/fsx"
)

// Store 提供 <path>/cache/ 下的文件缓存读写。
//
// 约束：
// - dry-run：只允许读（ReadOnly=true）
// - apply：允许写（ReadOnly=false）
type Store struct {
	Root     string // <path>（扫描根目录）
	ReadOnly bool
}

var ErrReadOnly = errors.New("cache: read-only")

func New(root string, readOnly bool) Store {
	return Store{
		Root:     filepath.Clean(strings.TrimSpace(root)),
		ReadOnly: readOnly,
	}
}

// ProviderHTMLPath 返回 provider HTML 缓存的绝对路径。
func (s Store) ProviderHTMLPath(provider string, code domain.Code) (string, error) {
	p, err := cleanProvider(provider)
	if err != nil {
		return "", err
	}
	if code == "" {
		return "", fmt.Errorf("code 不能为空")
	}
	return filepath.Join(s.Root, "cache", "providers", p, string(code)+".html"), nil
}

// ProviderJSONPath 返回 provider JSON 缓存的绝对路径。
func (s Store) ProviderJSONPath(provider string, code domain.Code) (string, error) {
	p, err := cleanProvider(provider)
	if err != nil {
		return "", err
	}
	if code == "" {
		return "", fmt.Errorf("code 不能为空")
	}
	return filepath.Join(s.Root, "cache", "providers", p, string(code)+".json"), nil
}

func (s Store) ReadProviderHTML(provider string, code domain.Code) ([]byte, bool, error) {
	path, err := s.ProviderHTMLPath(provider, code)
	if err != nil {
		return nil, false, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

func (s Store) ReadProviderJSON(provider string, code domain.Code) ([]byte, bool, error) {
	path, err := s.ProviderJSONPath(provider, code)
	if err != nil {
		return nil, false, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

func (s Store) WriteProviderHTML(provider string, code domain.Code, html []byte) error {
	if s.ReadOnly {
		return ErrReadOnly
	}
	p, err := cleanProvider(provider)
	if err != nil {
		return err
	}
	if code == "" {
		return fmt.Errorf("code 不能为空")
	}
	dir := filepath.Join(s.Root, "cache", "providers", p)
	name := string(code) + ".html"
	return fsx.WriteFileAtomicReplace(dir, name, html)
}

func (s Store) WriteProviderJSON(provider string, code domain.Code, json []byte) error {
	if s.ReadOnly {
		return ErrReadOnly
	}
	p, err := cleanProvider(provider)
	if err != nil {
		return err
	}
	if code == "" {
		return fmt.Errorf("code 不能为空")
	}
	dir := filepath.Join(s.Root, "cache", "providers", p)
	name := string(code) + ".json"
	return fsx.WriteFileAtomicReplace(dir, name, json)
}

var providerNameRE = regexp.MustCompile(`^[a-z0-9_]+$`)

func cleanProvider(p string) (string, error) {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "", fmt.Errorf("provider 不能为空")
	}
	// 最小约束：避免路径穿越；provider 名称本身是枚举（javbus/javdb），这里不做更多“聪明”处理。
	if !providerNameRE.MatchString(p) {
		return "", fmt.Errorf("非法 provider：%q", p)
	}
	return p, nil
}
