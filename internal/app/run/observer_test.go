package run

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/John-Robertt/AVMC/internal/config"
	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/provider"
)

type recordObserver struct {
	mu sync.Mutex

	startCalls int
	phases     []string
	items      []domain.Code
}

func (o *recordObserver) OnStart(eff config.EffectiveConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.startCalls++
}

func (o *recordObserver) OnPhaseDone(name string, fields map[string]any, dur time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.phases = append(o.phases, name)
}

func (o *recordObserver) OnItemDone(idx, total int, code domain.Code, res domain.ItemResult, dur time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.items = append(o.items, code)
}

func (o *recordObserver) OnProgress(done, total, ok, fail, skip, active int, activeCodes []string, elapsed time.Duration) {
	// keepalive 由 CLI 触发；这里无需断言。
}

func TestExecuteWithObserver_EmitsPhaseAndItemEvents(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{Title: "T", FanartURL: "https://img.test/f.jpg"}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	obs := &recordObserver{}
	_ = ExecuteWithObserver(context.Background(), config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       false,
		Concurrency: 1,
	}, reg, obs)

	if obs.startCalls != 1 {
		t.Fatalf("期望 OnStart 调用 1 次，实际 %d", obs.startCalls)
	}

	wantPhases := []string{"scan", "group", "plan", "exec"}
	if !reflect.DeepEqual(obs.phases, wantPhases) {
		t.Fatalf("阶段事件不符合预期：got=%v want=%v", obs.phases, wantPhases)
	}
	if len(obs.items) != 1 || obs.items[0] != "CAWD-895" {
		t.Fatalf("条目事件不符合预期：items=%v", obs.items)
	}
}

func TestExecuteWithObserver_NilObserver_SameResultAsExecute(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "in", "CAWD-895.mp4")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入视频失败：%v", err)
	}

	reg, err := provider.NewRegistry(
		stubProvider{name: "javbus", meta: domain.MovieMeta{Title: "T", FanartURL: "https://img.test/f.jpg"}},
		stubProvider{name: "javdb", meta: domain.MovieMeta{Title: "T2"}},
	)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}

	cfg := config.EffectiveConfig{
		Path:        root,
		Provider:    "javbus",
		Apply:       false,
		Concurrency: 1,
	}

	a := Execute(context.Background(), cfg, reg)
	b := ExecuteWithObserver(context.Background(), cfg, reg, nil)

	// 时间字段本身允许有微小差异；对比时归零。
	a.StartedAt, a.FinishedAt = time.Time{}, time.Time{}
	b.StartedAt, b.FinishedAt = time.Time{}, time.Time{}

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("nil observer 不应改变结果：\nExecute=%+v\nWithObs=%+v", a, b)
	}
}

