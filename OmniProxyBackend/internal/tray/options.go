package tray

type Options struct {
	Tooltip        string
	StatusLabel    func() string
	PortLabel      func() string
	IsProxyRunning func() bool
	StartProxy     func() error
	StopProxy      func() error
	ShowWindow     func()
	Quit           func()
	Log            func(string, ...any)
}
