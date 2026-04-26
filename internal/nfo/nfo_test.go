package nfo

import (
	"encoding/xml"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

type movieOut struct {
	Title     string   `xml:"title"`
	SortTitle string   `xml:"sorttitle"`
	Studios   []string `xml:"studio"`
	Director  string   `xml:"director"`
	Set       struct {
		Name string `xml:"name"`
	} `xml:"set"`
	Premiered string `xml:"premiered"`
	Year      int    `xml:"year"`
	Runtime   int    `xml:"runtime"`
	MPAA      string `xml:"mpaa"`
	Country   string `xml:"country"`
	Thumbs    []struct {
		Aspect string `xml:"aspect,attr"`
		URL    string `xml:",chardata"`
	} `xml:"thumb"`
	Fanart struct {
		Thumbs []struct {
			URL string `xml:",chardata"`
		} `xml:"thumb"`
	} `xml:"fanart"`
	Ratings struct {
		Rating []struct {
			Name    string  `xml:"name,attr"`
			Max     int     `xml:"max,attr"`
			Default bool    `xml:"default,attr"`
			Value   float64 `xml:"value"`
			Votes   int     `xml:"votes"`
		} `xml:"rating"`
	} `xml:"ratings"`
	Tags      []string `xml:"tag"`
	Genres    []string `xml:"genre"`
	UniqueIDs []struct {
		Type    string `xml:"type,attr"`
		Default bool   `xml:"default,attr"`
		ID      string `xml:",chardata"`
	} `xml:"uniqueid"`
	Actors []struct {
		Name  string `xml:"name"`
		Role  string `xml:"role"`
		Order int    `xml:"order"`
	} `xml:"actor"`
}

func TestEncode_XMLRoundTripAndDeterministicLists(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")
	meta := domain.MovieMeta{
		Code:      code,
		Title:     "Title",
		Director:  "Dir",
		Studio:    "Studio",
		Label:     "Label",
		Series:    "Series",
		Release:   "2025-01-02",
		Year:      2025,
		RuntimeM:  120,
		Rating:    4.5,
		Votes:     200,
		Actors:    []string{"b", "a", "a", " "},
		Genres:    []string{"z", "x", "x"},
		Tags:      []string{"t2", "t1"},
		Website:   "https://example.test/page",
		CoverURL:  "https://img.test/cover.jpg",
		FanartURL: "https://img.test/fanart.jpg",
	}

	b, err := Encode(meta)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	var out movieOut
	if err := xml.Unmarshal(b, &out); err != nil {
		t.Fatalf("xml.Unmarshal 失败：%v", err)
	}

	if out.Title != "CAWD-895 Title" {
		t.Fatalf("title 不一致：%q", out.Title)
	}
	if out.SortTitle != "CAWD-895" {
		t.Fatalf("sorttitle 不一致：%q", out.SortTitle)
	}
	if len(out.Studios) != 2 || out.Studios[0] != "Studio" || out.Studios[1] != "Label" {
		t.Fatalf("studio 不一致：%v", out.Studios)
	}
	if out.Director != "Dir" {
		t.Fatalf("director 不一致：%q", out.Director)
	}
	if out.Set.Name != "Series" {
		t.Fatalf("set/name 不一致：%q", out.Set.Name)
	}
	if out.Country != DefaultCountry || out.MPAA != DefaultMPAA {
		t.Fatalf("country/mpaa 不一致：%q %q", out.Country, out.MPAA)
	}

	// thumb aspect="poster" → CoverURL
	if len(out.Thumbs) != 1 || out.Thumbs[0].Aspect != "poster" || out.Thumbs[0].URL != meta.CoverURL {
		t.Fatalf("thumb 不一致：%+v", out.Thumbs)
	}
	// fanart 容器
	if len(out.Fanart.Thumbs) != 1 || out.Fanart.Thumbs[0].URL != meta.FanartURL {
		t.Fatalf("fanart 不一致：%+v", out.Fanart)
	}

	// ratings 容器
	if len(out.Ratings.Rating) != 1 {
		t.Fatalf("ratings 数量不一致：%d", len(out.Ratings.Rating))
	}
	r := out.Ratings.Rating[0]
	if r.Name != "javdb" || r.Max != 5 || !r.Default || r.Value != 4.5 || r.Votes != 200 {
		t.Fatalf("rating 不一致：%+v", r)
	}

	// actors 去重
	if len(out.Actors) != 2 || out.Actors[0].Name != "b" || out.Actors[1].Name != "a" || out.Actors[0].Role != "b" || out.Actors[1].Role != "a" {
		t.Fatalf("actors 未去重且 role 应与 name 相同：%v", out.Actors)
	}
	if out.Actors[0].Order != 0 || out.Actors[1].Order != 1 {
		t.Fatalf("actor order 不一致：%d %d", out.Actors[0].Order, out.Actors[1].Order)
	}

	// tags 追加 actors；genres 不追加 actors
	if len(out.Tags) != 4 || out.Tags[0] != "t2" || out.Tags[1] != "t1" || out.Tags[2] != "b" || out.Tags[3] != "a" {
		t.Fatalf("tags 未按输入顺序去重并追加 actors：%v", out.Tags)
	}
	if len(out.Genres) != 2 || out.Genres[0] != "z" || out.Genres[1] != "x" {
		t.Fatalf("genres 应只包含分类标签不含 actors：%v", out.Genres)
	}

	// uniqueid
	if len(out.UniqueIDs) != 1 {
		t.Fatalf("uniqueid 数量不一致：%d", len(out.UniqueIDs))
	}
	if out.UniqueIDs[0].Type != "url" || out.UniqueIDs[0].ID != meta.Website {
		t.Fatalf("uniqueid[0] 不一致：%+v", out.UniqueIDs[0])
	}
}

func TestEncode_TitleFallbackToCode(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")
	b, err := Encode(domain.MovieMeta{Code: code})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	var out movieOut
	if err := xml.Unmarshal(b, &out); err != nil {
		t.Fatalf("xml.Unmarshal 失败：%v", err)
	}
	if out.Title != "CAWD-895" {
		t.Fatalf("期望 title 回退到 CODE，实际=%q", out.Title)
	}
}

func TestEncode_NoRatingsWhenZero(t *testing.T) {
	code, _ := domain.ParseCode("TEST-001")
	b, err := Encode(domain.MovieMeta{Code: code})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	var out movieOut
	if err := xml.Unmarshal(b, &out); err != nil {
		t.Fatalf("xml.Unmarshal 失败：%v", err)
	}
	if len(out.Ratings.Rating) != 0 {
		t.Fatalf("rating/votes 为零时不应输出 ratings 块：%+v", out.Ratings)
	}
}

func TestEncode_StudioUsesLabelWithoutDuplicates(t *testing.T) {
	code, _ := domain.ParseCode("TEST-001")
	b, err := Encode(domain.MovieMeta{Code: code, Studio: " Studio ", Label: "Studio"})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	var out movieOut
	if err := xml.Unmarshal(b, &out); err != nil {
		t.Fatalf("xml.Unmarshal 失败：%v", err)
	}
	if len(out.Studios) != 1 || out.Studios[0] != "Studio" {
		t.Fatalf("studio 应去空白并去重：%v", out.Studios)
	}
}

func TestEncode_StudioFallsBackToLabel(t *testing.T) {
	code, _ := domain.ParseCode("TEST-001")
	b, err := Encode(domain.MovieMeta{Code: code, Label: "Label"})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	var out movieOut
	if err := xml.Unmarshal(b, &out); err != nil {
		t.Fatalf("xml.Unmarshal 失败：%v", err)
	}
	if len(out.Studios) != 1 || out.Studios[0] != "Label" {
		t.Fatalf("studio 应在 Studio 为空时使用 Label：%v", out.Studios)
	}
}
