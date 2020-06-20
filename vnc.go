package i2vnc

import (
	"context"
	"fmt"
	"net"

	"github.com/kward/go-vnc"
	"github.com/kward/go-vnc/buttons"
	"github.com/kward/go-vnc/keys"
	"github.com/sirupsen/logrus"
)

type VncRemote struct {
	l   *logrus.Entry
	vcc *vnc.ClientConn
}

func NewVncRemote(logger *logrus.Logger, config *Config, pw string) (*VncRemote, error) {
	l := logrus.NewEntry(logger)
	// Establish TCP connection to VNC server.
	// var err error
	nc, err := net.Dial("tcp", fmt.Sprintf("%v:%v", config.Server, config.Port))
	if err != nil {
		return nil, fmt.Errorf("error connecting to VNC host: %v", err)
	}
	l.Info("connected")

	//todo figure this out
	cc := vnc.NewClientConfig(pw)
	cc.ServerMessageCh = make(chan vnc.ServerMessage)

	// Negotiate connection with the server.
	vcc, err := vnc.Connect(context.Background(), nc, cc)
	if err != nil {
		return nil, fmt.Errorf("error negotiating connection to VNC host: %v", err)
	}
	l.Info("authenticated")

	// configure settle (UI) time to reduce lag
	vnc.SetSettle(config.SettleMs)

	// vcc.FramebufferUpdateRequest(rfbflags.RFBTrue, 10, 20, 30, 40)
	// vcc.ListenAndHandle()

	return &VncRemote{l, vcc}, nil
}

func (r *VncRemote) Disconnect() error {
	return r.vcc.Close()
}

func (r VncRemote) ScreenW() uint16 {
	return r.vcc.FramebufferWidth()
}

func (r VncRemote) ScreenH() uint16 {
	return r.vcc.FramebufferHeight()
}

func (r VncRemote) SendKeyEvent(name string, key uint32, isPress bool) error {
	if err := r.vcc.KeyEvent(keys.Key(key), isPress); err != nil {
		r.l.WithError(err).Error("failed to send key event")
		return err
	}
	DebugEvent(r.l, "VncRemote", true, name, 0, 0, isPress)
	return nil
}

func (r VncRemote) SendPointerEvent(name string, button uint8, x, y uint16, isPress bool) error {
	if !isPress {
		// The `button` is a bitwise mask of various Button values. When a button
		// is set, it is pressed, when it is unset, it is released.
		button = 0
	}
	if err := r.vcc.PointerEvent(buttonAdapter(button), x, y); err != nil {
		r.l.WithError(err).Error("failed to send pointer event")
		return err
	}
	DebugEvent(r.l, "VncRemote", false, name, x, y, isPress)
	return nil
}

func buttonAdapter(button uint8) buttons.Button {
	if button == 3 {
		return buttons.Right
	}
	if button == 4 {
		return buttons.Four
	}
	if button == 5 {
		return buttons.Five
	}
	return buttons.Button(button)
}
