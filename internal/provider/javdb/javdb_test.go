package javdb

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/John-Robertt/AVMC/internal/domain"
)

func TestFindDetailHref_FromSearchFixture(t *testing.T) {
	code, _ := domain.ParseCode("SNOS-052")
	searchHTML, err := os.ReadFile(filepath.Join("testdata", "search.html"))
	if err != nil {
		t.Fatalf("读取 search fixture 失败：%v", err)
	}

	href, err := findDetailHref(searchHTML, code)
	if err != nil {
		t.Fatalf("findDetailHref 失败：%v", err)
	}
	if href != "/v/ve39eW" {
		t.Fatalf("期望 href=/v/ve39eW，实际=%q", href)
	}
}

func TestParse_Golden(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("读取 testdata 失败：%v", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		if e.Name() == "search.html" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("未找到任何 fixture（testdata/*.html，排除 search.html）")
	}

	update := os.Getenv("UPDATE_GOLDEN") == "1"
	if update {
		if err := os.MkdirAll("golden", 0o755); err != nil {
			t.Fatalf("创建 golden 目录失败：%v", err)
		}
	}

	for _, name := range names {
		base := strings.TrimSuffix(name, ".html")
		code, ok := domain.ParseCode(base)
		if !ok {
			t.Fatalf("fixture 文件名不是合法 CODE：%s", name)
		}

		html, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("读取 fixture 失败：%v", err)
		}
		pageURL := canonicalURLFromHTML(t, html)
		if pageURL == "" {
			pageURL = "https://javdb.com/v/" + base
		}

		meta, err := Provider{}.Parse(code, html, pageURL)
		if err != nil {
			t.Fatalf("Parse 失败：code=%s fixture=%s err=%v", code, name, err)
		}

		got, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			t.Fatalf("json.Marshal 失败：%v", err)
		}
		got = append(got, '\n')

		goldenPath := filepath.Join("golden", base+".json")
		if update {
			if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
				t.Fatalf("写入 golden 失败：%v", err)
			}
			continue
		}

		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("读取 golden 失败：%s err=%v（可用 UPDATE_GOLDEN=1 生成）", goldenPath, err)
		}
		if string(want) != string(got) {
			t.Fatalf("golden 不匹配：%s（重新生成：UPDATE_GOLDEN=1 go test ./internal/provider/javdb）", goldenPath)
		}
	}
}

func canonicalURLFromHTML(t *testing.T, html []byte) string {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		t.Fatalf("解析 fixture HTML 失败：%v", err)
	}
	href, _ := doc.Find("link[rel='canonical']").First().Attr("href")
	return strings.TrimSpace(href)
}
