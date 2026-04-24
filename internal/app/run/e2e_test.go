package run

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

type countingProvider struct {
	name       string
	meta       domain.MovieMeta
	fetchCalls int
}

func (p *countingProvider) Name() string { return p.name }

func (p *countingProvider) Fetch(ctx context.Context, code domain.Code, c *http.Client) ([]byte, string, error) {
	p.fetchCalls++
	return []byte("<html/>"), "https://example.test/detail/" + string(code), nil
}

func (p *countingProvider) Parse(code domain.Code, html []byte, pageURL string) (domain.MovieMeta, error) {
	m := p.meta
	m.Code = code
	return m, nil
}

type imageHeaderProvider struct {
	stubProvider
}

func (p imageHeaderProvider) PrepareImageRequest(req *http.Request, meta domain.MovieMeta) {
	req.Header.Set("Referer", meta.Website)
	req.Header.Set("Cookie", "age=verified")
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
		t.Fatalf("dry-run NeedsScrape()=true 时应验证 provider：%+v", it)
	}
	if len(it.Files) != 1 || it.Files[0].Status != domain.FileStatusPlanned || it.Files[0].Dst == "" {
		t.Fatalf("files 不符合预期：%+v", it.Files)
	}
}

func TestExecute_Apply_TargetConflictWhenSidecarIsDirectory(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建输入目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "out", "CAWD-895", "poster.jpg"), 0o755); err != nil {
		t.Fatalf("创建 sidecar 冲突目录失败：%v", err)
	}

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{Title: "T"}},
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
	}, reg)

	if rr.Summary.Failed != 1 {
		t.Fatalf("期望 failed=1，实际 summary=%+v items=%+v", rr.Summary, rr.Items)
	}
	if len(rr.Items) != 1 || rr.Items[0].ErrorCode != domain.ErrCodeTargetConflict {
		t.Fatalf("期望 target_conflict，实际 items=%+v", rr.Items)
	}
	if _, err := os.Stat(in); err != nil {
		t.Fatalf("发生 target_conflict 时不应移动视频：%v", err)
	}
}

func TestExecute_Apply_TargetConflictWhenOutRootIsFile(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建输入目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "out"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("写入 out 文件失败：%v", err)
	}

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{Title: "T"}},
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
	}, reg)

	if rr.Summary.Failed != 1 {
		t.Fatalf("期望 failed=1，实际 summary=%+v items=%+v", rr.Summary, rr.Items)
	}
	if len(rr.Items) != 1 || rr.Items[0].ErrorCode != domain.ErrCodeTargetConflict {
		t.Fatalf("期望 target_conflict，实际 items=%+v", rr.Items)
	}
	if _, err := os.Stat(in); err != nil {
		t.Fatalf("发生 target_conflict 时不应移动视频：%v", err)
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

	imageURL := "https://image.test/fanart.jpg"
	imageClient := staticImageClient(t, fanartBytes, func(req *http.Request) {
		if req.URL.String() != imageURL {
			t.Fatalf("图片 URL 不符合预期：%s", req.URL.String())
		}
	})

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{
			Title:     "T",
			FanartURL: imageURL,
		}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	rr := executeWithObserverDeps(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       true,
		Concurrency: 1,
		ImageProxy:  false,
	}, reg, nil, testDeps(imageClient, nil))

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

func TestExecute_Apply_GeneratesPosterFromExistingFanartWithoutScrape(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建输入目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	outDir := filepath.Join(root, "out", "CAWD-895")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("创建输出目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "CAWD-895.nfo"), []byte("<movie/>"), 0o644); err != nil {
		t.Fatalf("写入 NFO 失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "fanart.jpg"), mustFanartJPEG(t, 200, 100), 0o644); err != nil {
		t.Fatalf("写入 fanart 失败：%v", err)
	}

	javbus := &countingProvider{name: "javbus", meta: domain.MovieMeta{Title: "T"}}
	javdb := &countingProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}}
	reg, err := provider.NewRegistry(javbus, javdb)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	rr := Execute(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       true,
		Concurrency: 1,
	}, reg)

	if rr.Summary.Failed != 0 || rr.Summary.Unmatched != 0 {
		t.Fatalf("不期望失败：summary=%+v items=%+v", rr.Summary, rr.Items)
	}
	if javbus.fetchCalls != 0 || javdb.fetchCalls != 0 {
		t.Fatalf("仅缺 poster 时不应抓 provider：javbus=%d javdb=%d", javbus.fetchCalls, javdb.fetchCalls)
	}
	if _, err := os.Stat(filepath.Join(outDir, "poster.jpg")); err != nil {
		t.Fatalf("期望从已有 fanart 生成 poster：%v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "CAWD-895.mp4")); err != nil {
		t.Fatalf("期望移动视频到 out/：%v", err)
	}
}

func TestExecute_Apply_UsesProviderImageRequestHeaders(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建输入目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	fanartBytes := mustFanartJPEG(t, 200, 100)
	imageURL := "https://image.test/fanart.jpg"
	imageClient := staticImageClient(t, fanartBytes, func(r *http.Request) {
		if r.URL.String() != imageURL {
			t.Fatalf("图片 URL 不符合预期：%s", r.URL.String())
		}
		if r.Header.Get("Referer") != "https://example.test/detail/CAWD-895" {
			t.Fatalf("Referer 不符合预期：%q", r.Header.Get("Referer"))
		}
		if r.Header.Get("Cookie") != "age=verified" {
			t.Fatalf("Cookie 不符合预期：%q", r.Header.Get("Cookie"))
		}
	})

	reg, err := provider.NewRegistry(
		imageHeaderProvider{stubProvider: stubProvider{name: "javbus", meta: domain.MovieMeta{
			Title:     "T",
			FanartURL: imageURL,
		}}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	rr := executeWithObserverDeps(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       true,
		Concurrency: 1,
	}, reg, nil, testDeps(imageClient, nil))

	if rr.Summary.Failed != 0 {
		t.Fatalf("期望 provider header 策略生效，实际 summary=%+v items=%+v", rr.Summary, rr.Items)
	}
}

func TestExecute_Apply_RollsBackMovedFilesWhenLaterMoveFails(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"CAWD-895-a.mp4", "CAWD-895-b.mp4"} {
		path := filepath.Join(root, "in", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("创建输入目录失败：%v", err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("写入视频失败：%v", err)
		}
	}

	outDir := filepath.Join(root, "out", "CAWD-895")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("创建输出目录失败：%v", err)
	}
	for _, name := range []string{"CAWD-895.nfo", "poster.jpg", "fanart.jpg"} {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("写入 sidecar 失败：%v", err)
		}
	}

	forwardMoves := 0
	var renameCalls []moveCall
	rename := func(src, dst string) error {
		renameCalls = append(renameCalls, moveCall{src: src, dst: dst})
		if strings.HasPrefix(src, outDir+string(filepath.Separator)) {
			return nil
		}
		forwardMoves++
		if forwardMoves == 2 {
			return errors.New("simulated move failure")
		}
		return nil
	}

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{Title: "T"}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	rr := executeWithObserverDeps(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       true,
		Concurrency: 1,
	}, reg, nil, testDeps(nil, rename))

	if rr.Summary.Failed != 1 || len(rr.Items) != 1 {
		t.Fatalf("期望单条失败：summary=%+v items=%+v", rr.Summary, rr.Items)
	}
	files := rr.Items[0].Files
	if len(files) != 2 {
		t.Fatalf("期望 2 个文件结果，实际=%+v", files)
	}
	if files[0].Status != domain.FileStatusRolledBack || files[1].Status != domain.FileStatusFailed {
		t.Fatalf("rollback 状态不符合预期：%+v", files)
	}
	if len(renameCalls) != 3 {
		t.Fatalf("rename 调用次数不符合预期：%+v", renameCalls)
	}
	wantRollback := moveCall{
		src: filepath.Join(outDir, "CAWD-895-a.mp4"),
		dst: filepath.Join(root, "in", "CAWD-895-a.mp4"),
	}
	if renameCalls[2] != wantRollback {
		t.Fatalf("rollback 方向不符合预期：got=%+v want=%+v calls=%+v", renameCalls[2], wantRollback, renameCalls)
	}
}

type moveCall struct {
	src string
	dst string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testDeps(imageClient *http.Client, rename func(src, dst string) error) runDeps {
	deps := runDeps{
		newMetaClient: func(proxyURL string) (*http.Client, error) {
			return http.DefaultClient, nil
		},
		newImageClient: func(proxyURL string, imageProxy bool) (*http.Client, error) {
			if imageClient != nil {
				return imageClient, nil
			}
			return http.DefaultClient, nil
		},
	}
	if rename != nil {
		deps.rename = rename
	}
	return deps
}

func staticImageClient(t *testing.T, body []byte, validate func(*http.Request)) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if validate != nil {
			validate(req)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    req,
		}, nil
	})}
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
