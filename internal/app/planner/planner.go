package planner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

type SidecarConflictError struct {
	Conflicts []domain.SidecarConflict
}

type TargetConflictError struct {
	Path string
	Want string
	Got  string
}

func (e *TargetConflictError) Error() string {
	if e == nil {
		return "目标路径类型冲突"
	}
	return fmt.Sprintf("目标路径类型冲突：%q（期望 %s，实际 %s）", e.Path, e.Want, e.Got)
}

func (e *SidecarConflictError) Error() string {
	if e == nil || len(e.Conflicts) == 0 {
		return "sidecar 路径类型冲突"
	}
	c := e.Conflicts[0]
	if len(e.Conflicts) == 1 {
		return fmt.Sprintf("sidecar 路径类型冲突：%q（期望 regular file，实际 %s）", c.Path, c.Got)
	}
	return fmt.Sprintf("sidecar 路径类型冲突：%q 等 %d 项（期望 regular file）", c.Path, len(e.Conflicts))
}

func IsSidecarConflict(err error) bool {
	var e *SidecarConflictError
	return errors.As(err, &e)
}

func IsTargetConflict(err error) bool {
	var e *TargetConflictError
	return errors.As(err, &e) || IsSidecarConflict(err)
}

// ReadOutState 读取 out/<CODE>/ 的现状（只做 ReadDir，不读文件内容）。
// 若 outDir 不存在，返回空状态且不报错。
func ReadOutState(root string, code domain.Code) (domain.OutState, error) {
	outRoot := filepath.Join(root, "out")
	outDir := filepath.Join(root, "out", string(code))
	st := domain.OutState{
		OutDir:        outDir,
		ExistingNames: map[string]struct{}{},
	}

	if exists, err := ensureExistingDir(outRoot); err != nil {
		return domain.OutState{}, err
	} else if !exists {
		return st, nil
	}
	if exists, err := ensureExistingDir(outDir); err != nil {
		return domain.OutState{}, err
	} else if !exists {
		return st, nil
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return domain.OutState{}, err
	}

	for _, e := range entries {
		name := e.Name()
		st.ExistingNames[name] = struct{}{}
		if !isSidecarName(code, name) {
			continue
		}
		if err := recordSidecar(&st, code, name, e); err != nil {
			return domain.OutState{}, err
		}
	}

	return st, nil
}

// PlanItem 基于 WorkItem + OutState 生成确定性的执行计划（不做任何写入/移动）。
func PlanItem(providerRequested string, files []domain.VideoFile, item domain.WorkItem, st domain.OutState) (domain.ItemPlan, error) {
	if len(st.SidecarConflicts) > 0 {
		return domain.ItemPlan{}, &SidecarConflictError{Conflicts: append([]domain.SidecarConflict(nil), st.SidecarConflicts...)}
	}

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

func isSidecarName(code domain.Code, name string) bool {
	return name == string(code)+".nfo" || name == "poster.jpg" || name == "fanart.jpg"
}

func ensureExistingDir(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, &TargetConflictError{
			Path: path,
			Want: "dir",
			Got:  fileModeKind(info.Mode()),
		}
	}
	return true, nil
}

func recordSidecar(st *domain.OutState, code domain.Code, name string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		st.SidecarConflicts = append(st.SidecarConflicts, domain.SidecarConflict{
			Name: name,
			Path: filepath.Join(st.OutDir, name),
			Got:  fileModeKind(info.Mode()),
		})
		return nil
	}

	switch name {
	case string(code) + ".nfo":
		st.HasNFO = true
	case "poster.jpg":
		st.HasPoster = true
	case "fanart.jpg":
		st.HasFanart = true
	}
	return nil
}

func fileModeKind(mode fs.FileMode) string {
	switch {
	case mode.IsDir():
		return "dir"
	case mode&fs.ModeSymlink != 0:
		return "symlink"
	case mode.IsRegular():
		return "regular file"
	}
	return mode.Type().String()
}

// SortPlans 让上层在需要时可显式保证稳定顺序（而不是依赖 map 遍历顺序）。
func SortPlans(plans []domain.ItemPlan) {
	sort.Slice(plans, func(i, j int) bool { return string(plans[i].Code) < string(plans[j].Code) })
}
