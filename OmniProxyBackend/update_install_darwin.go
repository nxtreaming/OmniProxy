//go:build darwin

package main

import "os/exec"

func defaultStartUpdateInstaller(filePath string, args []string) error {
	openArgs := append([]string{filePath}, args...)
	return exec.Command("open", openArgs...).Start()
}
