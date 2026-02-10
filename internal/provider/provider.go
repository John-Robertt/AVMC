package provider

import (
	"context"
	"net/http"

	"github.com/John-Robertt/AVMC/internal/domain"
)

// Provider 把“站点变化”限制在 provider 包内部；核心流程只依赖统一接口与稳定的 MovieMeta。
//
// 约束：
// - Fetch 不做缓存、不做重试、不做限速（这些由核心 http/cache 层统一实现）
// - Parse 必须是纯函数：相同输入 => 相同输出
// - pageURL 必须是详情页（用于 NFO <website> 与 report 追溯）
type Provider interface {
	Name() string
	Fetch(ctx context.Context, code domain.Code, c *http.Client) (html []byte, pageURL string, err error)
	Parse(code domain.Code, html []byte, pageURL string) (domain.MovieMeta, error)
}
