package i2vnc

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type WindowSystem uint8

const (
	X11 WindowSystem = iota
)

func NewInput(log Logger, ws WindowSystem, remote Remote, config *Config) (Input, error) {
	switch ws {
	case X11:
		return NewX11Input(log, remote, config)
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
	SendKeyEvent(name string, key uint32, isPress bool) error
	SendPointerEvent(name string, button uint8, x, y uint16) error
}

type Logger interface {
	Printf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

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

type Config struct {
	Server      string
	Port        int
	Keychain    string
	Hotkey      string
	Keymap      map[string]string
	ScrollSpeed uint8         `yaml:"scrollSpeed"`
	SettleMs    time.Duration `yaml:"settleMs"`
}

func NewConfig(path, name string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var wrapper struct {
		Config map[string]Config
	}
	if err := yaml.NewDecoder(file).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("Unable to decode config: %s", err)
	}

	config, ok := wrapper.Config[name]
	if !ok {
		return nil, fmt.Errorf("Config named '%v' not found.", name)
	}
	config.SettleMs = config.SettleMs * time.Millisecond
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateConfig(c Config) error {
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

type Event struct {
	Name   string
	Key    uint32
	Button uint8
	IsKey  bool
}

func newKeyEvent(key uint32) (*Event, error) {
	e := Event{}
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

func newButtonEvent(button uint8) (*Event, error) {
	e := Event{}
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

func findEvent(name string) (*Event, error) {
	e := Event{Name: name}
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

func debugEvent(log Logger, state string, isKey bool, name string, x, y uint16, isPress bool) {
	if isKey {
		log.Debugf("%v key event: %v, press: %v", state, name, isPress)
	} else {
		log.Debugf("%v pointer event: %v, coords: %v %v", state, name, x, y)
	}
}
