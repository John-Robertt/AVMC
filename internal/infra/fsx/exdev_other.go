//go:build !unix && !windows

package fsx

func isEXDEV(err error) bool { return false }
