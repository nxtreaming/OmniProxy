//go:build !windows

package autostart

import "errors"

func Enabled(string) (bool, error) {
	return false, nil
}

func Set(string, bool, ...string) error {
	return errors.New("auto start is only supported on Windows")
}
