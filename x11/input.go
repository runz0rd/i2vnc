package x11

import (
	"fmt"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/runz0rd/i2vnc"
)

type X11Input struct {
	log i2vnc.Logger
	xu  *xgbutil.XUtil
	r   i2vnc.Remote
	e   *i2vnc.Event
	c   *i2vnc.Config
}

func NewInput(log i2vnc.Logger, r i2vnc.Remote, c *i2vnc.Config) (*X11Input, error) {
	// validate config
	if err := validateConfig(c); err != nil {
		return nil, err
	}
	// create X connection
	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("Could not connect to X: %s", err)
	}
	// create a new event
	e := i2vnc.NewEvent(xu.Screen().WidthInPixels, xu.Screen().HeightInPixels)

	return &X11Input{log, xu, r, e, c}, nil
}

func (i X11Input) Grab() error {
	// use current root window
	w := i.xu.RootWin()

	// grab keyboard and pointer
	keybind.Initialize(i.xu)
	if err := keybind.GrabKeyboard(i.xu, w); err != nil {
		return fmt.Errorf("Could not grab keyboard: %s", err)
	}
	mousebind.Initialize(i.xu)
	if grabbed, err := mousebind.GrabPointer(i.xu, w, xproto.WindowNone,
		xproto.CursorNone); !grabbed {
		return fmt.Errorf("Could not grab pointer: %s", err)
	}

	// set coords to middle of remote screen
	i.e.SetToScreenMid(i.r.ScreenW(), i.r.ScreenH())
	// set the local pointer to the middle of local screen
	i.warpPointerToScreenMid()
	// set the remote pointer to the middle of remote screen
	i.r.SendPointerEvent("Button_None", 0, i.e.Coords.X, i.e.Coords.Y)

	// connect event handlers
	xevent.KeyPressFun(i.handleKeyPress).Connect(i.xu, w)
	xevent.KeyReleaseFun(i.handleKeyRelease).Connect(i.xu, w)
	xevent.ButtonPressFun(i.handleButtonPress).Connect(i.xu, w)
	xevent.ButtonReleaseFun(i.handleButtonRelease).Connect(i.xu, w)
	xevent.MotionNotifyFun(i.handleMotionNotify).Connect(i.xu, w)

	// start X event loop
	i.log.Printf("Grab successful. Press %s to ungrab.", i.c.Hotkey)
	xevent.Main(i.xu)
	return nil
}

func (i X11Input) warpPointerToScreenMid() {
	xproto.WarpPointer(i.xu.Conn(), xproto.WindowNone, i.xu.RootWin(), 0, 0, 0, 0,
		int16(i.xu.Screen().WidthInPixels/2), int16(i.xu.Screen().HeightInPixels/2))
}

func (i X11Input) handleKeyPress(xu *xgbutil.XUtil, e xevent.KeyPressEvent) {
	// modStr := keybind.ModifierString(e.State)
	// keyStr := keybind.LookupString(xu, e.State, e.Detail)
	// spew.Dump(modStr, keyStr, e.Detail)

	if i.isHotkey(e) {
		return
	}

	key := uint32(keybind.KeysymGet(i.xu, e.Detail, 0))
	i.handleKeyEvent(key, true)
}

func (i X11Input) handleKeyRelease(xu *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
	key := uint32(keybind.KeysymGet(i.xu, e.Detail, 0))
	i.handleKeyEvent(key, false)
}

func (i X11Input) handleButtonPress(xu *xgbutil.XUtil, e xevent.ButtonPressEvent) {
	i.handlePointerEvent(uint8(e.Detail), true, e.EventX, e.EventY)
}

func (i X11Input) handleButtonRelease(xu *xgbutil.XUtil, e xevent.ButtonReleaseEvent) {
	i.handlePointerEvent(0, false, e.EventX, e.EventY)
}

func (i X11Input) handleMotionNotify(xu *xgbutil.XUtil, e xevent.MotionNotifyEvent) {
	// limit number of motion events,
	// large number can make handler lag
	e = compressMotionNotify(xu, e)
	// activate warp only if there are changes to prevX or prevY
	// avoids the endless motionNotifyEvent loop
	if e.EventX != int16(i.e.PrevCoords.X) || e.EventY != int16(i.e.PrevCoords.Y) {
		i.warpPointerToScreenMid()
		// keeps the cursor in the local screen center.
		// needed for hitting the end on local screens while using a larger remote screen
	}
	i.handlePointerEvent(i.e.Def.Button, true, e.EventX, e.EventY)
}

func (i X11Input) handleKeyEvent(key uint32, isPress bool) {
	kdef, err := newKeyEventDef(key)
	if err != nil {
		i.log.Errorf("%s", err)
		return
	}
	i.e.HandleEvent(*kdef, isPress)
	// spew.Dump(i.e)
	i.sendEvent()
}

func (i X11Input) handlePointerEvent(button uint8, isPress bool, x, y int16) {
	bed, err := newButtonEventDef(button)
	if err != nil {
		i.log.Errorf("%s", err)
		return
	}
	i.e.Def = *bed
	i.e.IsPress = isPress
	i.e.SetPrevCoords(uint16(x), uint16(y))

	//todo request update from server
	i.e.SetCoords(uint16(x), uint16(y), i.r.ScreenW(), i.r.ScreenH())
	i.sendEvent()
}

func (i X11Input) isHotkey(e xevent.KeyPressEvent) bool {
	if keybind.KeyMatch(i.xu, i.c.Hotkey, e.State, e.Detail) {
		i.log.Printf("Hotkey %s pressed. Bye!", i.c.Hotkey)
		xevent.Quit(i.xu)
		return true
	}
	return false
}

func (i X11Input) handleScrollButtonEvent(e *i2vnc.Event) bool {
	// handle scroll button speed
	if e.IsPress && (e.Def.Button == 4 || e.Def.Button == 5) {
		for j := 0; j < int(i.c.ScrollSpeed); j++ {
			i.r.SendPointerEvent(e.Def.Name, e.Def.Button, e.Coords.X, i.e.Coords.Y)
		}
		return true
	}
	return false
}

func handleCapsLockEvent(e *i2vnc.Event) bool {
	if e.Def.Key == Keysyms["Caps_Lock"] {
		if e.IsPress && !e.IsLocked {
			e.IsLocked = true
			return false
		}
		if e.IsPress && e.IsLocked {
			e.IsLocked = false
			return false
		}
		if !e.IsPress && e.IsLocked {
			return true
		}
	}
	return false
}

func (i X11Input) sendEvent() {
	i2vnc.DebugEvent(i.log, "Recieved", i.e.Definition().IsKey, i.e.Definition().Name,
		i.e.Coords.X, i.e.Coords.Y, i.e.IsPress)
	mappedE := resolveMapping(i.c.Keymap, *i.e)

	if mappedE.Definition().IsKey {
		capsHandled := handleCapsLockEvent(mappedE)
		if !capsHandled {
			i.r.SendKeyEvent(mappedE.Definition().Name, mappedE.Definition().Key, mappedE.IsPress)
		}
	} else {
		scrollHandled := i.handleScrollButtonEvent(mappedE)
		if !scrollHandled {
			i.r.SendPointerEvent(mappedE.Definition().Name, mappedE.Definition().Button,
				mappedE.Coords.X, mappedE.Coords.Y)
		}
	}
}
