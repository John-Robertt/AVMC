package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

// FetchParse 尝试按“requested -> fallback”顺序抓取并解析元数据。
//
// 返回值：
// - meta：成功解析的结构化元数据
// - providerUsed：最终成功的 provider name
// - website：详情页 URL（也是来源标记）
// - html：抓取到的原始 HTML（用于 cache）
func FetchParse(ctx context.Context, reg Registry, providerRequested string, code domain.Code, c *http.Client) (meta domain.MovieMeta, providerUsed string, website string, html []byte, err error) {
	providerRequested = strings.ToLower(strings.TrimSpace(providerRequested))
	if providerRequested == "" {
		return domain.MovieMeta{}, "", "", nil, fmt.Errorf("provider_requested 不能为空")
	}
	if code == "" {
		return domain.MovieMeta{}, "", "", nil, fmt.Errorf("code 不能为空")
	}

	order, err := fallbackOrder(providerRequested)
	if err != nil {
		return domain.MovieMeta{}, "", "", nil, err
	}

	var lastErr error
	for _, name := range order {
		p, ok := reg.Get(name)
		if !ok {
			lastErr = fmt.Errorf("provider 未注册：%q", name)
			continue
		}

		h, pageURL, ferr := p.Fetch(ctx, code, c)
		if ferr != nil {
			lastErr = &Error{Provider: name, Stage: "fetch", Err: ferr}
			continue
		}

		m, perr := p.Parse(code, h, pageURL)
		if perr != nil {
			lastErr = &Error{Provider: name, Stage: "parse", Err: perr}
			continue
		}

		m.Website = pageURL
		return m, name, pageURL, h, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无可用 provider")
	}
	return domain.MovieMeta{}, "", "", nil, lastErr
}

// Error 是 provider 阶段的可追溯错误。
// 上层可以据此把失败归类为 fetch_failed / parse_failed，并写入 report。
type Error struct {
	Provider string // provider name（小写）
	Stage    string // "fetch" 或 "parse"
	Err      error
}

func (e *Error) Error() string {
	return fmt.Sprintf("provider=%s stage=%s: %v", e.Provider, e.Stage, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func fallbackOrder(requested string) ([]string, error) {
	switch requested {
	case "javbus":
		return []string{"javbus", "javdb"}, nil
	case "javdb":
		return []string{"javdb", "javbus"}, nil
	default:
		return nil, fmt.Errorf("未知 provider：%q", requested)
	}
}
