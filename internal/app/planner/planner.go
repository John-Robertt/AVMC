package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

// ReadOutState 读取 out/<CODE>/ 的现状（只做 ReadDir，不读文件内容）。
// 若 outDir 不存在，返回空状态且不报错。
func ReadOutState(root string, code domain.Code) (domain.OutState, error) {
	outDir := filepath.Join(root, "out", string(code))
	st := domain.OutState{
		OutDir:        outDir,
		ExistingNames: map[string]struct{}{},
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return domain.OutState{}, err
	}

	for _, e := range entries {
		st.ExistingNames[e.Name()] = struct{}{}
	}

	if _, ok := st.ExistingNames[string(code)+".nfo"]; ok {
		st.HasNFO = true
	}
	if _, ok := st.ExistingNames["poster.jpg"]; ok {
		st.HasPoster = true
	}
	if _, ok := st.ExistingNames["fanart.jpg"]; ok {
		st.HasFanart = true
	}

	return st, nil
}

// PlanItem 基于 WorkItem + OutState 生成确定性的执行计划（不做任何写入/移动）。
func PlanItem(providerRequested string, files []domain.VideoFile, item domain.WorkItem, st domain.OutState) (domain.ItemPlan, error) {
	used := make(map[string]struct{}, len(st.ExistingNames)+len(item.FileIdx))
	for n := range st.ExistingNames {
		used[n] = struct{}{}
	}

	moves := make([]domain.MovePlan, 0, len(item.FileIdx))
	for _, idx := range item.FileIdx {
		if idx < 0 || idx >= len(files) {
			return domain.ItemPlan{}, fmt.Errorf("非法 file index：%d", idx)
		}

		srcAbs := files[idx].AbsPath
		name := filepath.Base(srcAbs) // 尽量保留原文件名（含扩展名大小写）
		dstName := allocName(name, used)
		used[dstName] = struct{}{}

		moves = append(moves, domain.MovePlan{
			SrcAbs: srcAbs,
			DstAbs: filepath.Join(st.OutDir, dstName),
		})
	}

	needNFO := !st.HasNFO
	needPoster := !st.HasPoster
	needFanart := !st.HasFanart

	return domain.ItemPlan{
		Code:              item.Code,
		ProviderRequested: providerRequested,
		Moves:             moves,
		Need: domain.SidecarNeed{
			// poster 由 fanart 裁切得到：仅当需要 NFO 或 fanart 时才必须刮削。
			NeedScrape: needNFO || needFanart,
			NeedNFO:    needNFO,
			NeedPoster: needPoster,
			NeedFanart: needFanart,
		},
	}, nil
}

func allocName(name string, used map[string]struct{}) string {
	if _, ok := used[name]; !ok {
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s__%d%s", base, n, ext)
		if _, ok := used[cand]; !ok {
			return cand
		}
	}
}

// SortPlans 让上层在需要时可显式保证稳定顺序（而不是依赖 map 遍历顺序）。
func SortPlans(plans []domain.ItemPlan) {
	sort.Slice(plans, func(i, j int) bool { return string(plans[i].Code) < string(plans[j].Code) })
}
