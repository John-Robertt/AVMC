package app

import (
	"errors"
	"sort"

	"github.com/John-Robertt/AVMC/internal/code"
	"github.com/John-Robertt/AVMC/internal/domain"
)

// GroupByCode 把视频文件按 CODE 分组为 WorkItem（WorkItem 只存 file index）。
//
// - items 稳定排序：按 Code 字典序
// - item 内 FileIdx 稳定排序：按 RelPath 字典序
func GroupByCode(files []domain.VideoFile) (items []domain.WorkItem, unmatched []domain.Unmatched, err error) {
	index := make(map[domain.Code]int, 128)
	items = make([]domain.WorkItem, 0, 128)
	unmatched = make([]domain.Unmatched, 0, 32)

	for i := range files {
		c, e := code.Extract(files[i])
		if e != nil {
			var ue *code.UnmatchedError
			if errors.As(e, &ue) {
				u := domain.Unmatched{
					File: files[i],
					Kind: ue.Kind,
				}
				if len(ue.Candidates) > 0 {
					u.Candidates = append([]domain.Code(nil), ue.Candidates...)
				}
				unmatched = append(unmatched, u)
				continue
			}
			return nil, nil, e
		}

		if idx, ok := index[c]; ok {
			items[idx].FileIdx = append(items[idx].FileIdx, i)
			continue
		}
		index[c] = len(items)
		items = append(items, domain.WorkItem{
			Code:    c,
			FileIdx: []int{i},
		})
	}

	sort.Slice(items, func(i, j int) bool { return string(items[i].Code) < string(items[j].Code) })
	for i := range items {
		sort.Slice(items[i].FileIdx, func(a, b int) bool {
			ia := items[i].FileIdx[a]
			ib := items[i].FileIdx[b]
			return files[ia].RelPath < files[ib].RelPath
		})
	}
	return items, unmatched, nil
}
