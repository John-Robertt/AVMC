package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ErrCodeNotFound 表示无参运行但 cwd 下没有 avmc.json。
	ErrCodeNotFound = "config_not_found"
	// ErrCodeInvalid 表示配置文件无法读取/解析，或字段不合法。
	ErrCodeInvalid = "config_invalid"
	// ErrCodeMissingPath 表示无参运行但配置文件缺少 path 字段。
	ErrCodeMissingPath = "config_missing_path"
)

const (
	// DefaultProvider 是 provider 的最终默认值（当 CLI 与配置文件都未指定时）。
	DefaultProvider = "javbus"
	// DefaultConcurrency 是并发的内置默认值（当配置未指定时）。
	DefaultConcurrency = 4
)

// CLIArgs 只包含 CLI 暴露的三项入口（path/provider/apply），并保留“是否显式指定”的信息。
// 这能保证覆盖优先级可实现：例如 --apply=false 必须能覆盖 config.apply=true。
type CLIArgs struct {
	Path string

	Provider    string
	ProviderSet bool

	Apply    bool
	ApplySet bool
}

// FileConfig 对应 avmc.json（v2）的解析结构。
type FileConfig struct {
	Path         string          `json:"path"`
	Provider     string          `json:"provider"`
	Apply        *bool           `json:"apply"`
	Concurrency  int             `json:"concurrency"`
	Proxy        *ProxyConfig    `json:"proxy"`
	ImageProxy   bool            `json:"image_proxy"`
	ExcludeDirs  []string        `json:"exclude_dirs"`
	JavDBBaseURL string          `json:"javdb_base_url"`
	_            json.RawMessage `json:"-"` // 预留：禁止在 Phase 1 做“未知字段报错”的决定
}

type ProxyConfig struct {
	URL string `json:"url"`
}

// EffectiveConfig 是合并并做最小规范化后的最终配置（实现层直接消费，不再做二次默认/优先级判断）。
type EffectiveConfig struct {
	Path string

	Provider string
	Apply    bool

	Concurrency int
	ProxyURL    string
	ImageProxy  bool
	ExcludeDirs []string

	// JavDBBaseURL 允许在 javdb.com 不可达/被阻断时切换到可用镜像域名（可选）。
	// 该字段属于高级能力，仅通过 avmc.json 配置，不暴露 CLI 参数。
	JavDBBaseURL string
}

// Error 是配置阶段的结构化错误（带 error_code）。
type Error struct {
	Code string
	Path string
	Err  error
}

func (e *Error) Error() string {
	switch e.Code {
	case ErrCodeNotFound:
		return fmt.Sprintf("%s：未找到配置文件 %q", e.Code, e.Path)
	case ErrCodeMissingPath:
		return fmt.Sprintf("%s：配置文件 %q 缺少必填字段 path", e.Code, e.Path)
	case ErrCodeInvalid:
		if e.Err != nil {
			return fmt.Sprintf("%s：配置文件 %q 无效：%v", e.Code, e.Path, e.Err)
		}
		return fmt.Sprintf("%s：配置文件 %q 无效", e.Code, e.Path)
	default:
		if e.Err != nil {
			return fmt.Sprintf("%s：%v", e.Code, e.Err)
		}
		return e.Code
	}
}

func (e *Error) Unwrap() error { return e.Err }

// Code 从 error 中提取 error_code；若不是 *Error 则返回空串。
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// LoadEffective 按文档约定发现并读取配置文件，然后与 CLI 参数合并为最终配置。
//
// 发现规则（固定）：
// 1) CLI 提供 path：尝试读取 <path>/avmc.json（可选）
// 2) CLI 未提供 path：必须读取 <cwd>/avmc.json（必选），且其中必须包含 path
//
// 覆盖优先级（固定）：
// - path：CLI path > config path
// - provider：CLI > config > 默认 javbus
// - apply：CLI --apply/--apply=false > config > 默认 false
// - 其他字段：仅由 config 控制（CLI 不暴露）
func LoadEffective(cwd string, cli CLIArgs) (EffectiveConfig, error) {
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cwd, Err: err}
	}

	var (
		cfgPath string
		fc      FileConfig
	)

	if strings.TrimSpace(cli.Path) != "" {
		// CLI 给了 path：配置文件可选，位置固定在 <path>/avmc.json。
		absPath := absCleanFrom(cwdAbs, cli.Path)
		cfgPath = filepath.Join(absPath, "avmc.json")

		var exists bool
		fc, exists, err = readFileConfig(cfgPath)
		if err != nil {
			return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: err}
		}
		_ = exists // 不存在也不报错

		return merge(absPath, cli, fc, cfgPath)
	}

	// CLI 没给 path：必须读取 <cwd>/avmc.json，且其中必须包含 path。
	cfgPath = filepath.Join(cwdAbs, "avmc.json")
	var exists bool
	fc, exists, err = readFileConfig(cfgPath)
	if err != nil {
		return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: err}
	}
	if !exists {
		return EffectiveConfig{}, &Error{Code: ErrCodeNotFound, Path: cfgPath, Err: os.ErrNotExist}
	}
	if strings.TrimSpace(fc.Path) == "" {
		return EffectiveConfig{}, &Error{Code: ErrCodeMissingPath, Path: cfgPath}
	}

	absPath := absCleanFrom(cwdAbs, fc.Path)
	return merge(absPath, cli, fc, cfgPath)
}

func merge(absPath string, cli CLIArgs, fc FileConfig, cfgPath string) (EffectiveConfig, error) {
	// provider：CLI > config > 默认
	provider := DefaultProvider
	if cli.ProviderSet {
		provider = cli.Provider
	} else if strings.TrimSpace(fc.Provider) != "" {
		provider = fc.Provider
	}
	if err := validateProvider(provider); err != nil {
		return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: err}
	}

	// apply：CLI > config > 默认 false
	apply := false
	if cli.ApplySet {
		apply = cli.Apply
	} else if fc.Apply != nil {
		apply = *fc.Apply
	}

	concurrency := fc.Concurrency
	if concurrency == 0 {
		concurrency = DefaultConcurrency
	}
	// 文档约定：范围建议 [1, 32]；超出截断。
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 32 {
		concurrency = 32
	}

	proxyURL := ""
	if fc.Proxy != nil {
		proxyURL = strings.TrimSpace(fc.Proxy.URL)
	}
	if proxyURL != "" {
		if _, err := url.Parse(proxyURL); err != nil {
			return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: fmt.Errorf("proxy.url 无效：%w", err)}
		}
	}
	if fc.ImageProxy && proxyURL == "" {
		return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: fmt.Errorf("image_proxy=true 但 proxy.url 为空")}
	}

	javdbBaseURL := strings.TrimSpace(fc.JavDBBaseURL)
	if javdbBaseURL != "" {
		u, err := url.Parse(javdbBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: fmt.Errorf("javdb_base_url 无效：%q", javdbBaseURL)}
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return EffectiveConfig{}, &Error{Code: ErrCodeInvalid, Path: cfgPath, Err: fmt.Errorf("javdb_base_url 必须是 http/https：%q", javdbBaseURL)}
		}
	}

	return EffectiveConfig{
		Path:         absPath,
		Provider:     provider,
		Apply:        apply,
		Concurrency:  concurrency,
		ProxyURL:     proxyURL,
		ImageProxy:   fc.ImageProxy,
		ExcludeDirs:  append([]string(nil), fc.ExcludeDirs...),
		JavDBBaseURL: javdbBaseURL,
	}, nil
}

func validateProvider(p string) error {
	switch p {
	case "javbus", "javdb":
		return nil
	case "":
		return fmt.Errorf("provider 不能为空")
	default:
		return fmt.Errorf("provider 只能是 javbus 或 javdb，实际是 %q", p)
	}
}

// absCleanFrom 以 base 为基准，把 p 变为 clean + absolute。
// - p 若已是绝对路径：直接 Clean
// - p 若是相对路径：Join(base, p) 后 Clean
func absCleanFrom(base, p string) string {
	p = filepath.Clean(strings.TrimSpace(p))
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Clean(filepath.Join(base, p))
}

// readFileConfig 读取并解析 JSON 配置文件。
// 返回值 exists 表示该文件是否存在（不存在不算错误）。
func readFileConfig(path string) (fc FileConfig, exists bool, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, false, nil
		}
		return FileConfig{}, false, err
	}
	if err := json.Unmarshal(b, &fc); err != nil {
		return FileConfig{}, true, err
	}
	return fc, true, nil
}
