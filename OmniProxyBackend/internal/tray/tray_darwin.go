//go:build darwin

package tray

/*
#cgo darwin CFLAGS: -fobjc-arc -fblocks
#cgo darwin LDFLAGS: -framework Cocoa

#include <stdlib.h>
void omniproxyDarwinStartStatusItem(const char *tooltip);
void omniproxyDarwinStopStatusItem(void);
*/
import "C"

import (
	"sync"
	"unsafe"
)

type Manager struct {
	opts Options
	once sync.Once
}

var (
	darwinActiveMu      sync.RWMutex
	darwinActiveManager *Manager
)

func Start(opts Options) (*Manager, error) {
	manager := &Manager{opts: opts}
	darwinActiveMu.Lock()
	darwinActiveManager = manager
	darwinActiveMu.Unlock()

	tooltip := C.CString(opts.Tooltip)
	defer C.free(unsafe.Pointer(tooltip))
	C.omniproxyDarwinStartStatusItem(tooltip)
	return manager, nil
}

func (m *Manager) Stop() {
	m.once.Do(func() {
		darwinActiveMu.Lock()
		if darwinActiveManager == m {
			darwinActiveManager = nil
		}
		darwinActiveMu.Unlock()
		C.omniproxyDarwinStopStatusItem()
	})
}

func darwinManager() *Manager {
	darwinActiveMu.RLock()
	defer darwinActiveMu.RUnlock()
	return darwinActiveManager
}

//export omniproxyTrayDarwinStatusLabel
func omniproxyTrayDarwinStatusLabel() *C.char {
	manager := darwinManager()
	if manager == nil || manager.opts.StatusLabel == nil {
		return C.CString("代理状态未知")
	}
	return C.CString(manager.opts.StatusLabel())
}

//export omniproxyTrayDarwinPortLabel
func omniproxyTrayDarwinPortLabel() *C.char {
	manager := darwinManager()
	if manager == nil || manager.opts.PortLabel == nil {
		return C.CString("端口未知")
	}
	return C.CString(manager.opts.PortLabel())
}

//export omniproxyTrayDarwinProxyRunning
func omniproxyTrayDarwinProxyRunning() C.int {
	manager := darwinManager()
	if manager == nil || manager.opts.IsProxyRunning == nil || !manager.opts.IsProxyRunning() {
		return 0
	}
	return 1
}

//export omniproxyTrayDarwinToggleProxy
func omniproxyTrayDarwinToggleProxy() {
	manager := darwinManager()
	if manager == nil {
		return
	}
	go func() {
		var err error
		if manager.opts.IsProxyRunning != nil && manager.opts.IsProxyRunning() {
			if manager.opts.StopProxy != nil {
				err = manager.opts.StopProxy()
			}
		} else if manager.opts.StartProxy != nil {
			err = manager.opts.StartProxy()
		}
		if err != nil && manager.opts.Log != nil {
			manager.opts.Log("macOS menu bar action failed: %v", err)
		}
	}()
}

//export omniproxyTrayDarwinShowWindow
func omniproxyTrayDarwinShowWindow() {
	manager := darwinManager()
	if manager == nil || manager.opts.ShowWindow == nil {
		return
	}
	go manager.opts.ShowWindow()
}

//export omniproxyTrayDarwinQuit
func omniproxyTrayDarwinQuit() {
	manager := darwinManager()
	if manager == nil || manager.opts.Quit == nil {
		return
	}
	go manager.opts.Quit()
}
