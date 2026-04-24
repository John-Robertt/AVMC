package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/infra/cache"
	"github.com/John-Robertt/AVMC/internal/provider"
)

func scrape(ctx context.Context, store cache.Store, reg provider.Registry, providerRequested string, code domain.Code, c *http.Client, allowWrite bool) (domain.MovieMeta, string, string, []byte, []domain.ProviderAttempt, error) {
	// 先尝试 cache（只读），命中则不再打网络。
	if b, ok, err := store.ReadProviderJSON(providerRequested, code); err == nil && ok {
		var meta domain.MovieMeta
		if e := json.Unmarshal(b, &meta); e == nil {
			return meta, providerRequested, meta.Website, nil, []domain.ProviderAttempt{{
				Provider:  providerRequested,
				Stage:     "ok",
				ErrorCode: "",
				ErrorMsg:  "",
			}}, nil
		}
		// 坏缓存：忽略，走网络（apply 会写回新缓存；dry-run 只验证）。
	}

	meta, used, website, html, trace, err := provider.FetchParseTrace(ctx, reg, providerRequested, code, c)
	if err != nil {
		return domain.MovieMeta{}, "", "", nil, attemptsFromTrace(trace), err
	}

	// apply：写缓存（HTML + JSON）。dry-run 禁止写入。
	if allowWrite && !store.ReadOnly {
		_ = store.WriteProviderHTML(used, code, html)
		if b, e := json.Marshal(meta); e == nil {
			_ = store.WriteProviderJSON(used, code, b)
		}
	}
	return meta, used, website, html, attemptsFromTrace(trace), nil
}

func attemptsFromTrace(trace []provider.Attempt) []domain.ProviderAttempt {
	if len(trace) == 0 {
		return []domain.ProviderAttempt{}
	}
	out := make([]domain.ProviderAttempt, 0, len(trace))
	for _, a := range trace {
		at := domain.ProviderAttempt{
			Provider:  a.Provider,
			Stage:     a.Stage,
			ErrorCode: "",
			ErrorMsg:  "",
		}
		switch a.Stage {
		case "fetch":
			at.ErrorCode = domain.ErrCodeFetchFailed
			at.ErrorMsg = stripProviderPrefix(a.Provider, humanizeFetchError(a.Provider, a.Err))
		case "parse":
			at.ErrorCode = domain.ErrCodeParseFailed
			at.ErrorMsg = stripProviderPrefix(a.Provider, humanizeParseError(a.Provider, a.Err))
		case "ok":
			// ok
		default:
			// 兜底：未知阶段也保留错误信息，避免“无痕失败”。
			if a.Err != nil {
				at.ErrorCode = domain.ErrCodeFetchFailed
				at.ErrorMsg = a.Err.Error()
			}
		}
		out = append(out, at)
	}
	return out
}

func stripProviderPrefix(providerName, msg string) string {
	msg = strings.TrimSpace(msg)
	p := strings.TrimSpace(providerName)
	if p == "" || msg == "" {
		return msg
	}
	prefix := p + " "
	if strings.HasPrefix(msg, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(msg, prefix))
	}
	return msg
}

func fillProviderError(item *domain.ItemResult, err error) {
	item.Status = domain.StatusFailed

	var pe *provider.Error
	if errors.As(err, &pe) {
		switch pe.Stage {
		case "fetch":
			item.ErrorCode = domain.ErrCodeFetchFailed
			item.ErrorMsg = humanizeFetchError(pe.Provider, pe.Err)
		case "parse":
			item.ErrorCode = domain.ErrCodeParseFailed
			item.ErrorMsg = humanizeParseError(pe.Provider, pe.Err)
		default:
			item.ErrorCode = domain.ErrCodeFetchFailed
			item.ErrorMsg = fmt.Sprintf("%s 失败：%v", pe.Provider, pe.Err)
		}
		return
	}

	item.ErrorCode = domain.ErrCodeFetchFailed
	item.ErrorMsg = err.Error()
}

func humanizeFetchError(providerName string, err error) string {
	if err == nil {
		return providerName + " 抓取失败"
	}

	var be *provider.BlockedError
	if errors.As(err, &be) {
		switch be.Reason {
		case "driver-verify":
			return fmt.Sprintf("%s 被站点引导到验证页（driver-verify）。当前不支持绕过；建议配置 proxy.url 代理池或改用另一 provider。", providerName)
		default:
			return fmt.Sprintf("%s 被站点拦截（%s）。建议配置 proxy.url 或稍后重试。", providerName, be.Reason)
		}
	}

	// HTTP 非 2xx：尽量给出可操作提示（反爬/限流/验证跳转是最常见问题）。
	var hs *provider.HTTPStatusError
	if errors.As(err, &hs) {
		loc := strings.TrimSpace(hs.Location)
		if hs.StatusCode >= 300 && hs.StatusCode < 400 && strings.Contains(loc, "driver-verify") {
			return fmt.Sprintf("%s 被站点跳转到验证页（driver-verify）。当前不支持绕过；建议配置 proxy.url 代理池或改用另一 provider。", providerName)
		}
		switch hs.StatusCode {
		case 403, 429:
			return fmt.Sprintf("%s 返回 HTTP %d（可能触发反爬/限流）。建议降低并发或配置 proxy.url。", providerName, hs.StatusCode)
		case 404:
			return fmt.Sprintf("%s 返回 HTTP 404（可能该 CODE 不存在/已下架）。", providerName)
		default:
			if loc != "" {
				return fmt.Sprintf("%s 返回 HTTP %d（重定向）：%s", providerName, hs.StatusCode, loc)
			}
			return fmt.Sprintf("%s 返回 HTTP %d。", providerName, hs.StatusCode)
		}
	}

	low := strings.ToLower(err.Error())
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(low, "timeout") {
		return fmt.Sprintf("%s 抓取超时。建议检查网络/代理，或降低并发后重试。", providerName)
	}
	if strings.Contains(low, "tls") || strings.Contains(low, "handshake") || strings.Contains(low, "ssl") {
		if providerName == "javdb" {
			return "javdb 连接失败（TLS/SSL 握手异常或域名不可达）。可在 avmc.json 设置 javdb_base_url 指向可用域名，或配置 proxy.url。"
		}
		return fmt.Sprintf("%s 连接失败（TLS/SSL）。建议配置 proxy.url 或稍后重试。", providerName)
	}

	return fmt.Sprintf("%s 抓取失败：%v", providerName, err)
}

func humanizeParseError(providerName string, err error) string {
	if err == nil {
		return providerName + " 解析失败"
	}
	// 解析失败通常意味着站点结构漂移或被返回了非预期页面（例如验证页/空内容）。
	return fmt.Sprintf("%s 解析失败（站点结构可能变化或返回了非详情页内容）：%v", providerName, err)
}
