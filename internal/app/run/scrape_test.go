package run

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/infra/cache"
	"github.com/John-Robertt/AVMC/internal/provider"
)

func TestScrape_UsesRequestedProviderJSONCache(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")
	store := cache.New(root, false)

	cached := domain.MovieMeta{
		Code:    code,
		Title:   "cached",
		Website: "https://cache.test/detail/CAWD-895",
	}
	b, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("marshal cache 失败：%v", err)
	}
	if err := store.WriteProviderJSON("javbus", code, b); err != nil {
		t.Fatalf("写入 cache 失败：%v", err)
	}

	javbus := &countingProvider{name: "javbus", meta: domain.MovieMeta{Title: "network"}}
	javdb := &countingProvider{name: "javdb", meta: domain.MovieMeta{Title: "fallback"}}
	reg, err := provider.NewRegistry(javbus, javdb)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	meta, used, website, _, attempts, err := scrape(context.Background(), store, reg, "javbus", code, http.DefaultClient, false)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if meta.Title != "cached" || used != "javbus" || website != cached.Website {
		t.Fatalf("cache 命中结果不符合预期：meta=%+v used=%q website=%q", meta, used, website)
	}
	if len(attempts) != 1 || attempts[0].Stage != "ok" {
		t.Fatalf("cache 命中 attempts 不符合预期：%+v", attempts)
	}
	if javbus.fetchCalls != 0 || javdb.fetchCalls != 0 {
		t.Fatalf("cache 命中不应访问网络：javbus=%d javdb=%d", javbus.fetchCalls, javdb.fetchCalls)
	}
}

func TestScrape_IgnoresBadJSONCacheAndFetches(t *testing.T) {
	root := t.TempDir()
	code, _ := domain.ParseCode("CAWD-895")
	store := cache.New(root, false)

	if err := store.WriteProviderJSON("javbus", code, []byte("{bad json")); err != nil {
		t.Fatalf("写入坏 cache 失败：%v", err)
	}

	javbus := &countingProvider{name: "javbus", meta: domain.MovieMeta{Title: "network"}}
	javdb := &countingProvider{name: "javdb", meta: domain.MovieMeta{Title: "fallback"}}
	reg, err := provider.NewRegistry(javbus, javdb)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	meta, used, _, _, attempts, err := scrape(context.Background(), store, reg, "javbus", code, http.DefaultClient, false)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if meta.Title != "network" || used != "javbus" {
		t.Fatalf("坏 cache 后应走网络：meta=%+v used=%q", meta, used)
	}
	if len(attempts) != 1 || attempts[0].Provider != "javbus" || attempts[0].Stage != "ok" {
		t.Fatalf("attempts 不符合预期：%+v", attempts)
	}
	if javbus.fetchCalls != 1 {
		t.Fatalf("期望 javbus fetch 1 次，实际=%d", javbus.fetchCalls)
	}
}
