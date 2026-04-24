package domain

// OutState 描述 out/<CODE>/ 的现状（只做 stat/ReadDir，不读内容）。
type OutState struct {
	OutDir string

	HasNFO    bool
	HasPoster bool
	HasFanart bool

	// ExistingNames 是目录内现有文件名集合，用于 O(1) 冲突判定。
	ExistingNames map[string]struct{}

	// SidecarConflicts 记录 sidecar 路径存在但不是普通文件的冲突。
	SidecarConflicts []SidecarConflict
}

type SidecarConflict struct {
	Name string
	Path string
	Got  string
}
