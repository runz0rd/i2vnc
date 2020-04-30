package i2vnc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kward/go-vnc"
	"github.com/kward/go-vnc/buttons"
	"github.com/kward/go-vnc/keys"
)

type VncRemote struct {
	log Logger
	vcc *vnc.ClientConn
}

func NewVncRemote(log Logger, server, port, pw string, settleMs time.Duration) (*VncRemote, error) {
	// Establish TCP connection to VNC server.
	// var err error
	nc, err := net.Dial("tcp", fmt.Sprintf("%v:%v", server, port))
	if err != nil {
		return nil, fmt.Errorf("Error connecting to VNC host. %v", err)
	}
	log.Printf("Connected.")

	// Negotiate connection with the server.
	vcc, err := vnc.Connect(context.Background(), nc, vnc.NewClientConfig(pw))
	if err != nil {
		return nil, fmt.Errorf("Error negotiating connection to VNC host. %v", err)
	}
	log.Printf("Authenticated.")

	// configure settle (UI) time to reduce lag
	vnc.SetSettle(settleMs)

	return &VncRemote{log, vcc}, nil
}

func (r VncRemote) ScreenW() uint16 {
	return r.vcc.FramebufferWidth()
}

func (r VncRemote) ScreenH() uint16 {
	return r.vcc.FramebufferHeight()
}

func (r VncRemote) SendKeyEvent(key uint32, isPress bool) error {
	if err := r.vcc.KeyEvent(keys.Key(key), isPress); err != nil {
		r.log.Errorf("Failed to send key event: %v", err)
		return fmt.Errorf("Failed to send key event: %v", err)
	}
	r.log.Debugf("Sent key event: %v", key)
	return nil
}

func (r VncRemote) SendPointerEvent(button uint8, x, y uint16) error {
	if err := r.vcc.PointerEvent(buttons.Button(button), x, y); err != nil {
		r.log.Errorf("Failed to send pointer event: %v", err)
		return fmt.Errorf("Failed to send pointer event: %v", err)
	}
	r.log.Debugf("Sent pointer event: %v, %v:%v", button, x, y)
	return nil
}
