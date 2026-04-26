package javdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/John-Robertt/AVMC/internal/domain"
	providerx "github.com/John-Robertt/AVMC/internal/provider"
)

// Provider 实现 JavDB 的页面抓取与 HTML 解析。
//
// 约束：
// - JavDB 需要先搜索再进入详情页（不能直接拼详情 URL）
// - Fetch/Parse 不做缓存/重试/限速（由上层统一控制）
// - Parse 必须是纯函数（依赖输入 html + pageURL）
type Provider struct {
	// BaseURL 允许指定 JavDB 的可用域名（例如 javdb565.com），用于绕过区域不可达。
	// 为空时使用默认的 https://javdb.com。
	BaseURL string
}

func (Provider) Name() string { return "javdb" }

func (p Provider) baseURL() string {
	u := strings.TrimSpace(p.BaseURL)
	if u == "" {
		return "https://javdb.com"
	}
	return strings.TrimRight(u, "/")
}

// Fetch 先搜索再进入详情页：
// https://javdb.com/search?q=<CODE>&f=all
func (p Provider) Fetch(ctx context.Context, code domain.Code, c *http.Client) ([]byte, string, error) {
	if c == nil {
		return nil, "", errors.New("http client 不能为空")
	}
	if code == "" {
		return nil, "", errors.New("code 不能为空")
	}

	base := p.baseURL()
	searchURL := base + "/search?q=" + url.QueryEscape(string(code)) + "&f=all"
	searchHTML, err := fetchURL(ctx, c, searchURL)
	if err != nil {
		return nil, "", err
	}

	href, err := findDetailHref(searchHTML, code)
	if err != nil {
		return nil, "", err
	}

	pageURL := resolveURL(base+"/", href)
	b, err := fetchURL(ctx, c, pageURL)
	return b, pageURL, err
}

// Parse 把 JavDB 详情页 HTML 解析为最小可用 MovieMeta。
func (Provider) Parse(code domain.Code, html []byte, pageURL string) (domain.MovieMeta, error) {
	if code == "" {
		return domain.MovieMeta{}, errors.New("code 不能为空")
	}
	if len(html) == 0 {
		return domain.MovieMeta{}, errors.New("html 为空")
	}
	if strings.TrimSpace(pageURL) == "" {
		return domain.MovieMeta{}, errors.New("pageURL 不能为空")
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return domain.MovieMeta{}, err
	}

	info := doc.Find("nav.movie-panel-info").First()
	if info.Length() == 0 {
		return domain.MovieMeta{}, errors.New("未找到详情信息区（疑似返回了登录页/非详情页内容）")
	}
	pageCode := normalizeCodeText(findPanelValueAny(info, []string{"番號", "番号", "ID", "Code"}))
	if pageCode == "" {
		return domain.MovieMeta{}, errors.New("未找到番號（疑似返回了登录页/非详情页内容）")
	}
	if !strings.EqualFold(pageCode, string(code)) {
		return domain.MovieMeta{}, errors.New("番號不匹配（疑似搜索结果/详情页返回了其它影片）")
	}

	// JavDB 的标题有时会显示中文翻译（current-title），同时提供隐藏的 origin-title。
	// 需求：优先使用“原标题”（origin-title），不存在时回退 current-title。
	//
	// 注意：goquery 不会执行 CSS/JS，因此即使 origin-title 是 display:none，Text() 仍可读到文本。
	title := normSpace(strings.TrimSpace(doc.Find("h2.title span.origin-title").First().Text()))
	if title == "" {
		title = normSpace(strings.TrimSpace(doc.Find("h2.title strong.current-title").First().Text()))
	}
	if title == "" {
		return domain.MovieMeta{}, errors.New("标题为空（疑似返回了登录页/非详情页内容）")
	}

	var (
		release  string
		runtimeM int
		director string
		studio   string
		label    string
		series   string
		actors   []string
		tags     []string
		rating   float64
		votes    int
	)

	info.Find(".panel-block").Each(func(_ int, s *goquery.Selection) {
		h := normHeader(s.Find("strong").First().Text())
		switch h {
		case "日期", "Date":
			release = strings.TrimSpace(s.Find("span.value").First().Text())
		case "時長", "时长", "Length", "Duration":
			runtimeM = firstInt(s.Find("span.value").First().Text())
		case "導演", "导演", "Director":
			director = strings.TrimSpace(s.Find("span.value a").First().Text())
		case "片商", "Maker", "Studio", "Manufacturer":
			studio = strings.TrimSpace(s.Find("span.value a").First().Text())
		case "發行", "发行", "Publisher", "Label":
			label = strings.TrimSpace(s.Find("span.value a").First().Text())
		case "系列", "Series":
			series = strings.TrimSpace(s.Find("span.value a").First().Text())
		case "演員", "演员", "Actor", "Actors", "Actress", "Cast":
			actors = append(actors, actorNamesFromPanel(s)...)
		case "類別", "类别", "Tag", "Tags", "Genre", "Genres", "Category", "Categories":
			s.Find("span.value a").Each(func(_ int, a *goquery.Selection) {
				tags = append(tags, strings.TrimSpace(a.Text()))
			})
		case "評分", "评分", "Rating", "Score":
			rating, votes = parseRatingText(s.Find("span.value").First().Text())
		}
	})

	actors = normList(actors)
	tags = normList(tags)

	coverURL := ""
	if href, ok := doc.Find(".column-video-cover a[data-fancybox='gallery']").First().Attr("href"); ok {
		coverURL = strings.TrimSpace(href)
	}
	if coverURL == "" {
		if src, ok := doc.Find(".column-video-cover img.video-cover").First().Attr("src"); ok {
			coverURL = strings.TrimSpace(src)
		}
	}

	// fanart 采用背景大图（这里直接复用 cover），poster 由 fanart 右半边裁切得到。
	fanartURL := coverURL

	year := yearFromRelease(release)

	meta := domain.MovieMeta{
		Code:      code,
		Title:     title,
		Director:  director,
		Studio:    studio,
		Label:     label,
		Series:    series,
		Release:   release,
		Year:      year,
		RuntimeM:  runtimeM,
		Rating:    rating,
		Votes:     votes,
		Actors:    actors,
		Genres:    tags,
		Tags:      tags,
		Website:   strings.TrimSpace(pageURL),
		CoverURL:  coverURL,
		FanartURL: fanartURL,
	}
	return meta, nil
}

func findPanelValueAny(info *goquery.Selection, headers []string) string {
	if info == nil {
		return ""
	}
	set := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		h = normHeader(h)
		if h == "" {
			continue
		}
		set[h] = struct{}{}
	}
	if len(set) == 0 {
		return ""
	}

	var out string
	info.Find(".panel-block").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		h := normHeader(s.Find("strong").First().Text())
		if _, ok := set[h]; !ok {
			return true
		}
		out = strings.TrimSpace(s.Find("span.value").First().Text())
		return false
	})
	return out
}

func actorNamesFromPanel(s *goquery.Selection) []string {
	if s == nil {
		return nil
	}
	actors := make([]string, 0, 4)
	s.Find("span.value a").Each(func(_ int, a *goquery.Selection) {
		name := strings.TrimSpace(a.Text())
		if name == "" {
			return
		}
		if actorSymbolGender(a.NextFiltered("strong.symbol")) == "male" {
			return
		}
		actors = append(actors, name)
	})
	return actors
}

func actorSymbolGender(symbol *goquery.Selection) string {
	if symbol == nil || symbol.Length() == 0 {
		return ""
	}
	className, _ := symbol.Attr("class")
	className = strings.ToLower(className)
	text := strings.TrimSpace(symbol.Text())
	switch {
	case strings.Contains(className, "female") || text == "♀":
		return "female"
	case strings.Contains(className, "male") || text == "♂":
		return "male"
	default:
		return ""
	}
}

func normalizeCodeText(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\u00a0' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func findDetailHref(searchHTML []byte, code domain.Code) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(searchHTML))
	if err != nil {
		return "", err
	}

	want := strings.ToUpper(string(code))

	var (
		href string
		ok   bool
	)
	doc.Find("div.movie-list div.item a.box").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		got := strings.TrimSpace(s.Find("div.video-title strong").First().Text())
		got = strings.ToUpper(got)
		if got != want {
			return true
		}
		href, ok = s.Attr("href")
		return false
	})
	if !ok || strings.TrimSpace(href) == "" {
		return "", fmt.Errorf("搜索结果中未找到匹配的详情页：%s", want)
	}
	return href, nil
}

func fetchURL(ctx context.Context, c *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &providerx.HTTPStatusError{URL: u, StatusCode: resp.StatusCode, Location: resp.Header.Get("Location")}
	}
	return io.ReadAll(resp.Body)
}

func resolveURL(base, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	bu, err := url.Parse(base)
	if err != nil {
		return href
	}
	ru, err := url.Parse(href)
	if err != nil {
		return href
	}
	return bu.ResolveReference(ru).String()
}

var (
	reRatingValue = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*分`)
	reRatingVotes = regexp.MustCompile(`由\s*([0-9][0-9,\s，]*)\s*人`)
)

func parseRatingText(s string) (float64, int) {
	var rating float64
	var votes int
	if m := reRatingValue.FindStringSubmatch(s); len(m) == 2 {
		rating, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := reRatingVotes.FindStringSubmatch(s); len(m) == 2 {
		voteText := strings.ReplaceAll(m[1], ",", "")
		voteText = strings.ReplaceAll(voteText, "，", "")
		voteText = strings.Join(strings.Fields(voteText), "")
		votes, _ = strconv.Atoi(voteText)
	}
	return rating, votes
}

func normSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func normHeader(s string) string {
	s = normSpace(s)
	s = strings.TrimSuffix(s, ":")
	s = strings.TrimSuffix(s, "：")
	return strings.TrimSpace(s)
}

func normList(in []string) []string {
	m := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := m[s]; ok {
			continue
		}
		m[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func firstInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			break
		}
	}
	if b.Len() == 0 {
		return 0
	}
	n, _ := strconv.Atoi(b.String())
	return n
}

func yearFromRelease(release string) int {
	release = strings.TrimSpace(release)
	if release == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", release)
	if err != nil {
		return 0
	}
	return t.Year()
}
