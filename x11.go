package i2vnc

import (
	"fmt"
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
	l       *logrus.Entry
	xu      *xgbutil.XUtil
	r       Remote
	c       Config
	ci      configItem
	e       *event
	forever bool
}

func NewX11Input(logger *logrus.Logger, r Remote, c Config, forever bool) (*X11Input, error) {
	l := logrus.NewEntry(logger)
	// create X connection
	l.Infof("connecting to X server")
	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, err
	}
	ci := configItem{}
	e := newEvent(ci.Keymap, ci.ScrollSpeed)
	return &X11Input{l, xu, r, c, ci, e, forever}, nil
}

func (i *X11Input) Grab() error {
	i.l.Infof("grabbing input")
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

	// connect event handlers
	xevent.KeyPressFun(i.handleKeyPress).Connect(i.xu, w)
	xevent.KeyReleaseFun(i.handleKeyRelease).Connect(i.xu, w)
	xevent.ButtonPressFun(i.handleButtonPress).Connect(i.xu, w)
	xevent.ButtonReleaseFun(i.handleButtonRelease).Connect(i.xu, w)
	xevent.MotionNotifyFun(i.handleMotionNotify).Connect(i.xu, w)

	// set the local pointer to the middle of local screen
	i.warpPointer(int16(i.xu.Screen().WidthInPixels/2), int16(i.xu.Screen().HeightInPixels/2))
	// set the remote pointer to the middle of remote screen
	i.r.SendPointerEvent("motion", 0, i.e.remote.X, i.e.remote.Y, false)

	i.l.Infof("grabbed! press a hotkey to connect")
	// start X event loop
	xevent.Main(i.xu)
	return nil
}

func (i *X11Input) Ungrab() error {
	xevent.Quit(i.xu)
	return nil
}

func (i *X11Input) Screen() Screen {
	return Screen{i.xu.Screen().WidthInPixels, i.xu.Screen().HeightInPixels}
}

func (i *X11Input) switchRemote(cname string) error {
	if err := i.r.Disconnect(); err != nil {
		return err
	}
	ci, err := i.c.getItem(cname)
	if err != nil {
		return err
	}
	if err := i.r.Connect(cname, ci.timeout); err != nil {
		return err
	}
	i.ci = ci

	// set coords to middle of remote screen
	remoteScreen := i.r.Screen()
	i.e.setCoords(remoteScreen.X/2, remoteScreen.Y/2, i.Screen(), remoteScreen)
	// set the remote pointer to the middle of remote screen
	i.handlePointerEvent(0, i.e.getButtonForMotion(), int16(i.e.remote.X), int16(i.e.remote.Y), false)

	// i.r.SendPointerEvent("motion", 0, i.r.ScreenW()/2, i.r.ScreenH()/2, false)
	return nil
}

func (i *X11Input) warpPointer(x, y int16) {
	xproto.WarpPointer(i.xu.Conn(), xproto.WindowNone, i.xu.RootWin(), 0, 0, 0, 0, x, y)
}

func (i *X11Input) handleKeyPress(xu *xgbutil.XUtil, e xevent.KeyPressEvent) {
	i.handleKeyEvent(e.State, e.Detail, true)
}

func (i *X11Input) handleKeyRelease(xu *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
	i.handleKeyEvent(e.State, e.Detail, false)
}

func (i *X11Input) handleButtonPress(xu *xgbutil.XUtil, e xevent.ButtonPressEvent) {
	i.handlePointerEvent(e.State, uint8(e.Detail), e.EventX, e.EventY, true)
}

func (i *X11Input) handleButtonRelease(xu *xgbutil.XUtil, e xevent.ButtonReleaseEvent) {
	i.handlePointerEvent(e.State, uint8(e.Detail), e.EventX, e.EventY, false)
}

func (i *X11Input) handleMotionNotify(xu *xgbutil.XUtil, e xevent.MotionNotifyEvent) {
	// limit number of motion events,
	// large number can make handler lag
	e = x11.CompressMotionNotify(xu, e)

	// activate warp only if there are changes to prevX or prevY
	// avoids the endless motionNotifyEvent loop
	if e.EventX != int16(i.e.local.X) || e.EventY != int16(i.e.local.Y) {
		// keeps the cursor in the local screen center.
		// needed for hitting the end on local screens while using a larger remote screen
		i.warpPointer(int16(i.xu.Screen().WidthInPixels/2), int16(i.xu.Screen().HeightInPixels/2))
	}
	// the current button and isPress must be sent along with
	// motion events in order for drag to work
	button := i.e.getButtonForMotion()
	i.handlePointerEvent(e.State, button, e.EventX, e.EventY, i.e.isPress)
}

func (i *X11Input) handleKeyEvent(state uint16, keycode xproto.Keycode, isPress bool) {
	// DebugX11Event(i.l, "X11Input", state, keycode, 0, 0, 0, isPress)
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
	i.e.handle(*kdef, isPress, i.ci)
	if i.handleHotkeys() {
		return
	}
	i.sendEvent()
}

func (i *X11Input) handlePointerEvent(state uint16, button uint8, x, y int16, isPress bool) {
	// DebugX11Event(i.l, "X11Input", state, 0, button, x, y, isPress)
	bdef, err := newEventDef(0, button, false)
	if err != nil {
		i.l.WithError(err).Error("handlePointerEvent failed")
		return
	}
	i.e.handle(*bdef, isPress, i.ci)

	i.e.setCoords(uint16(x), uint16(y), i.Screen(), i.r.Screen())
	i.sendEvent()
}

func (i *X11Input) sendEvent() {
	for _, def := range i.e.resolve(i.ci) {
		if def.IsKey {
			if err := i.r.SendKeyEvent(def.Name, def.Key, i.e.isPress); err != nil {
				i.l.Warn(err)
			}
		} else {
			if err := i.r.SendPointerEvent(def.Name, def.Button, i.e.remote.X, i.e.remote.Y, i.e.isPress); err != nil {
				i.l.Warn(err)
			}
		}
	}
}

func (i *X11Input) hotkeyPressed(cname, hotkey string) bool {
	hotkeyDefs, err := getConfigDefs(hotkey)
	if err != nil {
		i.l.WithError(err).Warnf("failed getting hotkey for %q", cname)
		return false
	}
	intersect := edIntersection(hotkeyDefs, i.e.resolve(i.ci))
	return len(intersect) == len(hotkeyDefs)
}

func (i *X11Input) handleHotkeys() bool {
	for cname, ci := range i.c {
		if i.hotkeyPressed(cname, ci.Hotkey) {
			if i.e.isPress {
				return true
			}
			if !i.forever && i.r.IsConnected() && cname == i.ci.Name {
				i.l.Infof("caught %q, disconnecting fom %q", ci.Hotkey, cname)
				xevent.Quit(i.xu)
				i.r.Disconnect()
				return true
			}
			i.l.Infof("caught %q, switching to %q", ci.Hotkey, cname)
			if err := i.switchRemote(cname); err != nil {
				// fmt.Print("\a") // bell terminal ring
				i.l.Warn(err)
			}
			return true
		}
	}
	return false
}
