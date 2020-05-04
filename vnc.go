package i2vnc

import (
	"context"
	"fmt"
	"net"

	"github.com/kward/go-vnc"
	"github.com/kward/go-vnc/buttons"
	"github.com/kward/go-vnc/keys"
)

type VncRemote struct {
	log Logger
	vcc *vnc.ClientConn
}

func NewVncRemote(log Logger, config *Config, pw string) (*VncRemote, error) {
	// Establish TCP connection to VNC server.
	// var err error
	nc, err := net.Dial("tcp", fmt.Sprintf("%v:%v", config.Server, config.Port))
	if err != nil {
		return nil, fmt.Errorf("Error connecting to VNC host. %v", err)
	}
	log.Printf("Connected.")

	//todo figure this out
	cc := vnc.NewClientConfig(pw)
	cc.ServerMessageCh = make(chan vnc.ServerMessage)

	// Negotiate connection with the server.
	vcc, err := vnc.Connect(context.Background(), nc, cc)
	if err != nil {
		return nil, fmt.Errorf("Error negotiating connection to VNC host. %v", err)
	}
	log.Printf("Authenticated.")

	// configure settle (UI) time to reduce lag
	vnc.SetSettle(config.SettleMs)

	// vcc.FramebufferUpdateRequest(rfbflags.RFBTrue, 10, 20, 30, 40)
	// vcc.ListenAndHandle()

	return &VncRemote{log, vcc}, nil
}

func (r VncRemote) ScreenW() uint16 {
	return r.vcc.FramebufferWidth()
}

func (r VncRemote) ScreenH() uint16 {
	return r.vcc.FramebufferHeight()
}

func (r VncRemote) SendKeyEvent(name string, key uint32, isPress bool) error {
	if err := r.vcc.KeyEvent(keys.Key(key), isPress); err != nil {
		r.log.Errorf("Failed to send key event: %v", err)
		return fmt.Errorf("Failed to send key event: %v", err)
	}
	DebugEvent(r.log, "Sent", true, name, 0, 0, isPress)
	return nil
}

func (r VncRemote) SendPointerEvent(name string, button uint8, x, y uint16) error {
	if err := r.vcc.PointerEvent(buttonAdapter(button), x, y); err != nil {
		r.log.Errorf("Failed to send pointer event: %v", err)
		return fmt.Errorf("Failed to send pointer event: %v", err)
	}
	DebugEvent(r.log, "Sent", false, name, x, y, false)
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
