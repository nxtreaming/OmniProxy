//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

func hidePremProxyWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func killPremProxyProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	hidePremProxyWindow(kill)
	_ = kill.Run()
	_ = cmd.Process.Kill()
}
