package run

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/John-Robertt/AVMC/internal/app"
	"github.com/John-Robertt/AVMC/internal/app/planner"
	"github.com/John-Robertt/AVMC/internal/config"
	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/infra/cache"
	"github.com/John-Robertt/AVMC/internal/infra/fsx"
	"github.com/John-Robertt/AVMC/internal/infra/httpx"
	"github.com/John-Robertt/AVMC/internal/infra/imgx"
	"github.com/John-Robertt/AVMC/internal/nfo"
	"github.com/John-Robertt/AVMC/internal/provider"
	"github.com/John-Robertt/AVMC/internal/scan"
)

type runDeps struct {
	newMetaClient  func(proxyURL string) (*http.Client, error)
	newImageClient func(proxyURL string, imageProxy bool) (*http.Client, error)
	rename         func(src, dst string) error
}

func defaultRunDeps() runDeps {
	return runDeps{
		newMetaClient:  httpx.NewMetaClient,
		newImageClient: httpx.NewImageClient,
		rename:         fsx.Rename,
	}
}

func (d runDeps) withDefaults() runDeps {
	defaults := defaultRunDeps()
	if d.newMetaClient == nil {
		d.newMetaClient = defaults.newMetaClient
	}
	if d.newImageClient == nil {
		d.newImageClient = defaults.newImageClient
	}
	if d.rename == nil {
		d.rename = defaults.rename
	}
	return d
}

// Execute 执行一次 run（dry-run/apply），并返回对外稳定的 RunReport。
// 该函数尽量把错误“降级”为 item 级失败（单条失败不影响其他）。
func Execute(ctx context.Context, eff config.EffectiveConfig, reg provider.Registry) domain.RunReport {
	return ExecuteWithObserver(ctx, eff, reg, nil)
}

// ExecuteWithObserver 与 Execute 相同，但允许传入 Observer 以输出进度/阶段信息（由上层决定是否启用）。
func ExecuteWithObserver(ctx context.Context, eff config.EffectiveConfig, reg provider.Registry, obs Observer) domain.RunReport {
	return executeWithObserverDeps(ctx, eff, reg, obs, defaultRunDeps())
}

func executeWithObserverDeps(ctx context.Context, eff config.EffectiveConfig, reg provider.Registry, obs Observer, deps runDeps) domain.RunReport {
	deps = deps.withDefaults()
	started := time.Now().UTC()

	if obs != nil {
		obs.OnStart(eff)
	}

	rr := domain.RunReport{
		Path:      eff.Path,
		DryRun:    !eff.Apply,
		StartedAt: started,
		Items:     make([]domain.ItemResult, 0, 128),
	}

	metaClient, err := deps.newMetaClient(eff.ProxyURL)
	if err != nil {
		rr.Items = append(rr.Items, syntheticFailed(domain.ErrCodeConfigInvalid, fmt.Sprintf("proxy.url 无效：%v", err)))
		rr.FinishedAt = time.Now().UTC()
		rr.Finalize()
		return rr
	}

	var imageClient *http.Client
	if eff.Apply {
		ic, e := deps.newImageClient(eff.ProxyURL, eff.ImageProxy)
		if e != nil {
			rr.Items = append(rr.Items, syntheticFailed(domain.ErrCodeConfigInvalid, e.Error()))
			rr.FinishedAt = time.Now().UTC()
			rr.Finalize()
			return rr
		}
		imageClient = ic
	}

	store := cache.New(eff.Path, !eff.Apply)

	scanStarted := time.Now()
	files, err := scan.ScanVideos(eff.Path, eff.ExcludeDirs)
	if err != nil {
		rr.Items = append(rr.Items, syntheticFailed(domain.ErrCodeIOFailed, fmt.Sprintf("扫描失败：%v", err)))
		rr.FinishedAt = time.Now().UTC()
		rr.Finalize()
		return rr
	}
	scanDur := time.Since(scanStarted)

	absToRel := make(map[string]string, len(files))
	for i := range files {
		absToRel[files[i].AbsPath] = files[i].RelPath
	}

	groupStarted := time.Now()
	items, unmatched, err := app.GroupByCode(files)
	if err != nil {
		rr.Items = append(rr.Items, syntheticFailed(domain.ErrCodeIOFailed, fmt.Sprintf("分组失败：%v", err)))
		rr.FinishedAt = time.Now().UTC()
		rr.Finalize()
		return rr
	}
	groupDur := time.Since(groupStarted)

	if obs != nil {
		// 输出按文档约定：scan 行同时展示 files + unmatched（unmatched 来自分组阶段）。
		obs.OnPhaseDone("scan", map[string]any{
			"files":     len(files),
			"unmatched": len(unmatched),
		}, scanDur)
		obs.OnPhaseDone("group", map[string]any{
			"codes": len(items),
		}, groupDur)
	}

	// unmatched：每个输入文件单独形成一条 item（更可解释，便于用户逐个修复）。
	for _, u := range unmatched {
		rr.Items = append(rr.Items, unmatchedItem(u))
	}

	planStarted := time.Now()
	plans := make([]domain.ItemPlan, 0, len(items))
	for _, it := range items {
		st, e := planner.ReadOutState(eff.Path, it.Code)
		if e != nil {
			errCode := domain.ErrCodeIOFailed
			if planner.IsTargetConflict(e) {
				errCode = domain.ErrCodeTargetConflict
			}
			rr.Items = append(rr.Items, failedPlanItem(eff.Provider, it, files, absToRel, errCode, fmt.Sprintf("读取 out 状态失败：%v", e)))
			continue
		}
		p, e := planner.PlanItem(eff.Provider, files, it, st)
		if e != nil {
			errCode := domain.ErrCodeIOFailed
			if planner.IsTargetConflict(e) {
				errCode = domain.ErrCodeTargetConflict
			}
			rr.Items = append(rr.Items, failedPlanItem(eff.Provider, it, files, absToRel, errCode, fmt.Sprintf("规划失败：%v", e)))
			continue
		}
		plans = append(plans, p)
	}
	planDur := time.Since(planStarted)

	if obs != nil {
		var needScrape, needNFO, needFanart, needPoster, moves int
		for i := range plans {
			p := plans[i]
			if p.Need.NeedsScrape() {
				needScrape++
			}
			if p.Need.NeedNFO {
				needNFO++
			}
			if p.Need.NeedFanart {
				needFanart++
			}
			if p.Need.NeedPoster {
				needPoster++
			}
			moves += len(p.Moves)
		}

		obs.OnPhaseDone("plan", map[string]any{
			"items":       len(plans),
			"need_scrape": needScrape,
			"need_nfo":    needNFO,
			"need_fanart": needFanart,
			"need_poster": needPoster,
			"moves":       moves,
		}, planDur)
	}

	// 执行阶段：按 CODE 并发（worker pool），item 内串行。
	workers := eff.Concurrency
	if workers < 1 {
		workers = 1
	}

	if obs != nil {
		obs.OnPhaseDone("exec", map[string]any{
			"workers":     workers,
			"total_items": len(plans),
		}, 0)
	}

	type execResult struct {
		code domain.Code
		res  domain.ItemResult
		dur  time.Duration
	}

	jobs := make(chan domain.ItemPlan)
	results := make(chan execResult, len(plans))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				oneStarted := time.Now()
				r := execOne(ctx, eff, p, reg, metaClient, imageClient, store, absToRel, deps)
				results <- execResult{
					code: p.Code,
					res:  r,
					dur:  time.Since(oneStarted),
				}
			}
		}()
	}

	go func() {
		for _, p := range plans {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	done := 0
	for it := range results {
		done++
		rr.Items = append(rr.Items, it.res)
		if obs != nil {
			obs.OnItemDone(done, len(plans), it.code, it.res, it.dur)
		}
	}

	rr.FinishedAt = time.Now().UTC()
	rr.Finalize()
	return rr
}

func unmatchedItem(u domain.Unmatched) domain.ItemResult {
	item := domain.ItemResult{
		Code:              "",
		ProviderRequested: "",
		ProviderUsed:      "",
		Website:           "",
		Status:            domain.StatusUnmatched,
		ErrorCode:         domain.ErrCodeUnmatchedCode,
		Candidates:        []string{},
		Attempts:          []domain.ProviderAttempt{},
		Files: []domain.FileResult{{
			Src:    u.File.RelPath,
			Dst:    "",
			Status: domain.FileStatusFailed,
		}},
	}

	switch u.Kind {
	case domain.UnmatchedAmbiguous:
		item.Candidates = make([]string, 0, len(u.Candidates))
		for _, c := range u.Candidates {
			item.Candidates = append(item.Candidates, string(c))
		}
		item.ErrorMsg = fmt.Sprintf("解析到多个不同 CODE（ambiguous）：%v；请重命名文件/目录使其只包含一个 CODE", item.Candidates)
	default:
		item.ErrorMsg = "无法从文件名或父目录解析出 CODE；请确保文件名包含类似 CAWD-895 的片段"
	}
	return item
}

func failedPlanItem(providerRequested string, it domain.WorkItem, files []domain.VideoFile, absToRel map[string]string, code, msg string) domain.ItemResult {
	out := domain.ItemResult{
		Code:              string(it.Code),
		ProviderRequested: providerRequested,
		Status:            domain.StatusFailed,
		ErrorCode:         code,
		ErrorMsg:          msg,
		Candidates:        []string{},
		Attempts:          []domain.ProviderAttempt{},
		Files:             make([]domain.FileResult, 0, len(it.FileIdx)),
	}
	for _, idx := range it.FileIdx {
		if idx < 0 || idx >= len(files) {
			continue
		}
		src := files[idx].RelPath
		if src == "" {
			src = absToRel[files[idx].AbsPath]
		}
		out.Files = append(out.Files, domain.FileResult{Src: src, Dst: "", Status: domain.FileStatusFailed})
	}
	return out
}

func syntheticFailed(code, msg string) domain.ItemResult {
	return domain.ItemResult{
		Code:              "",
		ProviderRequested: "",
		ProviderUsed:      "",
		Website:           "",
		Status:            domain.StatusFailed,
		ErrorCode:         code,
		ErrorMsg:          msg,
		Candidates:        []string{},
		Attempts:          []domain.ProviderAttempt{},
		Files:             []domain.FileResult{},
	}
}

func execOne(ctx context.Context, eff config.EffectiveConfig, p domain.ItemPlan, reg provider.Registry, metaClient, imageClient *http.Client, store cache.Store, absToRel map[string]string, deps runDeps) domain.ItemResult {
	deps = deps.withDefaults()
	item := domain.ItemResult{
		Code:              string(p.Code),
		ProviderRequested: p.ProviderRequested,
		ProviderUsed:      "",
		Website:           "",
		Status:            domain.StatusProcessed, // 失败时覆盖
		ErrorCode:         "",
		ErrorMsg:          "",
		Candidates:        []string{},
		Attempts:          []domain.ProviderAttempt{},
		Files:             buildFileResults(eff, p, absToRel),
	}

	needSidecar := p.Need.NeedNFO || p.Need.NeedPoster || p.Need.NeedFanart
	if !needSidecar && len(p.Moves) == 0 {
		item.Status = domain.StatusSkipped
		return item
	}

	// dry-run：只做 fetch+parse 验证；不落盘、不下载图片、不移动。
	if !eff.Apply {
		if p.Need.NeedsScrape() {
			meta, used, website, html, attempts, err := scrape(ctx, store, reg, p.ProviderRequested, p.Code, metaClient, false)
			_ = meta
			_ = html
			item.Attempts = attempts
			if err != nil {
				fillProviderError(&item, err)
				return item
			}
			item.ProviderUsed = used
			item.Website = website
		}
		return item
	}

	// apply：严格遵守“移动最后一步”。
	var meta domain.MovieMeta
	var used, website string
	var html []byte
	if p.Need.NeedsScrape() {
		m, u, w, h, attempts, err := scrape(ctx, store, reg, p.ProviderRequested, p.Code, metaClient, true)
		item.Attempts = attempts
		if err != nil {
			fillProviderError(&item, err)
			// sidecar 未满足：禁止移动视频（文件状态保持 failed）
			for i := range item.Files {
				item.Files[i].Status = domain.FileStatusFailed
			}
			return item
		}
		meta, used, website, html = m, u, w, h
		item.ProviderUsed = used
		item.Website = website
		_ = html
	}

	outDir := filepath.Join(eff.Path, "out", string(p.Code))
	if err := ensureDir(outDir); err != nil {
		item.Status = domain.StatusFailed
		if fsx.IsPathTypeConflict(err) {
			item.ErrorCode = domain.ErrCodeTargetConflict
		} else {
			item.ErrorCode = domain.ErrCodeIOFailed
		}
		item.ErrorMsg = err.Error()
		for i := range item.Files {
			item.Files[i].Status = domain.FileStatusFailed
		}
		return item
	}

	// sidecar 写入（原子 + 不覆盖）。任何失败都禁止 move。
	if p.Need.NeedNFO {
		b, err := nfo.Encode(meta)
		if err != nil {
			item.Status = domain.StatusFailed
			item.ErrorCode = domain.ErrCodeIOFailed
			item.ErrorMsg = fmt.Sprintf("生成 NFO 失败：%v", err)
			failAllFiles(&item)
			return item
		}
		if err := fsx.WriteFileAtomicNoOverwrite(outDir, string(p.Code)+".nfo", b); err != nil {
			if errors.Is(err, os.ErrExist) {
				// 已存在视为满足
			} else if fsx.IsPathTypeConflict(err) {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeTargetConflict
				item.ErrorMsg = err.Error()
				failAllFiles(&item)
				return item
			} else {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeIOFailed
				item.ErrorMsg = fmt.Sprintf("写入 NFO 失败：%v", err)
				failAllFiles(&item)
				return item
			}
		}
	}

	var fanartBytes []byte

	if p.Need.NeedFanart {
		if stringsTrim(meta.FanartURL) == "" {
			item.Status = domain.StatusFailed
			item.ErrorCode = domain.ErrCodeParseFailed
			item.ErrorMsg = "provider 未提供 fanart_url，无法下载 fanart.jpg"
			failAllFiles(&item)
			return item
		}
		b, err := download(ctx, imageClient, meta.FanartURL, imageRequestConfigurer(reg, used, meta))
		if err != nil {
			item.Status = domain.StatusFailed
			item.ErrorCode = domain.ErrCodeFetchFailed
			item.ErrorMsg = fmt.Sprintf("下载 fanart 失败：%v", err)
			failAllFiles(&item)
			return item
		}
		fanartBytes = b
		if err := fsx.WriteFileAtomicNoOverwrite(outDir, "fanart.jpg", b); err != nil {
			if errors.Is(err, os.ErrExist) {
				// ok
			} else if fsx.IsPathTypeConflict(err) {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeTargetConflict
				item.ErrorMsg = err.Error()
				failAllFiles(&item)
				return item
			} else {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeIOFailed
				item.ErrorMsg = fmt.Sprintf("写入 fanart 失败：%v", err)
				failAllFiles(&item)
				return item
			}
		}
	}

	// poster 由 fanart 的右半边裁切得到；不再单独下载 cover。
	if p.Need.NeedPoster {
		src := fanartBytes
		if len(src) == 0 {
			// fanart 已存在：从本地读取（apply 才会走到这里）。
			b, err := os.ReadFile(filepath.Join(outDir, "fanart.jpg"))
			if err != nil {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeIOFailed
				item.ErrorMsg = fmt.Sprintf("读取 fanart 失败，无法生成 poster：%v", err)
				failAllFiles(&item)
				return item
			}
			src = b
		}

		b, err := imgx.PosterFromFanartRightHalfJPEG(src)
		if err != nil {
			item.Status = domain.StatusFailed
			item.ErrorCode = domain.ErrCodeIOFailed
			item.ErrorMsg = fmt.Sprintf("生成 poster 失败：%v", err)
			failAllFiles(&item)
			return item
		}
		if err := fsx.WriteFileAtomicNoOverwrite(outDir, "poster.jpg", b); err != nil {
			if errors.Is(err, os.ErrExist) {
				// ok
			} else if fsx.IsPathTypeConflict(err) {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeTargetConflict
				item.ErrorMsg = err.Error()
				failAllFiles(&item)
				return item
			} else {
				item.Status = domain.StatusFailed
				item.ErrorCode = domain.ErrCodeIOFailed
				item.ErrorMsg = fmt.Sprintf("写入 poster 失败：%v", err)
				failAllFiles(&item)
				return item
			}
		}
	}

	// move：最后一步。中途失败 => 尝试回滚已移动文件。
	moved := make([]domain.MovePlan, 0, len(p.Moves))
	for i := range p.Moves {
		mv := p.Moves[i]
		if err := deps.rename(mv.SrcAbs, mv.DstAbs); err != nil {
			item.Status = domain.StatusFailed
			item.ErrorCode = domain.ErrCodeMoveFailed
			item.ErrorMsg = err.Error()

			// 失败文件标记 failed；之前成功的尝试回滚。
			item.Files[i].Status = domain.FileStatusFailed
			rollbackMoves(&item, moved, deps.rename)
			return item
		}

		moved = append(moved, mv)
		item.Files[i].Status = domain.FileStatusMoved
	}

	return item
}

func buildFileResults(eff config.EffectiveConfig, p domain.ItemPlan, absToRel map[string]string) []domain.FileResult {
	out := make([]domain.FileResult, 0, len(p.Moves))
	for _, mv := range p.Moves {
		src := absToRel[mv.SrcAbs]
		if src == "" {
			// 兜底：尽量输出相对路径；失败则输出原始 abs（至少可追溯）。
			if rel, err := filepath.Rel(eff.Path, mv.SrcAbs); err == nil {
				src = rel
			} else {
				src = mv.SrcAbs
			}
		}

		dst := ""
		if rel, err := filepath.Rel(eff.Path, mv.DstAbs); err == nil {
			dst = rel
		} else {
			dst = mv.DstAbs
		}

		out = append(out, domain.FileResult{
			Src:    src,
			Dst:    dst,
			Status: domain.FileStatusPlanned,
		})
	}
	return out
}

func failAllFiles(item *domain.ItemResult) {
	for i := range item.Files {
		item.Files[i].Status = domain.FileStatusFailed
	}
}

func rollbackMoves(item *domain.ItemResult, moved []domain.MovePlan, rename func(src, dst string) error) {
	// 回滚顺序：倒序（更符合栈语义）。
	for i := len(moved) - 1; i >= 0; i-- {
		mv := moved[i]
		if err := rename(mv.DstAbs, mv.SrcAbs); err == nil {
			// moved[i] 对应 p.Moves[i]，file 结果顺序一致。
			item.Files[i].Status = domain.FileStatusRolledBack
		} else {
			item.Files[i].Status = domain.FileStatusFailed
		}
	}
}

func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if err == nil {
		if fi.IsDir() {
			return nil
		}
		return &fsx.PathTypeConflictError{Path: dir, Want: "dir", Got: "file"}
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func stringsTrim(s string) string { return strings.TrimSpace(s) }
