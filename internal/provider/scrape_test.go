package provider

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/John-Robertt/AVMC/internal/domain"
)

type stubProvider struct {
	name string

	fetchErr error
	parseErr error

	html []byte
	url  string
	meta domain.MovieMeta

	fetchCalls int
	parseCalls int
}

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) Fetch(ctx context.Context, code domain.Code, c *http.Client) ([]byte, string, error) {
	p.fetchCalls++
	if p.fetchErr != nil {
		return nil, "", p.fetchErr
	}
	return p.html, p.url, nil
}

func (p *stubProvider) Parse(code domain.Code, html []byte, pageURL string) (domain.MovieMeta, error) {
	p.parseCalls++
	if p.parseErr != nil {
		return domain.MovieMeta{}, p.parseErr
	}
	m := p.meta
	m.Code = code
	return m, nil
}

func TestFetchParse_FallbackOnFetchFail(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")

	javbus := &stubProvider{name: "javbus", fetchErr: errors.New("nope")}
	javdb := &stubProvider{name: "javdb", html: []byte("<html/>"), url: "https://example.test/javdb/1", meta: domain.MovieMeta{Title: "t"}}

	reg, err := NewRegistry(javbus, javdb)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	meta, used, website, _, err := FetchParse(context.Background(), reg, "javbus", code, nil)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if used != "javdb" {
		t.Fatalf("期望 used=javdb，实际=%q", used)
	}
	if website != javdb.url {
		t.Fatalf("期望 website=%q，实际=%q", javdb.url, website)
	}
	if meta.Website != javdb.url {
		t.Fatalf("期望 meta.Website=%q，实际=%q", javdb.url, meta.Website)
	}
	if meta.Code != code {
		t.Fatalf("期望 meta.Code=%q，实际=%q", code, meta.Code)
	}
}

func TestFetchParseTrace_RecordsFallbackReason(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")

	javbus := &stubProvider{name: "javbus", fetchErr: errors.New("nope")}
	javdb := &stubProvider{name: "javdb", html: []byte("<html/>"), url: "https://example.test/javdb/1", meta: domain.MovieMeta{Title: "t"}}

	reg, err := NewRegistry(javbus, javdb)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	_, used, _, _, attempts, err := FetchParseTrace(context.Background(), reg, "javbus", code, nil)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if used != "javdb" {
		t.Fatalf("期望 used=javdb，实际=%q", used)
	}
	if len(attempts) != 2 {
		t.Fatalf("期望 2 条 attempts，实际 %d: %+v", len(attempts), attempts)
	}
	if attempts[0].Provider != "javbus" || attempts[0].Stage != "fetch" || attempts[0].Err == nil {
		t.Fatalf("attempt[0] 不符合预期：%+v", attempts[0])
	}
	if attempts[1].Provider != "javdb" || attempts[1].Stage != "ok" || attempts[1].Err != nil {
		t.Fatalf("attempt[1] 不符合预期：%+v", attempts[1])
	}
}

func TestFetchParse_FallbackOnParseFail(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")

	javdb := &stubProvider{name: "javdb", html: []byte("<bad/>"), url: "https://example.test/javdb/1", parseErr: errors.New("parse fail")}
	javbus := &stubProvider{name: "javbus", html: []byte("<ok/>"), url: "https://example.test/javbus/1", meta: domain.MovieMeta{Title: "ok"}}

	reg, err := NewRegistry(javdb, javbus)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	meta, used, _, _, err := FetchParse(context.Background(), reg, "javdb", code, nil)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if used != "javbus" {
		t.Fatalf("期望 used=javbus，实际=%q", used)
	}
	if meta.Title != "ok" {
		t.Fatalf("期望 meta.Title=ok，实际=%q", meta.Title)
	}
}

func TestFetchParse_UnknownProvider(t *testing.T) {
	code, _ := domain.ParseCode("CAWD-895")

	reg, err := NewRegistry(&stubProvider{name: "javbus"})
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	_, _, _, _, err = FetchParse(context.Background(), reg, "nope", code, nil)
	if err == nil {
		t.Fatalf("期望错误，但得到 nil")
	}
}
