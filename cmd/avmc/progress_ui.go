package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/John-Robertt/AVMC/internal/app/run"
	"github.com/John-Robertt/AVMC/internal/config"
	"github.com/John-Robertt/AVMC/internal/domain"
)

var _ run.Observer = (*progressUI)(nil)

// progressUI 是一个“简洁版”的交互终端进度输出。
//
// 设计目标：
// - 所有过程信息写到 stderr（或 fallback 到 stdout），不污染 stdout 的 JSON 输出契约
// - 事件驱动：run 层只发事件，CLI 决定如何展示
// - keepalive：长时间无条目完成时也会定期输出一行，降低等待焦虑
type progressUI struct {
	w io.Writer

	mu          sync.Mutex
	startedAt   time.Time
	lastPrinted time.Time

	workers int
	total   int
	done    int
	ok      int
	fail    int
	skip    int

	keepaliveThreshold time.Duration
	tickerInterval     time.Duration

	stopCh        chan struct{}
	tickerStarted bool
}

func newProgressUI(w io.Writer) *progressUI {
	return &progressUI{
		w:                  w,
		keepaliveThreshold: 6 * time.Second,
		tickerInterval:     2 * time.Second,
	}
}

func (p *progressUI) OnStart(eff config.EffectiveConfig) {
	now := time.Now()

	p.mu.Lock()
	if p.startedAt.IsZero() {
		p.startedAt = now
	}

	mode := "dry-run"
	modeHint := " (不写入/不下载/不移动)"
	if eff.Apply {
		mode = "apply"
		modeHint = ""
	}

	fmt.Fprintf(p.w, "[%s] AVMC run (%s)\n", now.Format("15:04:05"), mode)
	fmt.Fprintln(p.w, "配置（生效）:")
	fmt.Fprintf(p.w, "  path: %s\n", eff.Path)
	fmt.Fprintf(p.w, "  mode: %s%s\n", mode, modeHint)
	fmt.Fprintf(p.w, "  provider: %s\n", providerChain(eff.Provider))
	fmt.Fprintf(p.w, "  concurrency: %d\n", eff.Concurrency)
	fmt.Fprintf(p.w, "  proxy: %s\n", formatProxy(eff.ProxyURL))
	fmt.Fprintf(p.w, "  image_proxy: %s\n", onOff(eff.ImageProxy))
	if strings.TrimSpace(eff.JavDBBaseURL) != "" {
		fmt.Fprintf(p.w, "  javdb_base_url: %s\n", truncate(eff.JavDBBaseURL, 120))
	}
	fmt.Fprintf(p.w, "  exclude_dirs: %s + 固定排除 out/, cache/\n", formatStringListJSON(eff.ExcludeDirs))

	fmt.Fprintln(p.w, "输出:")
	fmt.Fprintf(p.w, "  out: %s\n", filepath.Join(eff.Path, "out"))
	fmt.Fprintf(p.w, "  cache: %s\n", filepath.Join(eff.Path, "cache"))
	if eff.Apply {
		fmt.Fprintf(p.w, "  report: %s\n", filepath.Join(eff.Path, "cache", "report.json"))
	}
	fmt.Fprintln(p.w)

	p.lastPrinted = time.Now()
	p.mu.Unlock()
}

func (p *progressUI) OnPhaseDone(name string, fields map[string]any, dur time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch name {
	case "scan":
		fmt.Fprintf(p.w, "扫描: files=%d unmatched=%d (%s)\n",
			intField(fields, "files"), intField(fields, "unmatched"), formatShortDuration(dur),
		)
	case "group":
		fmt.Fprintf(p.w, "分组: codes=%d (%s)\n",
			intField(fields, "codes"), formatShortDuration(dur),
		)
	case "plan":
		fmt.Fprintf(p.w, "规划: items=%d need_scrape=%d need_nfo=%d need_fanart=%d need_poster=%d moves=%d (%s)\n",
			intField(fields, "items"),
			intField(fields, "need_scrape"),
			intField(fields, "need_nfo"),
			intField(fields, "need_fanart"),
			intField(fields, "need_poster"),
			intField(fields, "moves"),
			formatShortDuration(dur),
		)
	case "exec":
		p.workers = intField(fields, "workers")
		p.total = intField(fields, "total_items")
		fmt.Fprintf(p.w, "执行: workers=%d total_items=%d\n\n", p.workers, p.total)
		if p.total > 0 && !p.tickerStarted {
			p.startTickerLocked()
		}
	default:
		// 兜底：未知阶段也不要静默（便于调试/演进）。
		fmt.Fprintf(p.w, "%s (%s)\n", name, formatShortDuration(dur))
	}

	p.lastPrinted = time.Now()
}

func (p *progressUI) OnItemDone(idx, total int, code domain.Code, res domain.ItemResult, dur time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// idx/total 由 run 层给出；这里同时维护自己的计数，供 keepalive 使用。
	p.done = idx
	p.total = total

	switch res.Status {
	case domain.StatusProcessed:
		p.ok++
	case domain.StatusFailed:
		p.fail++
	case domain.StatusSkipped:
		p.skip++
	}

	status := strings.ToUpper(res.Status)
	switch res.Status {
	case domain.StatusProcessed:
		status = "OK"
	case domain.StatusSkipped:
		status = "SKIP"
	case domain.StatusFailed:
		status = "FAIL"
	}

	prov := strings.TrimSpace(res.ProviderUsed)
	if prov == "" {
		prov = strings.TrimSpace(res.ProviderRequested)
	}

	moveCount := len(res.Files)

	fallbackNote := ""
	if res.Status == domain.StatusProcessed {
		fallbackNote = formatFallbackNote(res)
	}

	switch res.Status {
	case domain.StatusFailed:
		chain := formatAttemptChain(res.Attempts, 1)
		if chain != "" {
			chain = " attempts=" + chain
		}
		fmt.Fprintf(p.w, "[%d/%d] %s %s %s: %s%s (%s)\n",
			idx, total, code, status, res.ErrorCode, truncate(res.ErrorMsg, 160), chain, formatShortDuration(dur),
		)
	case domain.StatusSkipped:
		fmt.Fprintf(p.w, "[%d/%d] %s %s (已完整，无需刮削/移动) (%s)\n",
			idx, total, code, status, formatShortDuration(dur),
		)
	default:
		if prov != "" {
			fmt.Fprintf(p.w, "[%d/%d] %s %s provider=%s move=%d%s (%s)\n",
				idx, total, code, status, prov, moveCount, fallbackNote, formatShortDuration(dur),
			)
		} else {
			fmt.Fprintf(p.w, "[%d/%d] %s %s move=%d (%s)\n",
				idx, total, code, status, moveCount, formatShortDuration(dur),
			)
		}
	}

	p.lastPrinted = time.Now()

	// 最后一条完成：停止 ticker，避免在结束打印后又冒出 keepalive。
	if p.tickerStarted && p.done >= p.total {
		close(p.stopCh)
		p.tickerStarted = false
	}
}

func (p *progressUI) OnProgress(done, total, ok, fail, skip, active int, activeCodes []string, elapsed time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.w, "进度: done=%d/%d ok=%d fail=%d skip=%d active=%d elapsed=%s\n",
		done, total, ok, fail, skip, active, formatElapsed(elapsed),
	)
	p.lastPrinted = time.Now()
}

func (p *progressUI) startTickerLocked() {
	p.stopCh = make(chan struct{})
	p.tickerStarted = true

	interval := p.tickerInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	threshold := p.keepaliveThreshold
	if threshold <= 0 {
		threshold = 6 * time.Second
	}

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				p.mu.Lock()
				// 已完成：安全退出（OnItemDone 会 close stopCh，但这里也做兜底）。
				if p.total > 0 && p.done >= p.total {
					p.mu.Unlock()
					return
				}

				if p.total > 0 && time.Since(p.lastPrinted) > threshold {
					active := p.workers
					remain := p.total - p.done
					if remain < active {
						active = remain
					}
					elapsed := time.Since(p.startedAt)
					fmt.Fprintf(p.w, "进度: done=%d/%d ok=%d fail=%d skip=%d active=%d elapsed=%s\n",
						p.done, p.total, p.ok, p.fail, p.skip, active, formatElapsed(elapsed),
					)
					p.lastPrinted = time.Now()
				}
				p.mu.Unlock()
			case <-p.stopCh:
				return
			}
		}
	}()
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func providerChain(requested string) string {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case "javdb":
		return "javdb -> javbus"
	default:
		return "javbus -> javdb"
	}
}

func formatProxy(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "off"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "on (" + truncate(raw, 120) + ")"
	}
	auth := "off"
	if u.User != nil {
		auth = "on"
	}
	return fmt.Sprintf("on (%s://%s, auth=%s)", u.Scheme, u.Host, auth)
}

func formatStringListJSON(xs []string) string {
	// json.Marshal(nil slice) => "null"；对用户更友好的是 "[]"
	if xs == nil {
		xs = []string{}
	}
	b, err := json.Marshal(xs)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func formatFallbackNote(res domain.ItemResult) string {
	req := strings.ToLower(strings.TrimSpace(res.ProviderRequested))
	used := strings.ToLower(strings.TrimSpace(res.ProviderUsed))
	if req == "" || used == "" || req == used {
		return ""
	}
	// 只展示“requested provider”为何失败（否则会变成噪音）。
	for _, a := range res.Attempts {
		if strings.ToLower(strings.TrimSpace(a.Provider)) != req {
			continue
		}
		if strings.TrimSpace(a.ErrorCode) == "" {
			continue
		}
		msg := strings.TrimSpace(a.ErrorMsg)
		if msg == "" {
			msg = a.ErrorCode
		} else {
			msg = a.ErrorCode + ": " + msg
		}
		return " fallback(" + req + " " + truncate(msg, 90) + ")"
	}
	return " fallback(" + req + ")"
}

func formatAttemptChain(attempts []domain.ProviderAttempt, max int) string {
	if len(attempts) == 0 || max == 0 {
		return ""
	}
	if max < 0 {
		max = len(attempts)
	}
	parts := make([]string, 0, len(attempts))
	for _, a := range attempts {
		p := strings.TrimSpace(a.Provider)
		st := strings.TrimSpace(a.Stage)
		ec := strings.TrimSpace(a.ErrorCode)
		em := strings.TrimSpace(a.ErrorMsg)
		s := p + ":" + st
		if ec != "" {
			s += ":" + ec
		}
		if em != "" {
			s += ":" + truncate(em, 80)
		}
		parts = append(parts, s)
		if len(parts) >= max {
			break
		}
	}
	return strings.Join(parts, ";")
}

func formatShortDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Seconds())
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func intField(fields map[string]any, key string) int {
	if fields == nil {
		return 0
	}
	v, ok := fields[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case uint:
		return int(x)
	case uint32:
		return int(x)
	case uint64:
		return int(x)
	default:
		return 0
	}
}
