package domain

// MovePlan 规划一次文件移动（只描述 src/dst；真正执行必须遵守“移动最后一步”）。
type MovePlan struct {
	SrcAbs string
	DstAbs string
}

type SidecarNeed struct {
	NeedNFO    bool
	NeedPoster bool
	NeedFanart bool
}

func (n SidecarNeed) NeedsScrape() bool {
	return n.NeedNFO || n.NeedFanart
}

// ItemPlan 是对某个 CODE 的最小执行计划。
type ItemPlan struct {
	Code              Code
	ProviderRequested string

	Moves []MovePlan
	Need  SidecarNeed
}
