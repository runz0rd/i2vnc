package x11

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/runz0rd/i2vnc"
)

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

func validateConfig(c *i2vnc.Config) error {
	_, _, err := getConfigDefs(c.Hotkey)
	if err != nil {
		return err
	}
	for from, to := range c.Keymap {
		_, _, err = getConfigDefs(from)
		if err != nil {
			return err
		}
		_, _, err = getConfigDefs(to)
		if err != nil {
			return err
		}
		if from == c.Hotkey || to == c.Hotkey {
			return fmt.Errorf("You shouldn't remap your hotkey.")
		}
	}
	return nil
}

func findEventDef(name string) (*i2vnc.EventDefintion, error) {
	ed := i2vnc.EventDefintion{Name: name}
	var ok bool

	ed.IsKey = true
	ed.Key, ok = Keysyms[name]
	if !ok {
		ed.IsKey = false
		ed.Button, ok = Buttons[name]
		if !ok {
			return nil, fmt.Errorf("No button or keysym definition found for '%v'", name)
		}
	}
	return &ed, nil
}

func newKeyEventDef(key uint32) (*i2vnc.EventDefintion, error) {
	e := i2vnc.EventDefintion{}
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

func newButtonEventDef(button uint8) (*i2vnc.EventDefintion, error) {
	e := i2vnc.EventDefintion{}
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

func getConfigDefs(value string) (*i2vnc.EventDefintion, *[]i2vnc.EventDefintion, error) {
	e := i2vnc.Event{}
	names := strings.Split(value, "+")
	for _, name := range names {
		def, err := findEventDef(name)
		if err != nil {
			return nil, nil, err
		}
		e.HandleEvent(*def, true)
	}
	return &e.Def, &e.Mods, nil
}

func resolveMapping(mapping map[string]string, e i2vnc.Event) *i2vnc.Event {
	for from, to := range mapping {
		fromDef, fromMods, _ := getConfigDefs(from)
		if *fromDef == e.Def && reflect.DeepEqual(*fromMods, e.Mods) {
			toDef, toMods, _ := getConfigDefs(to)
			e.Def = *toDef
			e.Mods = *toMods
		}
	}
	return &e
}
