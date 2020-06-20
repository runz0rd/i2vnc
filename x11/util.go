package x11

import (
	"fmt"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
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
func CompressMotionNotify(xu *xgbutil.XUtil, ev xevent.MotionNotifyEvent) xevent.MotionNotifyEvent {

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

func FindDefName(key uint32, button uint8, isKey bool) (string, error) {
	if isKey {
		for k, v := range Keysyms {
			if v == key {
				return k, nil
			}
		}
	} else {
		for k, v := range Buttons {
			if v == button {
				return k, nil
			}
		}
	}
	return "", fmt.Errorf("no definition found for '%v'", button)
}

func FindDefValue(name string) (key uint32, button uint8, isKey bool, err error) {
	k, ok := Keysyms[name]
	if ok {
		return k, 0, true, nil
	}
	b, ok := Buttons[name]
	if ok {
		return 0, b, false, nil
	}
	return 0, 0, false, fmt.Errorf("no button or keysym definition found for '%v'", name)
}
