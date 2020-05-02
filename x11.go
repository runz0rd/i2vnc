package i2vnc

import (
	"fmt"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/kward/go-vnc/keys"
)

type X11Input struct {
	log     Logger
	xu      *xgbutil.XUtil
	remote  Remote
	pointer *Pointer
	config  *Config
}

func NewX11Input(log Logger, remote Remote, config *Config) (*X11Input, error) {
	// create X connection
	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("Could not connect to X: %s", err)
	}
	// set the pointer to the middle of remote screen
	pointer := newPointer(xu.Screen().WidthInPixels, xu.Screen().HeightInPixels,
		remote.ScreenW(), remote.ScreenH())

	return &X11Input{log, xu, remote, pointer, config}, nil
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
	if grabbed, err := mousebind.GrabPointer(i.xu, w, xproto.WindowNone, xproto.CursorNone); !grabbed {
		return fmt.Errorf("Could not grab pointer: %s", err)
	}

	// set the pointer location to the middle of remote screen
	i.remote.SendPointerEvent("Center", 0, i.pointer.X, i.pointer.Y)

	// connect event handlers
	xevent.KeyPressFun(i.handleKeyPress).Connect(i.xu, w)
	xevent.KeyReleaseFun(i.handleKeyRelease).Connect(i.xu, w)
	xevent.ButtonPressFun(i.handleButtonPress).Connect(i.xu, w)
	xevent.ButtonReleaseFun(i.handleButtonRelease).Connect(i.xu, w)
	xevent.MotionNotifyFun(i.handleMotionNotify).Connect(i.xu, w)

	// start X event loop
	i.log.Printf("Program initialized. Start pressing keys!")
	xevent.Main(i.xu)
	return nil
}

func (i X11Input) warpPointerToScreenMid() {
	xproto.WarpPointer(i.xu.Conn(), xproto.WindowNone,
		i.xu.RootWin(), 0, 0, 0, 0, int16(i.pointer.midW), int16(i.pointer.midH))
}

func (i X11Input) handleKeyPress(xu *xgbutil.XUtil, e xevent.KeyPressEvent) {
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
	if e.EventX != int16(i.pointer.PrevX) || e.EventY != int16(i.pointer.PrevY) {
		i.warpPointerToScreenMid()
		// keeps the cursor in the local screen center.
		// needed for hitting the end on local screens while using a larger remote screen
	}
	i.pointer.setPrev(uint16(e.EventX), uint16(e.EventY))
	i.handlePointerEvent(i.pointer.Btn, true, e.EventX, e.EventY)
}

func (i X11Input) sendScrollButtonEvent(e *Event, isPress bool) bool {
	// handle scroll button speed
	if isPress && (e.Button == 4 || e.Button == 5) {
		for j := 0; j < int(i.config.ScrollSpeed); j++ {
			i.remote.SendPointerEvent(e.Name, i.pointer.Btn, i.pointer.X, i.pointer.Y)
		}
		return true
	}
	return false
}

func (i X11Input) isHotkey(e xevent.KeyPressEvent) bool {
	if keybind.KeyMatch(i.xu, i.config.Hotkey, e.State, e.Detail) {
		i.log.Printf("Exit hotkey detected. Quitting...")
		xevent.Quit(i.xu)
		return true
	}
	return false
}

func isCapsLocked(key keys.Key, lockedKeys map[keys.Key]bool) bool {
	//todo handle key events with capslock
	return false
}

// compressMotionNotify takes a MotionNotify event, and inspects the event
// queue for any future MotionNotify events that can be received without
// blocking. The most recent MotionNotify event is then returned.
// We need to make sure that the Event, Child, Detail, State, Root
// and SameScreen fields are the same to ensure the same window/action is
// generating events. That is, we are only compressing the RootX, RootY,
// EventX and EventY fields.
// This function is not thread safe, since Peek returns a *copy* of the
// event queue---which could be out of date by the time we dequeue events.
func compressMotionNotify(xu *xgbutil.XUtil, ev xevent.MotionNotifyEvent) xevent.MotionNotifyEvent {

	// We force a round trip request so that we make sure to read all
	// available events.
	xu.Sync()
	xevent.Read(xu, false)

	// The most recent MotionNotify event that we'll end up returning.
	lastE := ev

	// Look through each event in the queue. If it's an event and it matches
	// all the fields in 'ev' that are detailed above, then set it to 'laste'.
	// In which case, we'll also dequeue the event, otherwise it will be
	// processed twice!
	// N.B. If our only goal was to find the most recent relevant MotionNotify
	// event, we could traverse the event queue backwards and simply use
	// the first MotionNotify we see. However, this could potentially leave
	// other MotionNotify events in the queue, which we *don't* want to be
	// processed. So we stride along and just pick off MotionNotify events
	// until we don't see any more.
	for i, ee := range xevent.Peek(xu) {
		if ee.Err != nil { // This is an error, skip it.
			continue
		}

		// Use type assertion to make sure this is a MotionNotify event.
		if mn, ok := ee.Event.(xproto.MotionNotifyEvent); ok {
			// Now make sure all appropriate fields are equivalent.
			if ev.Event == mn.Event && ev.Child == mn.Child &&
				ev.Detail == mn.Detail && ev.State == mn.State &&
				ev.Root == mn.Root && ev.SameScreen == mn.SameScreen {

				// Set the most recent/valid motion notify event.
				lastE = xevent.MotionNotifyEvent{&mn}

				// We cheat and use the stack semantics of defer to dequeue
				// most recent motion notify events first, so that the indices
				// don't become invalid. (If we dequeued oldest first, we'd
				// have to account for all future events shifting to the left
				// by one.)
				defer func(i int) { xevent.DequeueAt(xu, i) }(i)
			}
		}
	}

	// This isn't strictly necessary, but is correct. We should update
	// xgbutil's sense of time with the most recent event processed.
	// This is typically done in the main event loop, but since we are
	// subverting the main event loop, we should take care of it.
	xu.TimeSet(lastE.Time)

	return lastE
}

func resolveMapping(mapping map[string]string, e *Event) *Event {
	for from, to := range mapping {
		fromE, _ := findEvent(from)

		if fromE.Name == e.Name {
			toE, _ := findEvent(to)
			return toE
		}
	}
	return e
}

func (i X11Input) handleKeyEvent(key uint32, isPress bool) {
	event, err := newKeyEvent(key)
	if err != nil {
		i.log.Errorf("%s", err)
		return
	}
	i.sendEvent(event, isPress)
}

func (i X11Input) handlePointerEvent(button uint8, isPress bool, x, y int16) {
	event, err := newButtonEvent(button)
	if err != nil {
		i.log.Errorf("%s", err)
		return
	}
	// i.pointer.Btn = event.Button
	i.pointer.set(uint16(x), uint16(y))
	i.sendEvent(event, isPress)
}

func (i X11Input) sendEvent(e *Event, isPress bool) {
	debugEvent(i.log, "Recieved", e.IsKey, e.Name, i.pointer.X, i.pointer.Y, isPress)
	ne := resolveMapping(i.config.Keymap, e)

	if ne.IsKey {
		i.remote.SendKeyEvent(ne.Name, ne.Key, isPress)
	} else {
		i.pointer.Btn = 0
		if isPress {
			i.pointer.Btn = ne.Button
		}
		if !i.sendScrollButtonEvent(ne, isPress) {
			i.remote.SendPointerEvent(ne.Name, i.pointer.Btn, i.pointer.X, i.pointer.Y)
		}
	}
}
