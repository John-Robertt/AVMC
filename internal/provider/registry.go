package provider

import (
	"fmt"
	"strings"
)

// Registry 是 provider 的只读注册表（按 name 索引）。
// 用 map 做 O(1) 查找；provider 数量极小，保持简单即可。
type Registry struct {
	byName map[string]Provider
}

func NewRegistry(providers ...Provider) (Registry, error) {
	byName := make(map[string]Provider, len(providers))
	for _, p := range providers {
		if p == nil {
			return Registry{}, fmt.Errorf("provider 不能为空")
		}
		name := strings.ToLower(strings.TrimSpace(p.Name()))
		if name == "" {
			return Registry{}, fmt.Errorf("provider.Name 不能为空")
		}
		if _, ok := byName[name]; ok {
			return Registry{}, fmt.Errorf("重复的 provider：%q", name)
		}
		byName[name] = p
	}
	return Registry{byName: byName}, nil
}

func (r Registry) Get(name string) (Provider, bool) {
	if r.byName == nil {
		return nil, false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	p, ok := r.byName[name]
	return p, ok
}
