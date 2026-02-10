package run

import (
	"time"

	"github.com/John-Robertt/AVMC/internal/config"
	"github.com/John-Robertt/AVMC/internal/domain"
)

// Observer 用于把“运行进度/阶段/条目结果”从核心执行流程中解耦出来。
//
// 约束：
// - run 包只负责发事件，不做任何输出（避免污染 stdout 的 JSON 契约）。
// - Observer 的实现必须并发安全：事件可能来自多个 goroutine。
type Observer interface {
	// OnStart 在 ExecuteWithObserver 开始时调用（应尽量早，保证用户 1 秒内看到输出）。
	OnStart(eff config.EffectiveConfig)
	// OnPhaseDone 在阶段结束/就绪时调用（用于打印阶段统计与耗时）。
	OnPhaseDone(name string, fields map[string]any, dur time.Duration)
	// OnItemDone 在某个 CODE 处理完成时调用（用于每条结果的一行输出）。
	OnItemDone(idx, total int, code domain.Code, res domain.ItemResult, dur time.Duration)
	// OnProgress 用于 keepalive（通常由 CLI 自己 ticker 触发；run 层不强制调用）。
	OnProgress(done, total, ok, fail, skip, active int, activeCodes []string, elapsed time.Duration)
}

