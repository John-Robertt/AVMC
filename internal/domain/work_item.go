package domain

// WorkItem 是按 CODE 聚合后的工作单元。
// 为了数据局部性，WorkItem 只保存文件下标（指向 []VideoFile），避免复制大结构体。
type WorkItem struct {
	Code    Code
	FileIdx []int
}
