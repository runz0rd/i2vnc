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

	return &config, nil
}

type Screen struct {
	X uint16
	Y uint16
}

type EventDefintion struct {
	Name   string
	Key    uint32
	Button uint8
	IsKey  bool
}

type Event struct {
	Def        EventDefintion
	Coords     Screen
	PrevCoords Screen
	localMax   Screen
	remoteMax  Screen
	IsPress    bool
	IsLocked   bool
}

func NewEvent(localMax Screen, remoteMax Screen) *Event {
	return &Event{
		localMax:  localMax,
		remoteMax: remoteMax,
	}
}
func (e *Event) SetToScreenMid(screenMaxW, screenMaxH uint16) {
	e.Coords.X = screenMaxW / 2
	e.Coords.Y = screenMaxH / 2
}

func (e *Event) SetCoords(x, y uint16) {
	e.Coords.X = e.calcOffset(e.Coords.X, x, e.localMax.X, e.remoteMax.X)
	e.Coords.Y = e.calcOffset(e.Coords.Y, y, e.localMax.Y, e.remoteMax.Y)
}

func (e *Event) SetPrevCoords(x, y uint16) {
	e.PrevCoords.X = x
	e.PrevCoords.Y = y
}

func (e *Event) calcOffset(value, offset, localMax, remoteMax uint16) uint16 {
	value += offset - localMax/2
	if value < 1 || value > 65535/2 {
		// when going under 0, the value starts going down from max uint16
		// if it is in the upper half of max uint16, reset to 0
		value = 0
	}
	if value >= remoteMax && value <= 65535/2 {
		// if it is in the lower half of max uint16, reset to max
		value = remoteMax
	}
	return value
}

func DebugEvent(log Logger, state string, isKey bool, name string, x, y uint16, isPress bool) {
	if isKey {
		log.Debugf("%v key event: %v, press: %v", state, name, isPress)
	} else {
		log.Debugf("%v pointer event: %v, coords: %v %v", state, name, x, y)
	}
}
