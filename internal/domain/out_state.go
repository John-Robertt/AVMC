package domain

// OutState 描述 out/<CODE>/ 的现状（只做 stat/ReadDir，不读内容）。
type OutState struct {
	OutDir string

	HasNFO    bool
	HasPoster bool
	HasFanart bool

	// ExistingNames 是目录内现有文件名集合，用于 O(1) 冲突判定。
	ExistingNames map[string]struct{}
}
