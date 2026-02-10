package fsx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// 通过可替换的函数指针，让测试能稳定模拟 EXDEV 等错误。
var renameFunc = os.Rename

// PathTypeConflictError 表示目标路径类型冲突（例如期望文件但实际是目录）。
// 上层可把它映射为 error_code=target_conflict。
type PathTypeConflictError struct {
	Path string
	Want string
	Got  string
}

func (e *PathTypeConflictError) Error() string {
	return fmt.Sprintf("目标路径类型冲突：%q（期望 %s，实际 %s）", e.Path, e.Want, e.Got)
}

func IsPathTypeConflict(err error) bool {
	var e *PathTypeConflictError
	return errors.As(err, &e)
}

// CrossDeviceError 表示跨盘（EXDEV）导致的 rename 失败。
// 按产品契约：遇到 EXDEV 必须失败并提示用户，不做 copy+delete。
type CrossDeviceError struct {
	Src string
	Dst string
	Err error
}

func (e *CrossDeviceError) Error() string {
	return fmt.Sprintf("跨盘移动失败（EXDEV）：%q -> %q；请确保源与目标在同一文件系统（本工具不会隐式 copy+delete）：%v", e.Src, e.Dst, e.Err)
}

func (e *CrossDeviceError) Unwrap() error { return e.Err }

// IsCrossDevice 判断 err 是否为跨盘（EXDEV）错误。
func IsCrossDevice(err error) bool {
	var e *CrossDeviceError
	return errors.As(err, &e)
}

// Rename 封装 os.Rename，并把 EXDEV 显式标记为 CrossDeviceError。
func Rename(src, dst string) error {
	if err := renameFunc(src, dst); err != nil {
		if isEXDEV(err) {
			return &CrossDeviceError{Src: src, Dst: dst, Err: err}
		}
		return err
	}
	return nil
}

// WriteFileAtomic 在 dir 下原子写入 name（临时文件 + rename）。
//
// 语义：若目标已存在则覆盖（即 replace）。
//
// 说明：
// - sidecar（nfo/poster/fanart）按产品契约“不允许覆盖”，请使用 WriteFileAtomicNoOverwrite。
// - cache/report 等内部状态可以覆盖，使用该函数即可。
func WriteFileAtomic(dir, name string, data []byte) error {
	return WriteFileAtomicReplace(dir, name, data)
}

// WriteFileAtomicNoOverwrite 在 dir 下原子写入 name（临时文件 + rename）。
//
// - 临时文件必须与目标文件在同目录，以保证 rename 的原子性
// - fsync 是可选但推荐：我们对临时文件做 Sync；目录 Sync 采用 best-effort（避免平台差异导致误报失败）
//
// 注意：WriteFileAtomicNoOverwrite 用于 sidecar 等“不允许覆盖”的文件写入。
// 若需要覆盖（例如 report/cache），请使用 WriteFileAtomicReplace。
func WriteFileAtomicNoOverwrite(dir, name string, data []byte) error {
	dst := filepath.Join(filepath.Clean(dir), name)
	if fi, err := os.Lstat(dst); err == nil {
		if fi.IsDir() {
			return &PathTypeConflictError{Path: dst, Want: "file", Got: "dir"}
		}
		if !fi.Mode().IsRegular() {
			return &PathTypeConflictError{Path: dst, Want: "regular file", Got: fi.Mode().Type().String()}
		}
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeFileAtomic(dir, name, data, 0o644, false)
}

// WriteFileAtomicReplace 写入并覆盖同名文件（尽量保持原子性；Windows 上为 best-effort）。
func WriteFileAtomicReplace(dir, name string, data []byte) error {
	return writeFileAtomic(dir, name, data, 0o644, true)
}

func writeFileAtomic(dir, name string, data []byte, perm os.FileMode, replace bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	dst := filepath.Join(dir, name)

	// 创建同目录临时文件（前缀带 '.'，避免污染媒体库视图）。
	tmp, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if err := writeAll(tmp, data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		// Windows 下 chmod 可能不完全支持，但失败通常不影响正确性；为了简单，仍当作错误返回。
		// 若未来需要更强兼容性，可把该错误降级为 warning。
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// rename 原子替换到最终文件名。
	if err := Rename(tmpName, dst); err != nil {
		return err
	}

	// 目录 fsync：best-effort（不同平台/文件系统的语义差异很大）。
	_ = syncDirBestEffort(dir)

	// rename 成功后，不应删除最终文件。
	return nil
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func syncDirBestEffort(dir string) error {
	// Windows 上目录 Sync 的语义与支持情况不稳定，这里直接跳过。
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
