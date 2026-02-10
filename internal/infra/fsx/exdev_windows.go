//go:build windows

package fsx

func isEXDEV(err error) bool {
	// Windows 不存在“跨设备 rename == EXDEV”的统一语义，这里默认 false。
	return false
}
