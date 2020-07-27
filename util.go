package i2vnc

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/runz0rd/i2vnc/x11"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
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

type configMap struct {
	from []string
	to   []string
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

func (c configItem) getConfigMaps() []configMap {
	var cms []configMap
	for key, value := range c.Keymap {
		cms = append(cms, configMap{strings.Split(key, "+"), strings.Split(value, "+")})
	}
	return cms
}

func (c configItem) validate() error {
	_, err := getConfigDefs(c.Hotkey, false)
	if err != nil {
		return err
	}
	for from, to := range c.Keymap {
		_, err = getConfigDefs(from, false)
		if err != nil {
			return err
		}
		_, err := getConfigDefs(to, false)
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

func getConfigDefs(value string, isPress bool) ([]EventDef, error) {
	var defs []EventDef
	names := strings.Split(value, "+")
	for _, name := range names {
		def, err := newEventDefByName(name, isPress)
		if err != nil {
			return nil, err
		}
		defs = append(defs, *def)
	}
	return defs, nil
}

func LoadConfig(path string) (Config, error) {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		dir := usr.HomeDir
		path = filepath.Join(dir, path[1:])
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
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
	"Super_R", "Shift_L", "Shift_R", "Meta_L", "Meta_R", "Caps_Lock"}

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
	Name    string
	Key     uint32
	Button  uint8
	IsKey   bool
	IsPress bool
}

type event struct {
	def           EventDef
	modMap        map[string]string
	skipOnRelease []string
	remote        Screen
	local         Screen
	scrollSpeed   uint8
}

func newEvent(defMapping map[string]string, scrollSpeed uint8) *event {
	return &event{
		modMap:      map[string]string{},
		remote:      Screen{},
		local:       Screen{},
		scrollSpeed: scrollSpeed,
	}
}

func (e *event) handle(def EventDef) {
	e.handleMod(def)
	e.def = def
}

func (e *event) resolve(cms []configMap, scrollSpeed uint8) []EventDef {
	if e.def.Button == x11.Buttons["Button_Up"] || e.def.Button == x11.Buttons["Button_Down"] {
		return resolveScrollButton(e.def, scrollSpeed)
	}
	resolved, skipOnRelease := resolveCombination(e.combination(), cms, e.def.IsPress)
	if e.def.IsPress {
		e.skipOnRelease = skipOnRelease
	} else {
		if handleSkipOnRelease(e.def.Name, &e.skipOnRelease, cms) {
			return nil
		}
	}
	// dont return already pressed mods
	if len(resolved) > 1 {
		for i := 0; i < len(resolved); i++ {
			_, ok := e.modMap[resolved[i].Name]
			if ok && resolved[i].IsPress == e.def.IsPress {
				resolved = append(resolved[:i], resolved[i+1:]...)
			}
		}
	}
	return resolved
}

func (e event) combination() []string {
	var mods []string
	for _, mod := range e.modMap {
		mods = append(mods, mod)
	}
	return stringSliceUnique(append(mods, e.def.Name))
}

func (e *event) handleMod(def EventDef) bool {
	if isMod(def) {
		if def.IsPress {
			e.modMap[def.Name] = def.Name
		} else {
			delete(e.modMap, def.Name)
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

func (e event) getLastEventIsPress() bool {
	return e.def.IsPress
}

func (e event) getButtonForMotion() uint8 {
	if e.def.IsKey {
		return x11.Buttons["Motion"]
	}
	return e.def.Button
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
			if x.Name == y.Name && x.IsPress == y.IsPress {
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

func newEventDef(key uint32, button uint8, isKey, isPress bool) (*EventDef, error) {
	name, err := x11.FindDefName(key, button, isKey)
	if err != nil {
		return nil, err
	}
	return &EventDef{name, key, button, isKey, isPress}, nil
}

func newEventDefByName(name string, isPress bool) (*EventDef, error) {
	key, button, isKey, err := x11.FindDefValue(name)
	if err != nil {
		return nil, err
	}
	return &EventDef{name, key, button, isKey, isPress}, nil
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

func stringSliceEquals(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func stringSliceIntersect(a []string, b []string) []string {
	var intersection []string
	for _, x := range a {
		for _, y := range b {
			if x == y {
				intersection = append(intersection, x)
				continue
			}
		}
	}
	return intersection
}

func stringSliceUnique(s []string) []string {
	exists := map[string]bool{}
	for i := 0; i < len(s); i++ {
		if !exists[s[i]] {
			exists[s[i]] = true
			continue
		}
		s = append(s[:i], s[i+1:]...)
	}
	return s
}

func makeEventDefs(names []string, isPress bool) []EventDef {
	var eds []EventDef
	for _, name := range names {
		ed, _ := newEventDefByName(name, isPress)
		eds = append(eds, *ed)
	}
	return eds
}

func resolveDef(def EventDef, configMaps []configMap) EventDef {
	for _, cm := range configMaps {
		if len(cm.from) == 1 && len(cm.to) == 1 && def.Name == cm.from[0] {
			ed, _ := newEventDefByName(cm.to[0], def.IsPress)
			return *ed
		}
	}
	return def
}

func resolveCombination(combination []string, configMaps []configMap,
	isPress bool) (resolved []EventDef, skipOnRelease []string) {
	for _, cm := range configMaps {
		if stringSliceEquals(cm.from, combination) {
			resolved := makeEventDefs(cm.to, isPress)
			modIntersection := stringSliceIntersect(cm.from, modNames)
			if len(modIntersection) > 0 && len(combination) > 1 {
				var resolvedMods []string
				for _, mod := range modIntersection {
					resolvedMod := resolveSingle(mod, configMaps)
					if resolvedMod == "" {
						resolvedMod = mod
					}
					resolvedMods = append(resolvedMods, resolvedMod)
				}
				if isPress {
					modReleaseEds := makeEventDefs(resolvedMods, false)
					resolved = append(modReleaseEds, resolved...)
				}
				return resolved, resolvedMods
			}
			return resolved, nil
		}
	}
	return makeEventDefs(combination, isPress), nil
}

func resolveSingle(name string, configMaps []configMap) string {
	for _, cm := range configMaps {
		if len(cm.from) == 1 && len(cm.to) == 1 {
			if cm.from[0] == name {
				return cm.to[0]
			}
		}
	}
	return ""
}

func handleSkipOnRelease(name string, toSkip *[]string, cms []configMap) bool {
	s := *toSkip
	for i := 0; i < len(s); i++ {
		if resolveSingle(name, cms) == s[i] {
			*toSkip = append(s[:i], s[i+1:]...)
			return true
		}
	}
	return false
}
