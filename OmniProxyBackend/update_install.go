//go:build !windows

package main

import "os/exec"

func defaultStartUpdateInstaller(filePath string, args []string) error {
	return exec.Command(filePath, args...).Start()
}
