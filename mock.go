package i2vnc

import (
	"github.com/sirupsen/logrus"
)

type MockRemote struct {
	l *logrus.Entry
}

func NewMockRemote(l *logrus.Entry) *MockRemote {
	return &MockRemote{l}
}

func (r MockRemote) ScreenW() uint16 {
	return 1
}

func (r MockRemote) ScreenH() uint16 {
	return 1
}

func (r MockRemote) SendKeyEvent(name string, key uint32, isPress bool) error {
	DebugEvent(r.l, "MockRemote", true, name, 0, 0, isPress)
	return nil
}

func (r MockRemote) SendPointerEvent(name string, button uint8, x, y uint16, isPress bool) error {
	DebugEvent(r.l, "MockRemote", false, name, x, y, isPress)
	return nil
}
