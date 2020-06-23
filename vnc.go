package i2vnc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kward/go-vnc"
	"github.com/kward/go-vnc/buttons"
	"github.com/kward/go-vnc/keys"
	"github.com/sirupsen/logrus"
)

type VncRemote struct {
	l  *logrus.Entry
	c  Config
	vc *vnc.ClientConn
	nc net.Conn
	ci configItem
}

func NewVncRemote(logger *logrus.Logger, config Config) *VncRemote {
	return &VncRemote{l: logrus.NewEntry(logger), c: config}
}

func (r *VncRemote) Connect(cname string, timeout time.Duration) error {
	ci, err := r.c.getItem(cname)
	if err != nil {
		return err
	}
	r.l.Infof("connecting to vnc remote %q", cname)
	r.nc, err = net.DialTimeout("tcp", fmt.Sprintf("%v:%v", ci.Server, ci.Port), timeout*time.Second)
	if err != nil {
		return err
	}

	cc := vnc.NewClientConfig(ci.Pw)
	// cc.ServerMessageCh = make(chan vnc.ServerMessage)

	// Negotiate connection with the server.
	r.l.Infof("negotiating with vnc remote %q", cname)
	r.vc, err = vnc.Connect(context.Background(), r.nc, cc)
	if err != nil {
		return err
	}
	r.l.Infof("connected to vnc remote %q", cname)
	// configure settle (UI) time to reduce lag
	vnc.SetSettle(ci.SettleMs())
	r.ci = ci
	return nil
}

func (r *VncRemote) IsConnected() bool {
	if r.nc == nil || r.vc == nil {
		return false
	}
	// if _, err := r.nc.Read(make([]byte, 1)); err == io.EOF {
	// 	return false
	// }
	return true
}

func (r *VncRemote) Disconnect() error {
	if !r.IsConnected() {
		return nil
	}
	err := r.nc.Close()
	if err != nil {
		return err
	}
	r.l.Infof("disconnected from %q", r.ci.Name)
	return nil
}

func (r *VncRemote) Screen() Screen {
	if !r.IsConnected() {
		return Screen{}
	}
	return Screen{r.vc.FramebufferWidth(), r.vc.FramebufferHeight()}
}

func (r *VncRemote) SendKeyEvent(name string, key uint32, isPress bool) error {
	if !r.IsConnected() {
		return fmt.Errorf("remote not connected")
	}
	if err := r.vc.KeyEvent(keys.Key(key), isPress); err != nil {
		r.l.WithError(err).Error("failed to send key event")
		return err
	}
	DebugEvent(r.l, "VncRemote", true, name, 0, 0, isPress)
	return nil
}

func (r *VncRemote) SendPointerEvent(name string, button uint8, x, y uint16, isPress bool) error {
	if !r.IsConnected() {
		return fmt.Errorf("remote not connected")
	}
	if !isPress {
		// The `button` is a bitwise mask of various Button values. When a button
		// is set, it is pressed, when it is unset, it is released.
		button = 0
	}
	if err := r.vc.PointerEvent(buttonAdapter(button), x, y); err != nil {
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
