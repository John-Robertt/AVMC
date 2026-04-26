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

func TestParse_RejectsLoginPage(t *testing.T) {
	code, _ := domain.ParseCode("HEYZO-3831")
	html := []byte(`<html><head><title>Login</title></head><body><a href="/login">登入</a></body></html>`)

	_, err := Provider{}.Parse(code, html, "https://javdb.com/v/7ybBPd")
	if err == nil {
		t.Fatalf("期望登录页解析失败，实际成功")
	}
}

func TestParse_RejectsCodeMismatch(t *testing.T) {
	code, _ := domain.ParseCode("KUM-013")
	html, err := os.ReadFile(filepath.Join("testdata", "SNOS-052.html"))
	if err != nil {
		t.Fatalf("读取 fixture 失败：%v", err)
	}

	_, err = Provider{}.Parse(code, html, "https://javdb.com/v/ve39eW")
	if err == nil {
		t.Fatalf("期望番號不匹配时解析失败，实际成功")
	}
}

func TestParse_AllowsNARuntimeOnDetailPage(t *testing.T) {
	code, _ := domain.ParseCode("NHDTC-172")
	html := []byte(`
<html>
  <body>
    <h2 class="title"><span class="origin-title">N/A runtime fixture</span></h2>
    <nav class="movie-panel-info">
      <div class="panel-block"><strong>番號:</strong><span class="value">NHDTC-172</span></div>
      <div class="panel-block"><strong>時長:</strong><span class="value">N/A</span></div>
    </nav>
  </body>
</html>`)

	meta, err := Provider{}.Parse(code, html, "https://javdb.com/v/82ZONd")
	if err != nil {
		t.Fatalf("Parse 失败：%v", err)
	}
	if meta.RuntimeM != 0 {
		t.Fatalf("N/A 时长期望 RuntimeM=0，实际=%d", meta.RuntimeM)
	}
	if meta.Title == "" {
		t.Fatalf("真实详情页不应解析出空标题")
	}
}

func TestActorNamesFromPanel_FiltersMaleActors(t *testing.T) {
	html := []byte(`
<div class="panel-block">
  <strong>演員:</strong>
  <span class="value">
    <a href="/actors/f1">東実果</a><strong class="symbol female">♀</strong>
    <a href="/actors/m1">羽田貴史</a><strong class="symbol male">♂</strong>
    <a href="/actors/u1">未標記</a>
  </span>
</div>`)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		t.Fatalf("解析 HTML 失败：%v", err)
	}

	got := actorNamesFromPanel(doc.Find(".panel-block").First())
	want := []string{"東実果", "未標記"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("actorNamesFromPanel() = %v, want %v", got, want)
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

func TestParseRatingText(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantRating float64
		wantVotes  int
	}{
		{
			name:       "fixture format",
			in:         "4.39分, 由1158人評價",
			wantRating: 4.39,
			wantVotes:  1158,
		},
		{
			name:       "simplified evaluation",
			in:         "4.1分, 由99人评价",
			wantRating: 4.1,
			wantVotes:  99,
		},
		{
			name:       "spaces",
			in:         "4.3 分, 由 783 人評價",
			wantRating: 4.3,
			wantVotes:  783,
		},
		{
			name:       "thousands separator",
			in:         "4.39分, 由1,158人評價",
			wantRating: 4.39,
			wantVotes:  1158,
		},
		{
			name:       "empty",
			in:         "",
			wantRating: 0,
			wantVotes:  0,
		},
		{
			name:       "no rating",
			in:         "尚無評分",
			wantRating: 0,
			wantVotes:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRating, gotVotes := parseRatingText(tt.in)
			if gotRating != tt.wantRating || gotVotes != tt.wantVotes {
				t.Fatalf("parseRatingText(%q) = (%v, %d), want (%v, %d)", tt.in, gotRating, gotVotes, tt.wantRating, tt.wantVotes)
			}
		})
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
