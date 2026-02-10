package run

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Robertt/AVMC/internal/config"
	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/provider"
)

type stubProvider struct {
	name string
	meta domain.MovieMeta
}

func (p stubProvider) Name() string { return p.name }

func (p stubProvider) Fetch(ctx context.Context, code domain.Code, c *http.Client) ([]byte, string, error) {
	return []byte("<html/>"), "https://example.test/detail/" + string(code), nil
}

func (p stubProvider) Parse(code domain.Code, html []byte, pageURL string) (domain.MovieMeta, error) {
	m := p.meta
	m.Code = code
	return m, nil
}

func TestExecute_DryRun_NoWrites(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	code, _ := domain.ParseCode("CAWD-895")
	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{Title: "T", CoverURL: "https://img.test/p.jpg", FanartURL: "https://img.test/f.jpg"}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	rr := Execute(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       false,
		Concurrency: 1,
	}, reg)

	if _, err := os.Stat(filepath.Join(root, "out")); !os.IsNotExist(err) {
		t.Fatalf("dry-run 不应创建 out/，但 Stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cache")); !os.IsNotExist(err) {
		t.Fatalf("dry-run 不应创建 cache/，但 Stat err=%v", err)
	}
	if _, err := os.Stat(in); err != nil {
		t.Fatalf("dry-run 不应移动视频，但源文件不存在：%v", err)
	}

	if rr.Summary.Failed != 0 || rr.Summary.Unmatched != 0 {
		t.Fatalf("不期望失败：summary=%+v items=%+v", rr.Summary, rr.Items)
	}
	if len(rr.Items) != 1 {
		t.Fatalf("期望 1 个 item，实际 %d", len(rr.Items))
	}
	it := rr.Items[0]
	if it.Code != string(code) || it.Status != domain.StatusProcessed {
		t.Fatalf("item 不符合预期：%+v", it)
	}
	if it.ProviderUsed != "javbus" || it.Website == "" {
		t.Fatalf("dry-run NeedScrape=true 时应验证 provider：%+v", it)
	}
	if len(it.Files) != 1 || it.Files[0].Status != domain.FileStatusPlanned || it.Files[0].Dst == "" {
		t.Fatalf("files 不符合预期：%+v", it.Files)
	}
}

func TestExecute_Apply_WritesSidecarsAndMoves(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	fanartW, fanartH := 200, 100
	fanartBytes := mustFanartJPEG(t, fanartW, fanartH)

	img := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fanart.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(fanartBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer img.Close()

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{
			Title:     "T",
			FanartURL: img.URL + "/fanart.jpg",
		}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	rr := Execute(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       true,
		Concurrency: 1,
		ImageProxy:  false,
	}, reg)

	outDir := filepath.Join(root, "out", "CAWD-895")
	if _, err := os.Stat(filepath.Join(outDir, "CAWD-895.nfo")); err != nil {
		t.Fatalf("期望写出 NFO：%v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "poster.jpg")); err != nil {
		t.Fatalf("期望写出 poster：%v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "fanart.jpg")); err != nil {
		t.Fatalf("期望写出 fanart：%v", err)
	}

	// poster 是 fanart 右半边裁切：尺寸应为 w/2 x h，且中心像素应偏白。
	pb, err := os.ReadFile(filepath.Join(outDir, "poster.jpg"))
	if err != nil {
		t.Fatalf("读取 poster 失败：%v", err)
	}
	pi, err := jpeg.Decode(bytes.NewReader(pb))
	if err != nil {
		t.Fatalf("解码 poster 失败：%v", err)
	}
	b := pi.Bounds()
	if b.Dx() != fanartW/2 || b.Dy() != fanartH {
		t.Fatalf("poster 尺寸不符合预期：got=%dx%d want=%dx%d", b.Dx(), b.Dy(), fanartW/2, fanartH)
	}
	c := color.RGBAModel.Convert(pi.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2)).(color.RGBA)
	if c.R < 200 || c.G < 200 || c.B < 200 {
		t.Fatalf("poster 裁切区域不符合预期：中心像素=%v（期望接近白色）", c)
	}

	if _, err := os.Stat(filepath.Join(outDir, "CAWD-895.mp4")); err != nil {
		t.Fatalf("期望移动视频到 out/：%v", err)
	}
	if _, err := os.Stat(in); !os.IsNotExist(err) {
		t.Fatalf("期望源视频被移动，但 Stat err=%v", err)
	}

	// cache 应写入（providers/<p>/<code>.html/.json）
	if _, err := os.Stat(filepath.Join(root, "cache", "providers", "javbus", "CAWD-895.html")); err != nil {
		t.Fatalf("期望写出 html cache：%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cache", "providers", "javbus", "CAWD-895.json")); err != nil {
		t.Fatalf("期望写出 json cache：%v", err)
	}

	if rr.Summary.Failed != 0 || rr.Summary.Unmatched != 0 {
		t.Fatalf("不期望失败：summary=%+v items=%+v", rr.Summary, rr.Items)
	}
	if len(rr.Items) != 1 || len(rr.Items[0].Files) != 1 || rr.Items[0].Files[0].Status != domain.FileStatusMoved {
		t.Fatalf("report files 状态不正确：%+v", rr.Items)
	}
}

func mustFanartJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// 左黑右白，方便验证裁切是否取右半边。
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				img.Set(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("生成 fanart jpeg 失败：%v", err)
	}
	return buf.Bytes()
}
