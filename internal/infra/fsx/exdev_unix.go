//go:build unix

package fsx

import (
	"errors"
	"os"
	"syscall"
)

func isEXDEV(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	var le *os.LinkError
	if errors.As(err, &le) && errors.Is(le.Err, syscall.EXDEV) {
		return true
	}
	return false
}
