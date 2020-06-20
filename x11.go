package i2vnc

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/runz0rd/i2vnc/x11"
	"github.com/sirupsen/logrus"
)

type X11Input struct {
	l  *logrus.Entry
	xu *xgbutil.XUtil
	r  Remote
	e  *Event
	c  *Config
}

func NewX11Input(logger *logrus.Logger, r Remote, c *Config) (*X11Input, error) {
	// validate config
	if err := c.validate(); err != nil {
		return nil, err
	}
	// create X connection
	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("could not connect to X: %s", err)
	}
	// create a new event
	e := NewEvent(c,
		xu.Screen().WidthInPixels,
		xu.Screen().HeightInPixels,
		r.ScreenW(),
		r.ScreenH(),
	)
	l := logrus.NewEntry(logger)
	return &X11Input{l, xu, r, e, c}, nil
}

func (i X11Input) Grab() error {
	// use current root window
	w := i.xu.RootWin()

	// grab keyboard and pointer
	keybind.Initialize(i.xu)
	if err := keybind.GrabKeyboard(i.xu, w); err != nil {
		return fmt.Errorf("could not grab keyboard: %s", err)
	}
	mousebind.Initialize(i.xu)
	if grabbed, err := mousebind.GrabPointer(i.xu, w, xproto.WindowNone,
		xproto.CursorNone); !grabbed {
		return fmt.Errorf("could not grab pointer: %s", err)
	}

	// set coords to middle of remote screen
	i.e.SetToScreenMid(i.r.ScreenW(), i.r.ScreenH())
	// set the local pointer to the middle of local screen
	i.warpPointerToScreenMid()
	// set the remote pointer to the middle of remote screen
	i.r.SendPointerEvent("motion", 0, i.e.Coords.X, i.e.Coords.Y, false)

	// connect event handlers
	xevent.KeyPressFun(i.handleKeyPress).Connect(i.xu, w)
	xevent.KeyReleaseFun(i.handleKeyRelease).Connect(i.xu, w)
	xevent.ButtonPressFun(i.handleButtonPress).Connect(i.xu, w)
	xevent.ButtonReleaseFun(i.handleButtonRelease).Connect(i.xu, w)
	xevent.MotionNotifyFun(i.handleMotionNotify).Connect(i.xu, w)

	// start X event loop
	i.l.Infof("grab successful")
	i.l.Infof("press %q to ungrab", i.c.Hotkey)
	xevent.Main(i.xu)
	return nil
}

func (i X11Input) warpPointerToScreenMid() {
	xproto.WarpPointer(i.xu.Conn(), xproto.WindowNone, i.xu.RootWin(), 0, 0, 0, 0,
		int16(i.xu.Screen().WidthInPixels/2), int16(i.xu.Screen().HeightInPixels/2))
}

func (i X11Input) handleKeyPress(xu *xgbutil.XUtil, e xevent.KeyPressEvent) {
	i.handleKeyEvent(e.State, e.Detail, true)
}

func (i X11Input) handleKeyRelease(xu *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
	i.handleKeyEvent(e.State, e.Detail, false)
}

func (i X11Input) handleButtonPress(xu *xgbutil.XUtil, e xevent.ButtonPressEvent) {
	i.handlePointerEvent(e.State, uint8(e.Detail), true, e.EventX, e.EventY)
}

func (i X11Input) handleButtonRelease(xu *xgbutil.XUtil, e xevent.ButtonReleaseEvent) {
	i.handlePointerEvent(e.State, uint8(e.Detail), false, e.EventX, e.EventY)
}

func (i X11Input) handleMotionNotify(xu *xgbutil.XUtil, e xevent.MotionNotifyEvent) {
	// limit number of motion events,
	// large number can make handler lag
	e = x11.CompressMotionNotify(xu, e)
	// activate warp only if there are changes to prevX or prevY
	// avoids the endless motionNotifyEvent loop
	if e.EventX != int16(i.e.PrevCoords.X) || e.EventY != int16(i.e.PrevCoords.Y) {
		i.warpPointerToScreenMid()
		// keeps the cursor in the local screen center.
		// needed for hitting the end on local screens while using a larger remote screen
	}
	// the current button and isPress must be sent along with
	// motion events in order for drag to work
	button := x11.Buttons["Motion"]
	c := i.e.getCombo()
	if len(c) > 0 {
		button = c[0].Button
	}
	i.handlePointerEvent(e.State, button, i.e.IsPress, e.EventX, e.EventY)
}

func (i X11Input) handleKeyEvent(state uint16, keycode xproto.Keycode, isPress bool) {
	DebugX11Event(i.l, "X11Input", state, keycode, 0, 0, 0, isPress)
	keysym := keybind.KeysymGet(i.xu, keycode, 0)
	shifted := keybind.KeysymGet(i.xu, keycode, 1)
	if strings.Contains(keybind.ModifierString(state), "shift") && shifted != 0 {
		// only for shiftable characters, since mods shifted keycode is 0
		keysym = shifted
	}

	kdef, err := newEventDef(uint32(keysym), 0, true)
	if err != nil {
		i.l.WithError(err).Error("handleKeyEvent failed")
		return
	}
	i.e.HandleEvent(*kdef, isPress)
	if i.handleHotkey() {
		return
	}
	i.sendEvents()
}

func (i X11Input) handlePointerEvent(state uint16, button uint8, isPress bool, x, y int16) {
	DebugX11Event(i.l, "X11Input", state, 0, button, x, y, isPress)
	bdef, err := newEventDef(0, button, false)
	if err != nil {
		i.l.WithError(err).Error("handlePointerEvent failed")
		return
	}

	i.e.HandleEvent(*bdef, isPress)
	i.e.SetPrevCoords(uint16(x), uint16(y))
	i.e.SetCoords(uint16(x), uint16(y))
	i.sendEvents()
}

func (i X11Input) sendEvents() {
	//todo bug: sending shift+'(") when using home/end
	for _, c := range i.e.getCombo() {
		if c.IsKey {
			i.r.SendKeyEvent(c.Name, c.Key, i.e.IsPress)
		} else {
			i.r.SendPointerEvent(c.Name, c.Button,
				i.e.Coords.X, i.e.Coords.Y, i.e.IsPress)
		}
	}
}

func (i X11Input) handleHotkey() bool {
	hotkeyDefs, _ := getConfigDefs(i.c.Hotkey)
	compareTo := edSliceUnique(append(i.e.Mods, i.e.getCombo()...))
	if reflect.DeepEqual(hotkeyDefs, compareTo) {
		i.l.Infof("Hotkey %s pressed. Bye!", i.c.Hotkey)
		xevent.Quit(i.xu)
		return true
	}
	return false
}
