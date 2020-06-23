package i2vnc

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/runz0rd/i2vnc/x11"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type WindowSystem uint8

const (
	X11WindowSystem         = iota
	LoggerFieldRemote       = "remote"
	LoggerFieldInput        = "input"
	LoggerFieldSource       = "source"
	LoggerFieldEvent        = "event"
	LoggerFieldEventKey     = "key"
	LoggerFieldEventButton  = "button"
	LoggerFieldName         = "name"
	LoggerFieldEventIsPress = "isPress"
	LoggerFieldEventCoords  = "coords"
	LoggerFieldEventX11     = "x11"
)

type Input interface {
	Grab() error
	Ungrab() error
	Screen() Screen
}

type Remote interface {
	IsConnected() bool
	Connect(cname string, timeout time.Duration) error
	Disconnect() error
	Screen() Screen
	SendKeyEvent(name string, key uint32, isPress bool) error
	SendPointerEvent(name string, button uint8, x, y uint16, isPress bool) error
}

type Config map[string]configItem

func (c Config) getItem(name string) (configItem, error) {
	item, ok := c[name]
	if !ok {
		return configItem{}, fmt.Errorf("couldnt find config defined with name %v", name)
	}
	return item, nil
}

type configItem struct {
	Name        string
	Server      string
	Port        int
	Pw          string
	Hotkey      string
	Keymap      map[string]string
	ScrollSpeed uint8         `yaml:"scrollSpeed"`
	settle      time.Duration `yaml:"settleMs"`
	timeout     time.Duration `yaml:"timeoutSec"`
}

func (c *configItem) SetPw(value string) {
	if value != "" {
		c.Pw = value
	}
}

func (c configItem) SettleMs() time.Duration {
	return c.settle * time.Millisecond
}

func (c configItem) TimeoutSec() time.Duration {
	return c.timeout * time.Second
}

func (c configItem) validate() error {
	_, err := getConfigDefs(c.Hotkey)
	if err != nil {
		return err
	}
	for from, to := range c.Keymap {
		_, err = getConfigDefs(from)
		if err != nil {
			return err
		}
		_, err := getConfigDefs(to)
		if err != nil {
			return err
		}
		// if len(toDefs) > 1 {
		// 	return fmt.Errorf("Mapping 'to' value should be a single key or button.")
		// }
		if from == c.Hotkey || to == c.Hotkey {
			return fmt.Errorf("you shouldn't remap your hotkey")
		}
	}
	return nil
}

func getConfigDefs(value string) ([]EventDef, error) {
	var defs []EventDef
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

func LoadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %s", err)
	}
	for name, c := range config {
		c.Name = name
		config[name] = c
	}
	return config, nil
}

var modNames = []string{
	"Control_L", "Control_R", "Alt_L", "Alt_R", "Super_L",
	"Super_R", "Shift_L", "Shift_R", "Meta_L", "Meta_R"}

func isMod(ed EventDef) bool {
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

type EventDef struct {
	Name   string
	Key    uint32
	Button uint8
	IsKey  bool
}

type event struct {
	defs        []EventDef
	mods        map[string]EventDef
	remote      Screen
	local       Screen
	isPress     bool
	defMapping  map[string]string
	scrollSpeed uint8
}

func newEvent(defMapping map[string]string, scrollSpeed uint8) *event {
	return &event{
		mods:        make(map[string]EventDef),
		remote:      Screen{},
		local:       Screen{},
		scrollSpeed: scrollSpeed,
	}
}

func (e *event) handle(def EventDef, isPress bool, c configItem) {
	e.isPress = isPress
	e.defs = []EventDef{}
	e.handleMod(def, isPress)
	e.defs = resolveDef(def, c)
	// e.defs = []EventDef{}
	// defs := resolveDef(def, e.defMapping)
	// for _, d := range defs {
	// 	if e.handleMod(def, isPress) {
	// 		return
	// 	}
	// 	if e.handleScrollButton(d, isPress, e.scrollSpeed) {
	// 		return
	// 	}
	// }
	// e.defs = defs
}

func (e event) resolve(c configItem) []EventDef {
	return resolveCombo(e.defs, mapToSlice(e.mods), c.Keymap, e.isPress)
}

func resolveCombo(defs []EventDef, mods []EventDef, defMapping map[string]string, isPress bool) []EventDef {
	unique := edSliceUnique(append(defs, mods...))
	for from, to := range defMapping {
		fromDefs, _ := getConfigDefs(from)
		toDefs, _ := getConfigDefs(to)
		intersect := edIntersection(unique, fromDefs)
		if len(fromDefs) > 1 && len(intersect) == len(unique) {
			return edSliceSortByPress(toDefs, isPress)
		}

	}
	return defs
}

func resolveDef(def EventDef, ci configItem) []EventDef {
	if def.Button == x11.Buttons["Button_Up"] || def.Button == x11.Buttons["Button_Down"] {
		return resolveScrollButton(def, ci.ScrollSpeed)
	}
	for from, to := range ci.Keymap {
		fromDefs, _ := getConfigDefs(from)
		if len(fromDefs) == 1 && fromDefs[0] == def {
			toDefs, _ := getConfigDefs(to)
			return toDefs
		}
	}
	return []EventDef{def}
}

func (e *event) handleMod(def EventDef, isPress bool) bool {
	if isMod(def) {
		if isPress {
			e.mods[def.Name] = def
		} else {
			delete(e.mods, def.Name)
		}
		return true
	}
	return false
}

func resolveScrollButton(def EventDef, scrollSpeed uint8) []EventDef {
	defs := []EventDef{def}
	for i := 1; i < int(scrollSpeed); i++ {
		defs = append(defs, def)
	}
	return defs
}

func (e event) getButtonForMotion() uint8 {
	button := x11.Buttons["Motion"]
	if len(e.defs) == 1 && !e.defs[0].IsKey {
		return e.defs[0].Button
	}
	return button
}

func (e *event) setCoords(x, y uint16, local, remote Screen) {
	e.local.X = x
	e.local.Y = y
	e.remote.X = screenOffset(e.remote.X, x, local.X, remote.X)
	e.remote.Y = screenOffset(e.remote.Y, y, local.Y, remote.Y)
}

func screenOffset(value, offset, localMax, remoteMax uint16) uint16 {
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

func edSliceSortByPress(s []EventDef, isPress bool) []EventDef {
	sortMod := func(i, j int) bool {
		if isMod(s[i]) && !isMod(s[j]) {
			return true
		}
		return false
	}
	sortModReverse := func(i, j int) bool {
		if isMod(s[i]) && !isMod(s[j]) {
			return false
		}
		return true
	}
	less := sortMod
	if !isPress {
		less = sortModReverse
	}
	sort.Slice(s, less)
	return s
}

func edIntersection(a []EventDef, b []EventDef) []EventDef {
	var intersection []EventDef
	for _, x := range a {
		for _, y := range b {
			if reflect.DeepEqual(x, y) {
				intersection = append(intersection, x)
			}
		}
	}
	return intersection
}

func edSliceUnique(s []EventDef) []EventDef {
	exists := map[EventDef]bool{}
	for i := 0; i < len(s); i++ {
		if !exists[s[i]] {
			exists[s[i]] = true
			continue
		}
		s = append(s[:i], s[i+1:]...)
	}
	return s
}

func newEventDef(key uint32, button uint8, isKey bool) (*EventDef, error) {
	name, err := x11.FindDefName(key, button, isKey)
	if err != nil {
		return nil, err
	}
	return &EventDef{name, key, button, isKey}, nil
}

func newEventDefByName(name string) (*EventDef, error) {
	key, button, isKey, err := x11.FindDefValue(name)
	if err != nil {
		return nil, err
	}
	return &EventDef{name, key, button, isKey}, nil
}

func DebugEvent(l *logrus.Entry, source string, isKey bool, name string, x, y uint16, isPress bool) {
	event := LoggerFieldEventButton
	if isKey {
		event = LoggerFieldEventKey
	}
	l = l.WithFields(logrus.Fields{
		LoggerFieldEventCoords:  fmt.Sprintf("%v %v", x, y),
		LoggerFieldEventIsPress: isPress,
		LoggerFieldName:         name,
		LoggerFieldEvent:        event,
	})
	l.Debug(source)
}

func DebugX11Event(l *logrus.Entry, source string, state uint16, keycode xproto.Keycode, button uint8, x, y int16, isPress bool) {
	l = l.WithFields(logrus.Fields{
		LoggerFieldEventCoords:  fmt.Sprintf("%v %v", x, y),
		LoggerFieldEventIsPress: isPress,
		LoggerFieldEventX11:     fmt.Sprintf("state:%v keycode:%v button:%v", state, keycode, button),
	})
	l.Debug(source)
}

func StringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func Try(l *logrus.Entry, times int, f func(i int) error) {
	i := 1
	for {
		err := f(i)
		if err == nil || i == times {
			return
		}
		l.Warn(err)
		i++
	}
}

func mapToSlice(m map[string]EventDef) []EventDef {
	var slice []EventDef
	for _, i := range m {
		slice = append(slice, i)
	}
	return slice
}
