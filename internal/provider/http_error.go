package provider

import (
	"fmt"
	"strings"
)

// HTTPStatusError 表示站点返回了非 2xx 的 HTTP 状态码。
// provider.Fetch 可以返回该错误，让上层生成更可操作的 error_msg。
type HTTPStatusError struct {
	URL        string
	StatusCode int
	Location   string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "HTTP status error"
	}
	loc := strings.TrimSpace(e.Location)
	if loc == "" {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d location=%s", e.StatusCode, loc)
}

// BlockedError 表示请求被站点引导到了“验证/拦截”页面（通常意味着需要浏览器执行 JS 或人工验证）。
// 产品约束：不尝试绕过，直接视为 fetch_failed，让上层走 provider 降级或提示用户配置代理。
type BlockedError struct {
	URL    string
	Reason string // 例如 "driver-verify"
}

func (e *BlockedError) Error() string {
	if e == nil {
		return "blocked"
	}
	if strings.TrimSpace(e.Reason) == "" {
		return "blocked"
	}
	return "blocked: " + strings.TrimSpace(e.Reason)
}
