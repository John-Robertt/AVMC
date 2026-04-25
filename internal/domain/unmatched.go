package domain

const (
	UnmatchedNoMatch   = "no_match"
	UnmatchedAmbiguous = "ambiguous"
)

// Unmatched 描述无法解析出唯一 CODE 的输入文件。
// 用于 report 的 unmatched 条目（含 ambiguous 候选列表）。
type Unmatched struct {
	File       VideoFile
	Kind       string // UnmatchedNoMatch | UnmatchedAmbiguous
	Candidates []Code // 仅 ambiguous 时非空（已排序）
}

