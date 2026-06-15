//go:build !windows

package main

import "os/exec"

func hidePremProxyWindow(cmd *exec.Cmd) {
}

func killPremProxyProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
