package javbus

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/John-Robertt/AVMC/internal/domain"
	providerx "github.com/John-Robertt/AVMC/internal/provider"
)

// Provider 实现 JavBus 的页面抓取与 HTML 解析。
//
// 约束：
// - Fetch/Parse 不做缓存/重试/限速（由上层统一控制）
// - Parse 必须是纯函数（依赖输入 html + pageURL）
type Provider struct{}

func (Provider) Name() string { return "javbus" }

// Fetch 直接进入详情页：https://www.javbus.com/<CODE>
func (Provider) Fetch(ctx context.Context, code domain.Code, c *http.Client) ([]byte, string, error) {
	if c == nil {
		return nil, "", errors.New("http client 不能为空")
	}
	if code == "" {
		return nil, "", errors.New("code 不能为空")
	}

	pageURL := "https://www.javbus.com/" + url.PathEscape(string(code))
	// JavBus 在未通过“成年确认”时通常会返回 302 到 /doc/driver-verify，
	// 但很多情况下 302 的 body 仍然是完整详情页 HTML。
	//
	// Go 默认会自动跟随 302，最终拿到的是验证页 HTML（会导致解析失败/降级）。
	// 因此这里禁用重定向：直接读取 302 的 body 并解析。
	c2 := *c
	c2.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	b, err := fetchURL(ctx, &c2, pageURL)
	return b, pageURL, err
}

// Parse 把 JavBus 详情页 HTML 解析为最小可用 MovieMeta。
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

	// 先校验“是不是详情页”：识别码必须存在且匹配（避免把验证页/拦截页当成成功解析）。
	id := strings.TrimSpace(findInfoValueAny(doc, []string{"識別碼", "识别码", "ID"}))
	if id == "" {
		return domain.MovieMeta{}, errors.New("未找到識別碼（疑似返回了验证页/非详情页内容）")
	}
	if !strings.EqualFold(id, string(code)) {
		return domain.MovieMeta{}, errors.New("識別碼不匹配（疑似跳转/返回了其它页面）")
	}

	title := normSpace(doc.Find("h3").First().Text())
	if title == "" {
		return domain.MovieMeta{}, errors.New("标题为空（疑似返回了验证页/非详情页内容）")
	}
	if strings.HasPrefix(title, string(code)) {
		title = strings.TrimSpace(strings.TrimPrefix(title, string(code)))
	}

	release := findInfoValueAny(doc, []string{"發行日期", "发行日期", "Release Date", "発売日"})
	runtimeS := findInfoValueAny(doc, []string{"長度", "长度", "Length", "時長", "时长", "Duration"})
	runtimeM := firstInt(runtimeS)

	// “發行商”更像对外的厂牌标识；缺失时再回退“製作商”。
	studio := findInfoValueAny(doc, []string{"發行商", "发行商", "Label", "Publisher"})
	if studio == "" {
		studio = findInfoValueAny(doc, []string{"製作商", "制作商", "Studio", "Maker", "Manufacturer"})
	}

	series := findInfoValueAny(doc, []string{"系列", "Series"})

	actors := make([]string, 0, 8)
	doc.Find("div.star-name a").Each(func(_ int, s *goquery.Selection) {
		actors = append(actors, strings.TrimSpace(s.Text()))
	})
	actors = normList(actors)

	genres := parseKeywordTags(doc, code, studio, series)
	if len(genres) == 0 {
		// 兜底：keywords 缺失时回退从 /genre/ 链接提取（可能包含噪音标签）。
		doc.Find("a").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if strings.Contains(href, "/genre/") {
				genres = append(genres, strings.TrimSpace(s.Text()))
			}
		})
	}
	genres = normList(genres)

	coverURL := ""
	if href, ok := doc.Find("a.bigImage").First().Attr("href"); ok {
		coverURL = resolveURL(pageURL, href)
	}
	if coverURL == "" {
		if src, ok := doc.Find("div.screencap img").First().Attr("src"); ok {
			coverURL = resolveURL(pageURL, src)
		}
	}

	// fanart 采用背景大图（优先 cover），poster 由 fanart 右半边裁切得到。
	fanartURL := coverURL
	if fanartURL == "" {
		// 极端兜底：有些页面 cover 可能缺失，但仍有样品图。
		if href, ok := doc.Find("#sample-waterfall a.sample-box").First().Attr("href"); ok {
			fanartURL = resolveURL(pageURL, href)
		}
	}

	year := yearFromRelease(release)

	meta := domain.MovieMeta{
		Code:     code,
		Title:    title,
		Studio:   studio,
		Series:   series,
		Release:  release,
		Year:     year,
		RuntimeM: runtimeM,
		Actors:   actors,
		// JavBus 的「類別」更像标签/分类；为了兼容性同时写入 Genres 与 Tags。
		Genres:   genres,
		Tags:     genres,
		Website:  strings.TrimSpace(pageURL),
		CoverURL: coverURL,
		// 若无单独背景图，则回退为 cover（避免 apply 因 fanart 缺失而失败）。
		FanartURL: fanartURL,
	}
	return meta, nil
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

	// 先把 body 读出来：对 302（且 body=详情页）这种情况，必须先拿到内容才能判断。
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 若（未禁用重定向时）最终落在 driver-verify，或 body 本身就是验证页，则视为被拦截。
	if resp.Request != nil && resp.Request.URL != nil {
		if strings.Contains(resp.Request.URL.Path, "/doc/driver-verify") {
			return nil, &providerx.BlockedError{URL: resp.Request.URL.String(), Reason: "driver-verify"}
		}
	}
	loc := strings.TrimSpace(resp.Header.Get("Location"))
	if resp.StatusCode >= 300 && resp.StatusCode < 400 && strings.Contains(loc, "/doc/driver-verify") {
		// 只有当 body 明确是验证页时才算 blocked；否则允许解析 302 body（常见且可用）。
		if bytes.Contains(b, []byte("id=\"ageVerify\"")) || bytes.Contains(b, []byte("/doc/driver-verify")) {
			return nil, &providerx.BlockedError{URL: loc, Reason: "driver-verify"}
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, &providerx.HTTPStatusError{URL: u, StatusCode: resp.StatusCode, Location: loc}
	}
	if len(b) == 0 {
		return nil, errors.New("empty response body")
	}
	return b, nil
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

func findInfoValueAny(doc *goquery.Document, headers []string) string {
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
	doc.Find("div.movie div.info p").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		headerSel := s.Find("span.header").First()
		rawHeader := normSpace(headerSel.Text())
		h := normHeader(rawHeader)
		if _, ok := set[h]; !ok {
			return true
		}
		// 该 <p> 内除了 header，还有可能包含 <a>（如厂牌），或纯文本（日期/长度）。
		// 优先取 a 文本；否则取移除 header 后的剩余文本。
		if a := strings.TrimSpace(s.Find("a").First().Text()); a != "" {
			out = a
			return false
		}
		txt := normSpace(s.Text())
		txt = strings.TrimSpace(strings.TrimPrefix(txt, rawHeader))
		out = txt
		return false
	})
	return out
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

func parseKeywordTags(doc *goquery.Document, code domain.Code, studio, series string) []string {
	if doc == nil {
		return nil
	}
	content, ok := doc.Find("meta[name='keywords']").First().Attr("content")
	if !ok {
		return nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	// keywords 通常形如：
	// CODE,Studio,Series,Tag1,Tag2,...
	// 我们不做“聪明猜测”，只剔除已知的 code/studio/series，剩下的视为标签集合。
	out := make([]string, 0, 16)
	seen := make(map[string]struct{}, 32)
	for _, p := range strings.Split(content, ",") {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if strings.EqualFold(s, string(code)) {
			continue
		}
		if studio != "" && s == studio {
			continue
		}
		if series != "" && s == series {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func firstInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// 提取连续数字段，避免对 “155分鐘 / 160 分鍾” 这类字符串做复杂解析。
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
