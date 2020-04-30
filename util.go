package i2vnc

import (
	"fmt"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/kward/go-vnc/buttons"
)

type WindowSystem uint8

const (
	X11 WindowSystem = iota
)

func NewInput(ws WindowSystem, log Logger, scrollSpeed uint8, remote Remote) (Input, error) {
	switch ws {
	case X11:
		return NewX11Input(log, scrollSpeed, remote)
	default:
		return nil, fmt.Errorf("No suitable window system found.")
	}
}

type Input interface {
	Grab() error
}

type Remote interface {
	ScreenW() uint16
	ScreenH() uint16
	SendKeyEvent(key uint32, isPress bool) error
	SendPointerEvent(button uint8, x, y uint16) error
}

type Logger interface {
	Printf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

type Pointer struct {
	midW        uint16
	midH        uint16
	maxW        uint16
	maxH        uint16
	PrevX       uint16
	PrevY       uint16
	X           uint16
	Y           uint16
	ScrollSpeed uint8
	Btn         uint8
}

func newPointer(localW, localH, remoteW, remoteH uint16, scrollspeed uint8) *Pointer {
	return &Pointer{
		midW:        localW / 2,
		midH:        localH / 2,
		maxW:        remoteW,
		maxH:        remoteH,
		X:           remoteW / 2,
		Y:           remoteH / 2,
		ScrollSpeed: scrollspeed,
		Btn:         0,
	}
}

func (p *Pointer) EventX(event int16) uint16 {
	// add the change from the middle screen value (where the cursor resets)
	p.X += uint16(event) - p.midW
	if p.X < 1 || p.X > 65535/2 {
		// when going under 0, the value starts going down from max uint16
		// if it is in the upper half of max uint16, reset to 0
		p.X = 0
	}
	if p.X >= p.maxW && p.X <= 65535/2 {
		// if it is in the lower half of max uint16, reset to max
		p.X = p.maxW
	}
	return p.X
}

func (p *Pointer) EventY(event int16) uint16 {
	p.Y += uint16(event) - p.midH
	if p.Y < 1 || p.Y > 65535/2 {
		p.Y = 0
	}
	if p.Y >= p.maxH && p.Y <= 65535/2 {
		p.Y = p.maxH
	}
	return p.Y
}

func xButtonAdapter(b xproto.Button) uint8 {
	if b == 3 {
		return uint8(buttons.Right)
	}
	if b == 4 {
		return uint8(buttons.Four)
	}
	if b == 5 {
		return uint8(buttons.Five)
	}
	return uint8(buttons.Button(b))
}
