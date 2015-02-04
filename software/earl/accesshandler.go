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
func (h *AccessHandler) ShutdownHandler() {}

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
			h.backends.doorbellUI.HandleDoorbell(Target(h.t.GetTerminalName()), "")
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

func (h *AccessHandler) HandleTick() {
	// Keypad got a partial code, but never finished with '#'
	if h.clock.Now().Sub(h.lastKeypressTime) > kKeypadTimeout &&
		h.currentCode != "" {
		h.currentCode = ""
		h.t.BuzzSpeaker("L", 500) // indicate timeout
	}
}

// Hashing a value in a way that we can't recover the content of the value,
// but only can compare if we get the same value.
func scrubLogValue(in string) string {
	hashgen := md5.New()
	io.WriteString(hashgen, in)
	return hex.EncodeToString(hashgen.Sum(nil))[0:6]
}

// TODO(hzeller): this guy blocks on OpenDoor() but shouldn't as this
// backlogs our event queue.
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
		h.t.ShowColor("G")
		h.t.BuzzSpeaker("H", 500)
		// Be sparse, don't log user, but keep track of level.
		log.Printf("%s: opened. %s Type=%s",
			target, fyi_origin, user.UserLevel)
		h.backends.physicalActions.OpenDoor(target)
		h.t.ShowColor("")
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
			h.t.ShowColor("R")
		} else {
			// Show blue (='nighttime') for authentication that is
			// just failing due to be outside daytime (or expired).
			// Better than otherwise confusing 'red' feeback.
			h.t.ShowColor("B")
			// Trigger doorbell. Usually if
			// someone is there, they might open the door.
			h.backends.doorbellUI.HandleDoorbell(target, user.Name)
		}
		h.t.BuzzSpeaker("L", 200)
		time.Sleep(500 * time.Millisecond)
		h.t.ShowColor("")
	}
}
