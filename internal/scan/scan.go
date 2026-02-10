package scan

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/John-Robertt/AVMC/internal/domain"
)

// ScanVideos 扫描 root 下的视频文件，并应用目录排除规则。
//
// 规则（硬约束）：
// - 永久排除：<root>/out/ 与 <root>/cache/
// - excludeDirs：来自配置文件，均视为相对 root 的路径（若是绝对路径，则按绝对路径处理）
//
// 注意：扫描阶段只做 stat（DirEntry.Info），不读文件内容。
func ScanVideos(root string, excludeDirs []string) ([]domain.VideoFile, error) {
	root = filepath.Clean(root)
	excluded := buildExcluded(root, excludeDirs)

	files := make([]domain.VideoFile, 0, 128)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// 统一的排除判断：目录用 SkipDir，文件则直接跳过。
		if isExcluded(path, excluded) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !isVideoExt(ext) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		files = append(files, domain.VideoFile{
			AbsPath: path,
			RelPath: rel,
			Base:    strings.TrimSuffix(name, filepath.Ext(name)),
			Ext:     ext,
			Size:    info.Size(),
			ModUnix: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 强制稳定输出，避免不同平台/文件系统行为差异带来的不确定性。
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, nil
}

func isVideoExt(ext string) bool {
	switch ext {
	case ".mp4", ".mkv", ".avi":
		return true
	default:
		return false
	}
}

func buildExcluded(root string, excludeDirs []string) []string {
	outDir := filepath.Join(root, "out")
	cacheDir := filepath.Join(root, "cache")

	excluded := make([]string, 0, 2+len(excludeDirs))
	excluded = append(excluded, filepath.Clean(outDir), filepath.Clean(cacheDir))

	for _, x := range excludeDirs {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		if filepath.IsAbs(x) {
			excluded = append(excluded, filepath.Clean(x))
			continue
		}
		// x 是相对路径：相对 root。
		excluded = append(excluded, filepath.Clean(filepath.Join(root, x)))
	}

	// 排除列表排序后，isExcluded 的行为更可预测（且便于测试）。
	sort.Strings(excluded)
	return excluded
}

func isExcluded(path string, excluded []string) bool {
	path = filepath.Clean(path)
	for _, base := range excluded {
		if isUnder(path, base) {
			return true
		}
	}
	return false
}

func isUnder(path, base string) bool {
	if path == base {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(path, base+sep)
}
