package domain

import (
	"encoding/json"
	"sort"
	"time"
)

const (
	StatusProcessed = "processed"
	StatusSkipped   = "skipped"
	StatusFailed    = "failed"
	StatusUnmatched = "unmatched"
)

const (
	FileStatusPlanned    = "planned"
	FileStatusMoved      = "moved"
	FileStatusRolledBack = "rolled_back"
	FileStatusFailed     = "failed"
)

const (
	ErrCodeUnmatchedCode     = "unmatched_code"
	ErrCodeFetchFailed       = "fetch_failed"
	ErrCodeParseFailed       = "parse_failed"
	ErrCodeTargetConflict    = "target_conflict"
	ErrCodeIOFailed          = "io_failed"
	ErrCodeMoveFailed        = "move_failed"
	ErrCodeConfigNotFound    = "config_not_found"
	ErrCodeConfigInvalid     = "config_invalid"
	ErrCodeConfigMissingPath = "config_missing_path"
)

// RunReport 是对外稳定输出（report.json / stdout JSON）的结构。
type RunReport struct {
	Path   string `json:"path"`
	DryRun bool   `json:"dry_run"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`

	Summary ReportSummary `json:"summary"`
	Items   []ItemResult  `json:"items"`
}

type ReportSummary struct {
	Processed int `json:"processed"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
	Unmatched int `json:"unmatched"`
}

type ItemResult struct {
	Code              string `json:"code"`
	ProviderRequested string `json:"provider_requested"`
	ProviderUsed      string `json:"provider_used"`
	Website           string `json:"website"`

	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`

	Candidates []string     `json:"candidates"`
	Files      []FileResult `json:"files"`
}

type FileResult struct {
	Src    string `json:"src"`
	Dst    string `json:"dst"`
	Status string `json:"status"`
}

// Finalize 做三件事：
// 1) 时间统一为 UTC（确保 JSON 为 RFC3339 且后缀 Z）
// 2) items 稳定排序：按 code 字典序；code=="" 的条目排在最后
// 3) summary 由 items 计算得出
func (r *RunReport) Finalize() {
	r.StartedAt = r.StartedAt.UTC()
	r.FinishedAt = r.FinishedAt.UTC()

	sort.SliceStable(r.Items, func(i, j int) bool {
		a := r.Items[i].Code
		b := r.Items[j].Code
		if a == "" && b == "" {
			return false
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a < b
	})

	var s ReportSummary
	for _, it := range r.Items {
		switch it.Status {
		case StatusProcessed:
			s.Processed++
		case StatusSkipped:
			s.Skipped++
		case StatusFailed:
			s.Failed++
		case StatusUnmatched:
			s.Unmatched++
		}
	}
	r.Summary = s
}

// MarshalJSON 仅用于集中约束输出的稳定性（避免未来不小心引入非确定字段）。
// 当前只是透传 encoding/json 的默认行为。
func (r RunReport) MarshalJSON() ([]byte, error) {
	type Alias RunReport
	return json.Marshal(Alias(r))
}
