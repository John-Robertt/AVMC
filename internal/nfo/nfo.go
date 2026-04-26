package nfo

import (
	"encoding/xml"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

const (
	DefaultCountry = "JP"
	DefaultMPAA    = "R18+"
)

type movie struct {
	XMLName xml.Name `xml:"movie"`

	Title     string `xml:"title"`
	SortTitle string `xml:"sorttitle"`

	Studios  []string `xml:"studio,omitempty"`
	Director string   `xml:"director,omitempty"`

	Set *movieSet `xml:"set,omitempty"`

	Premiered string `xml:"premiered,omitempty"`
	Year      int    `xml:"year,omitempty"`
	Runtime   int    `xml:"runtime,omitempty"`

	MPAA    string `xml:"mpaa,omitempty"`
	Country string `xml:"country,omitempty"`

	Thumbs []thumb `xml:"thumb,omitempty"`
	Fanart *fanart `xml:"fanart,omitempty"`

	Ratings *ratings `xml:"ratings,omitempty"`

	Actors []actor  `xml:"actor,omitempty"`
	Tags   []string `xml:"tag,omitempty"`
	Genres []string `xml:"genre,omitempty"`

	UniqueIDs []uniqueID `xml:"uniqueid,omitempty"`
}

type movieSet struct {
	Name string `xml:"name"`
}

type thumb struct {
	Aspect string `xml:"aspect,attr,omitempty"`
	URL    string `xml:",chardata"`
}

type fanart struct {
	Thumbs []thumb `xml:"thumb"`
}

type ratings struct {
	Rating []ratingEntry `xml:"rating"`
}

type ratingEntry struct {
	Name    string  `xml:"name,attr"`
	Max     int     `xml:"max,attr"`
	Default bool    `xml:"default,attr,omitempty"`
	Value   float64 `xml:"value"`
	Votes   int     `xml:"votes"`
}

type uniqueID struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr,omitempty"`
	ID      string `xml:",chardata"`
}

type actor struct {
	Name  string `xml:"name"`
	Role  string `xml:"role,omitempty"`
	Order int    `xml:"order"`
}

// Encode 把 MovieMeta 转成 Kodi/Jellyfin/Emby 可读取的 NFO（XML）。
func Encode(meta domain.MovieMeta) ([]byte, error) {
	code := strings.TrimSpace(string(meta.Code))
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = code
	} else if code != "" && !strings.HasPrefix(title, code) {
		title = code + " " + title
	}

	actors := normList(meta.Actors)

	m := movie{
		Title:     title,
		SortTitle: code,

		Studios:  normList([]string{meta.Studio, meta.Label}),
		Director: strings.TrimSpace(meta.Director),

		Premiered: strings.TrimSpace(meta.Release),
		Year:      meta.Year,
		Runtime:   meta.RuntimeM,

		MPAA:    DefaultMPAA,
		Country: DefaultCountry,

		Tags:   normList(append(append([]string(nil), meta.Tags...), actors...)),
		Genres: normList(meta.Genres),
	}

	if s := strings.TrimSpace(meta.Series); s != "" {
		m.Set = &movieSet{Name: s}
	}

	if w := strings.TrimSpace(meta.Website); w != "" {
		m.UniqueIDs = append(m.UniqueIDs, uniqueID{Type: "url", ID: w})
	}

	if c := strings.TrimSpace(meta.CoverURL); c != "" {
		m.Thumbs = append(m.Thumbs, thumb{Aspect: "poster", URL: c})
	}
	if f := strings.TrimSpace(meta.FanartURL); f != "" {
		m.Fanart = &fanart{Thumbs: []thumb{{URL: f}}}
	}

	if meta.Rating > 0 || meta.Votes > 0 {
		m.Ratings = &ratings{Rating: []ratingEntry{{
			Name: "javdb", Max: 5, Default: true,
			Value: meta.Rating, Votes: meta.Votes,
		}}}
	}

	if len(actors) > 0 {
		m.Actors = make([]actor, 0, len(actors))
		for i, a := range actors {
			m.Actors = append(m.Actors, actor{Name: a, Role: a, Order: i})
		}
	}

	b, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
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
