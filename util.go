package i2vnc

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/runz0rd/i2vnc/x11"
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
	SendPointerEvent(name string, button uint8, x, y uint16, isPress bool) error
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

func (c *Config) validate() error {
	_, err := getConfigDefs(c.Hotkey)
	if err != nil {
		return err
	}
	for from, to := range c.Keymap {
		_, err = getConfigDefs(from)
		if err != nil {
			return err
		}
		toDefs, err := getConfigDefs(to)
		if err != nil {
			return err
		}
		if len(toDefs) > 1 {
			return fmt.Errorf("Mapping 'to' value should be a single key or button.")
		}
		if from == c.Hotkey || to == c.Hotkey {
			return fmt.Errorf("You shouldn't remap your hotkey.")
		}
	}
	return nil
}

func getConfigDefs(value string) ([]EventDefintion, error) {
	var defs []EventDefintion
	names := strings.Split(value, "+")
	for _, name := range names {
		def, err := newEventDefByName(name)
		if err != nil {
			return nil, err
		}
		defs = append(defs, *def)
	}
	return defs, nil
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

var modNames = [...]string{
	"Control_L", "Control_R", "Alt_L", "Alt_R", "Super_L",
	"Super_R", "Shift_L", "Shift_R", "Meta_L", "Meta_R"}

func IsMod(ed EventDefintion) bool {
	for _, mn := range modNames {
		if mn == ed.Name {
			return true
		}
	}
	return false
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
	def        EventDefintion
	Coords     Screen
	PrevCoords Screen
	localMax   Screen
	remoteMax  Screen
	IsPress    bool
	IsLocked   bool
	Mods       []EventDefintion
	defMapping map[string]string
}

func NewEvent(defMapping map[string]string, localW, localH, remoteW, remoteH uint16) *Event {
	return &Event{
		localMax:   Screen{localW, localH},
		remoteMax:  Screen{remoteW, remoteH},
		defMapping: defMapping,
	}
}

func (e *Event) HandleEvent(def EventDefintion, isPress bool) {
	// resolve single key/button mappings
	// called right after event creation
	// to ensure all mappings are correct
	def = resolveDef(def, e.defMapping)
	e.def = def
	e.IsPress = isPress
	e.handleMods(def, isPress)
	e.handleCapsLock(def, isPress)
}

func (e *Event) getDef() EventDefintion {
	// resolve combo key/button mappings
	// called right before event sending
	// to ensure avoid confusion around combo conversion
	return resolveDefCombo(e.def, e.Mods, e.defMapping)
}

func (e *Event) handleMods(def EventDefintion, isPress bool) {
	for i := 0; i < len(e.Mods); i++ {
		if def.Name == e.Mods[i].Name && !isPress {
			e.Mods = append(e.Mods[:i], e.Mods[i+1:]...)
			return
		}
	}
	if isPress && IsMod(def) {
		e.Mods = append(e.Mods, def)
	}
}

func (e *Event) handleCapsLock(def EventDefintion, isPress bool) {
	if def.Key == x11.Keysyms["Caps_Lock"] {
		if isPress && !e.IsLocked {
			e.IsLocked = true
			return
		}
		if isPress && e.IsLocked {
			e.IsLocked = false
			return
		}
		if !isPress && e.IsLocked {
			// modify caps to do a press instead of release if its locked
			e.IsPress = true
		}
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

func edSliceUnique(s []EventDefintion) []EventDefintion {
	exists := map[EventDefintion]bool{}
	for i := 0; i < len(s); i++ {
		if !exists[s[i]] {
			exists[s[i]] = true
			continue
		}
		s = append(s[:i], s[i+1:]...)
	}
	return s
}

func edSliceContains(s []EventDefintion, find EventDefintion) bool {
	for _, item := range s {
		if reflect.DeepEqual(item, find) {
			return true
		}
	}
	return false
}

func edIntersection(a []EventDefintion, b []EventDefintion) []EventDefintion {
	var intersection []EventDefintion
	for _, aa := range a {
		for _, bb := range b {
			if reflect.DeepEqual(aa, bb) {
				intersection = append(intersection, aa)
			}
		}
	}
	return intersection
}

func newEventDef(key uint32, button uint8, isKey bool) (*EventDefintion, error) {
	name, err := x11.FindDefName(key, button, isKey)
	if err != nil {
		return nil, err
	}
	return &EventDefintion{name, key, button, isKey}, nil
}

func newEventDefByName(name string) (*EventDefintion, error) {
	key, button, isKey, err := x11.FindDefValue(name)
	if err != nil {
		return nil, err
	}
	return &EventDefintion{name, key, button, isKey}, nil
}

func resolveDef(def EventDefintion, defMapping map[string]string) EventDefintion {
	for from, to := range defMapping {
		fromDefs, _ := getConfigDefs(from)
		if len(fromDefs) == 1 && fromDefs[0] == def {
			toDefs, _ := getConfigDefs(to)
			return toDefs[0]
		}
	}
	return def
}

func resolveDefCombo(def EventDefintion, mods []EventDefintion, defMapping map[string]string) EventDefintion {
	combo := edSliceUnique(append(mods, def))
	if len(combo) == 1 {
		return def
	}
	for from, to := range defMapping {
		fromDefs, _ := getConfigDefs(from)
		if reflect.DeepEqual(combo, fromDefs) {
			toDefs, _ := getConfigDefs(to)
			return toDefs[0]
		}
	}
	return def
}

func DebugEvent(log Logger, state string, isKey bool, name string, x, y uint16, isPress bool) {
	if isKey {
		log.Debugf("%v key event: %v, press: %v", state, name, isPress)
	} else {
		log.Debugf("%v pointer event: %v, coords: %v %v, press: %v", state, name, x, y, isPress)
	}
}
