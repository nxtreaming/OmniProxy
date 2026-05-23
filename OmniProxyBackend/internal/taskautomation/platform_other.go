//go:build !windows

package taskautomation

import "fmt"

type noopPlatform struct{}

func defaultPlatformController() platformController {
	return noopPlatform{}
}

func (noopPlatform) ForegroundWindow() windowHandle {
	return 0
}

func (noopPlatform) Launch(launchRequest) (launchResult, error) {
	return launchResult{}, fmt.Errorf("task automation is only supported on Windows")
}

func (noopPlatform) PressSpace() error {
	return fmt.Errorf("task automation is only supported on Windows")
}

func (noopPlatform) Focus(windowHandle) error {
	return fmt.Errorf("task automation is only supported on Windows")
}
