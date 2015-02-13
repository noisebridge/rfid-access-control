// AccessHandler.
//
// A TerminalEventHandler that deals with the terminal mounted to an entrance,
// receiving PIN and RFID events and triggering opening gates and doors (through
// an implementation of interface DoorActions) if appropriate.
// Also user-feedback with LEDs and feedback tones.
// Each entrance has its own independent instance running.
package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"time"
)

type AccessHandler struct {
	backends *Backends
	clock    Clock

	t Terminal // Our terminal we can do operations on

	// Current state
	currentCode        string    // PIN typed so far on keypad
	lastKeypressTime   time.Time // Last touch of key to reset
	currentRFID        string    // Current RFID we received
	nextRFIDActionTime time.Time // Time we have seen the current RFID

	colorShown   bool
	colorOffTime time.Time
}

const (
	kRFIDRepeatDebounce = 300 * time.Millisecond // RFID is repeated. Pace down.
	kKeypadTimeout      = 30 * time.Second       // Timeout: user stopped typing
)

func NewAccessHandler(backends *Backends) *AccessHandler {
	return &AccessHandler{
		backends: backends,
		clock:    RealClock{}}
}

func (h *AccessHandler) Init(t Terminal) {
	h.t = t
}
func (h *AccessHandler) HandleShutdown() {}

func (h *AccessHandler) HandleKeypress(b byte) {
	h.lastKeypressTime = h.clock.Now()
	switch b {
	case '#':
		if h.currentCode != "" {
			h.checkAccess(h.currentCode, "keypad")
			h.currentCode = ""
		} else {
			// As long as we don't have a 4x4 keypad, we
			// use the single '#' to be the doorbell.
			h.backends.appEventBus.Post(&AppEvent{
				Ev:     AppDoorbellTriggerEvent,
				Target: Target(h.t.GetTerminalName()),
				Source: h.t.GetTerminalName(),
				Msg:    "Terminal keypad button.",
			})
		}
	case '*':
		h.currentCode = "" // reset
	default:
		h.currentCode += string(b)
	}
}

func (h *AccessHandler) HandleRFID(rfid string) {
	// The reader might send IDs faster than we can checkAccess()
	// which is problematic, as checkAccess() blocks the event thread.
	// If we get the same ID again, ignore until nextRFIDActionTime
	if rfid == h.currentRFID && h.clock.Now().Before(h.nextRFIDActionTime) {
		return
	}

	h.checkAccess(rfid, "RFID")
	h.currentRFID = rfid
	h.nextRFIDActionTime = h.clock.Now().Add(kRFIDRepeatDebounce)
}

func (h *AccessHandler) HandleAppEvent(event *AppEvent) {
	switch event.Ev {
	case AppOpenRequest:
		// This happens either because we triggered it ourselves,
		// or has been triggered elsewhere, e.g. someone triggered
		// the gate-buzzer button - in that case, we also show green
		// on the respective terminal, making it a round experience.
		if event.Target == Target(h.t.GetTerminalName()) {
			h.setColorForTime("G", 2000*time.Millisecond)
		}
	}
}

func (h *AccessHandler) HandleTick() {
	now := h.clock.Now()
	// Keypad got a partial code, but never finished with '#'
	if now.Sub(h.lastKeypressTime) > kKeypadTimeout && h.currentCode != "" {
		h.currentCode = ""
		h.t.BuzzSpeaker("L", 500) // indicate timeout
	}
	if h.colorShown && now.After(h.colorOffTime) {
		h.t.ShowColor("")
		h.colorShown = false
	}
}

// Hashing a value in a way that we can't recover the content of the value,
// but only can compare if we get the same value.
func scrubLogValue(in string) string {
	hashgen := md5.New()
	io.WriteString(hashgen, in)
	return hex.EncodeToString(hashgen.Sum(nil))[0:6]
}

func (h *AccessHandler) setColorForTime(color string, duration time.Duration) {
	h.t.ShowColor(color)
	h.colorShown = true
	h.colorOffTime = h.clock.Now().Add(duration)
}

func (h *AccessHandler) checkAccess(code string, fyi_origin string) {
	// Don't bother with too short codes. In particular, don't buzz
	// or flash lights to not to seem overly interactive.
	if !hasMinimalCodeRequirements(code) {
		return
	}
	target := Target(h.t.GetTerminalName())
	user := h.backends.authenticator.FindUser(code)
	auth_result, msg := h.backends.authenticator.AuthUser(code, target)
	if user != nil && auth_result == AuthOk {
		h.t.BuzzSpeaker("H", 500)
		// Be sparse, don't log user, but keep track of level.
		log.Printf("%s: granted. %s Type=%s",
			target, fyi_origin, user.UserLevel)
		h.backends.appEventBus.Post(&AppEvent{
			Ev:     AppOpenRequest,
			Target: target,
			Source: h.t.GetTerminalName(),
			Msg:    "Opening for " + string(user.UserLevel),
		})
		// Note, this will automatically trigger the green LED as
		// we subsequently receive the AppOpenRequest ourselves.
	} else {
		// This is either an invalid RFID (or used outside the
		// validity), or a PIN-code, which is not valid for user
		// access.
		// In that case, we log a scrubbed code - we won't be able
		// to recover the code (we don't store the plain code anywhere
		// to create a reverse table), but can see patterns when the
		// same thing happens multiple times.
		log.Printf("%s: denied. %s | %s (%s)",
			target, msg, fyi_origin, scrubLogValue(code))
		if auth_result == AuthFail {
			h.setColorForTime("R", 500*time.Millisecond)
		} else {
			// Show blue (='nighttime') for authentication that is
			// just failing due to be outside daytime (or expired).
			// Better than otherwise confusing 'red' feeback.
			h.setColorForTime("B", 1000*time.Millisecond)
			// Trigger doorbell artificially. Usually if
			// someone is in the space, they might open the door.
			h.backends.appEventBus.Post(&AppEvent{
				Ev:     AppDoorbellTriggerEvent,
				Target: target,
				Source: h.t.GetTerminalName(),
				Msg:    "@night:" + user.Name,
			})
		}
		h.t.BuzzSpeaker("L", 200)
	}
}
