//go:build !unix

package service

import "errors"

var errDetachUnsupported = errors.New("detached services are supported on macOS and Linux")

func StartDetached(string, []string, string, string) (int, error) {
	return 0, errDetachUnsupported
}

func ProcessRunning(int) bool { return false }
func StopProcess(int) error   { return errDetachUnsupported }
