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

type Event struct {
	Name   string
	Key    uint32
	Button uint8
	IsKey  bool
}

func DebugEvent(log Logger, state string, isKey bool, name string, x, y uint16, isPress bool) {
	if isKey {
		log.Debugf("%v key event: %v, press: %v", state, name, isPress)
	} else {
		log.Debugf("%v pointer event: %v, coords: %v %v", state, name, x, y)
	}
}
