package nfo

import (
	"encoding/xml"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

type movieOut struct {
	Title     string   `xml:"title"`
	SortTitle string   `xml:"sorttitle"`
	Num       string   `xml:"num"`
	Studio    string   `xml:"studio"`
	Set       string   `xml:"set"`
	Release   string   `xml:"release"`
	Premiered string   `xml:"premiered"`
	Year      int      `xml:"year"`
	Runtime   int      `xml:"runtime"`
	MPAA      string   `xml:"mpaa"`
	Country   string   `xml:"country"`
	Poster    string   `xml:"poster"`
	Thumb     string   `xml:"thumb"`
	Fanart    string   `xml:"fanart"`
	Rating    int      `xml:"rating"`
	UserRate  int      `xml:"userrating"`
	Votes     int      `xml:"votes"`
	Website   string   `xml:"website"`
	Tags      []string `xml:"tag"`
	Genres    []string `xml:"genre"`
	Cover     string   `xml:"cover"`
	Actors    []struct {
		Name string `xml:"name"`
		Role string `xml:"role"`
	} `xml:"actor"`
}

func TestEncode_XMLRoundTripAndDeterministicLists(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")
	meta := domain.MovieMeta{
		Code:     code,
		Title:    "Title",
		Studio:   "Studio",
		Series:   "Series",
		Release:  "2025-01-02",
		Year:     2025,
		RuntimeM: 120,
		Actors:   []string{"b", "a", "a", " "},
		Genres:   []string{"z", "x", "x"},
		Tags:     []string{"t2", "t1"},
		Website:  "https://example.test/page",
		CoverURL: "https://img.test/cover.jpg",
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
	if out.SortTitle != "CAWD-895" || out.Num != "CAWD-895" {
		t.Fatalf("sorttitle/num 不一致：%q %q", out.SortTitle, out.Num)
	}
	if out.Country != DefaultCountry || out.MPAA != DefaultMPAA {
		t.Fatalf("country/mpaa 不一致：%q %q", out.Country, out.MPAA)
	}
	if out.Poster != "poster.jpg" || out.Thumb != "poster.jpg" || out.Fanart != "fanart.jpg" {
		t.Fatalf("poster/thumb/fanart 不一致：%q %q %q", out.Poster, out.Thumb, out.Fanart)
	}
	if out.Rating != 0 || out.UserRate != 0 || out.Votes != 0 {
		t.Fatalf("rating/userrating/votes 不一致：%d %d %d", out.Rating, out.UserRate, out.Votes)
	}
	if len(out.Actors) != 2 || out.Actors[0].Name != "b" || out.Actors[1].Name != "a" || out.Actors[0].Role != "b" || out.Actors[1].Role != "a" {
		t.Fatalf("actors 未去重且 role 应与 name 相同：%v", out.Actors)
	}
	// tags/genres 会追加 actors（便于媒体库按人名过滤）。
	if len(out.Tags) != 4 || out.Tags[0] != "t2" || out.Tags[1] != "t1" || out.Tags[2] != "b" || out.Tags[3] != "a" {
		t.Fatalf("tags 未按输入顺序去重并追加 actors：%v", out.Tags)
	}
	if len(out.Genres) != 4 || out.Genres[0] != "z" || out.Genres[1] != "x" || out.Genres[2] != "b" || out.Genres[3] != "a" {
		t.Fatalf("genres 未按输入顺序去重并追加 actors：%v", out.Genres)
	}
	if out.Cover != meta.CoverURL {
		t.Fatalf("cover 不一致：%q", out.Cover)
	}
	if out.Website != meta.Website {
		t.Fatalf("website 不一致：%q", out.Website)
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
