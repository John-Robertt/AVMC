package nfo

import (
	"encoding/xml"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

const (
	// DefaultCountry / DefaultMPAA 不对外暴露配置；保持最小但够用。
	DefaultCountry = "JP"
	DefaultMPAA    = "R18+"
)

type movie struct {
	XMLName xml.Name `xml:"movie"`

	Title     string `xml:"title"`
	SortTitle string `xml:"sorttitle"`
	Num       string `xml:"num"`

	Studio string `xml:"studio,omitempty"`
	Set    string `xml:"set,omitempty"`

	Release   string `xml:"release,omitempty"`
	Premiered string `xml:"premiered,omitempty"`
	Year      int    `xml:"year,omitempty"`
	Runtime   int    `xml:"runtime,omitempty"`

	MPAA    string `xml:"mpaa,omitempty"`
	Country string `xml:"country,omitempty"`

	Poster string `xml:"poster,omitempty"`
	Thumb  string `xml:"thumb,omitempty"`
	Fanart string `xml:"fanart,omitempty"`

	Rating     int `xml:"rating"`
	UserRating int `xml:"userrating"`
	Votes      int `xml:"votes"`

	Actors []actor  `xml:"actor,omitempty"`
	Tags   []string `xml:"tag,omitempty"`
	Genres []string `xml:"genre,omitempty"`

	Cover   string `xml:"cover,omitempty"`
	Website string `xml:"website,omitempty"`
}

type actor struct {
	Name string `xml:"name"`
	Role string `xml:"role,omitempty"`
}

// Encode 把 MovieMeta 转成 Kodi/Jellyfin/Emby 可读取的 NFO（XML）。
//
// 规则：
// - 字段缺失允许为空；但输出结构尽量稳定（去空白、去重、保持输入顺序）
// - title 为空时回退到 CODE（避免生成空 title）
func Encode(meta domain.MovieMeta) ([]byte, error) {
	code := strings.TrimSpace(string(meta.Code))
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = code
	} else if code != "" && !strings.HasPrefix(title, code) {
		// 约定：title 以 CODE 开头（更利于媒体库识别与展示）。
		title = code + " " + title
	}

	m := movie{
		Title:     title,
		SortTitle: code,
		Num:       code,

		Studio: strings.TrimSpace(meta.Studio),
		Set:    strings.TrimSpace(meta.Series),

		Release:   strings.TrimSpace(meta.Release),
		Premiered: strings.TrimSpace(meta.Release),
		Year:      meta.Year,
		Runtime:   meta.RuntimeM,

		MPAA:    DefaultMPAA,
		Country: DefaultCountry,

		Poster: "poster.jpg",
		Thumb:  "poster.jpg",
		Fanart: "fanart.jpg",

		Rating:     0,
		UserRating: 0,
		Votes:      0,

		Tags:   normList(append(meta.Tags, meta.Actors...)),
		Genres: normList(append(meta.Genres, meta.Actors...)),

		Cover:   strings.TrimSpace(meta.CoverURL),
		Website: strings.TrimSpace(meta.Website),
	}

	actors := normList(meta.Actors)
	if len(actors) > 0 {
		m.Actors = make([]actor, 0, len(actors))
		for _, a := range actors {
			m.Actors = append(m.Actors, actor{Name: a, Role: a})
		}
	}

	b, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	// 约定：输出带 standalone="yes" 的 XML 头，便于与常见刮削器产物兼容。
	const header = `<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>` + "\n"
	return append([]byte(header), b...), nil
}

func normList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
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
	if len(out) == 0 {
		return nil
	}
	return out
}
