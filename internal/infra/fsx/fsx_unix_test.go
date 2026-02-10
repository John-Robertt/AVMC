//go:build unix

package fsx

import (
	"os"
	"syscall"
	"testing"
)

func TestRename_CrossDeviceEXDEV(t *testing.T) {
	old := renameFunc
	renameFunc = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EXDEV}
	}
	defer func() { renameFunc = old }()

	err := Rename("/a", "/b")
	if err == nil {
		t.Fatalf("期望错误，但得到 nil")
	}
	if !IsCrossDevice(err) {
		t.Fatalf("期望 CrossDeviceError，实际：%T %v", err, err)
	}
}
