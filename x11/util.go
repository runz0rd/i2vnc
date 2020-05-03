package x11

import (
	"fmt"

	"github.com/runz0rd/i2vnc"
)

type Pointer struct {
	midW  uint16
	midH  uint16
	maxW  uint16
	maxH  uint16
	PrevX uint16
	PrevY uint16
	X     uint16
	Y     uint16
	Btn   uint8
}

func newPointer(localW, localH, remoteW, remoteH uint16) *Pointer {
	return &Pointer{
		midW: localW / 2,
		midH: localH / 2,
		maxW: remoteW,
		maxH: remoteH,
		X:    remoteW / 2,
		Y:    remoteH / 2,
		Btn:  0,
	}
}

func (p *Pointer) set(x, y uint16) {
	p.setX(x)
	p.setY(y)
}

func (p *Pointer) setPrev(x, y uint16) {
	p.PrevX = x
	p.PrevY = y
}

func (p *Pointer) setX(event uint16) uint16 {
	// add the change from the middle screen value (where the cursor resets)
	p.X += event - p.midW
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

func (p *Pointer) setY(event uint16) uint16 {
	p.Y += event - p.midH
	if p.Y < 1 || p.Y > 65535/2 {
		p.Y = 0
	}
	if p.Y >= p.maxH && p.Y <= 65535/2 {
		p.Y = p.maxH
	}
	return p.Y
}

func validateConfig(c *i2vnc.Config) error {
	_, err := findEvent(c.Hotkey)
	if err != nil {
		return err
	}
	for from, to := range c.Keymap {
		_, err = findEvent(from)
		if err != nil {
			return err
		}
		_, err = findEvent(to)
		if err != nil {
			return err
		}
	}
	return nil
}

func newKeyEvent(key uint32) (*i2vnc.Event, error) {
	e := i2vnc.Event{}
	for k, v := range Keysyms {
		if v == key {
			e.IsKey = true
			e.Key = v
			e.Name = k
		}
	}
	if e.Name == "" {
		return nil, fmt.Errorf("No keysym definition found for '%v'", key)
	}
	return &e, nil
}

func newButtonEvent(button uint8) (*i2vnc.Event, error) {
	e := i2vnc.Event{}
	for k, v := range Buttons {
		if v == button {
			e.IsKey = false
			e.Button = v
			e.Name = k
		}
	}
	if e.Name == "" {
		return nil, fmt.Errorf("No button definition found for '%v'", button)
	}
	return &e, nil
}

func findEvent(name string) (*i2vnc.Event, error) {
	e := i2vnc.Event{Name: name}
	var ok bool

	e.IsKey = true
	e.Key, ok = Keysyms[name]
	if !ok {
		e.IsKey = false
		e.Button, ok = Buttons[name]
		if !ok {
			return nil, fmt.Errorf("No button or keysym definition found for '%v'", name)
		}
	}
	return &e, nil
}
