//go:build !windows

package tray

type Manager struct{}

func Start(Options) (*Manager, error) {
	return &Manager{}, nil
}

func (m *Manager) Stop() {}
