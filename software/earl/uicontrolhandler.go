// UIControlHandler.
// A TerminalEventHandler, that interacts with users in the space.
//
// Its primary goal is not to open doors and such, but to provide an
// user-interface to verify RFID cards and convenient way for members to
// add new users.
//
// It is a regular terminal (same serial protocol), but has a LCD attached as
// output and a Keypad and RFID reader as input.
package main

// TODO
//  - A single member can give a day's pass, 2 members a user
//  - How to enter names for members ? For initiall mass-adding: on console
//  - make this state-machine more readable.
import (
	"fmt"
	"sync"
	"time"
)

type UIState int

const (
	StateIdle               = iota // When there is nothing to do; idle screen.
	StateDisplayInfoMessage        // Interrupt idle screen and show info message
	StateWaitMemberCommand         // Member showed RFID; awaiting instruction
	StateAddAwaitNewRFID           // Member adds new user: wait for new user RFID
	StateUpdateAwaitRFID           // Member updates user: wait for new user RFID
	StateDoorbellRequest           // Someone just rang
)

const (
	// Display doorbell for this amount of time
	showDoorbellDuration = 45 * time.Second

	// After some action has been taken (RFID or snooze), this is the time
	// things remain displayed before going back to idle.
	postDoorbellSnoozeDuration = 3 * time.Second
	postDoorbellRFIDDuration   = 3 * time.Second

	// Don't allow to ring more often than this.
	defaultDoorbellRatelimit = 3 * time.Second

	// For annoying people...
	offerSnoozeWhenRepeatedRingsUnder = 5 * time.Second
	snoozedDoorbellRatelimit          = 60 * time.Second
)

const (
	// We programmed the LCD to show a doorbell pictogram
	DoorBellCharacter = "\001"
)

type UIDoorbellRequest struct {
	target  Target
	message string
}

type UIControlHandler struct {
	backends *Backends
	auth     Authenticator // shortcut, copy of the pointer in backends

	t Terminal

	authUserCode string // current active member code

	state        UIState   // state of our state machine
	stateTimeout time.Time // timeout of current state

	userCounter int // counter to generate new user names.

	// There might be requests do do something on behalf of handlers running
	// in different threads. This is to pass over this request to be handled
	// in the right thread.
	outOfThreadRequest sync.Mutex
	doorbellRequest    *UIDoorbellRequest

	// We allow rate-limiting of the doorbell.
	lastDoorbellRequest   time.Time
	nextAllowdDoorbell    time.Time
	doorbellTarget        Target
	doorWhileSnoozedCount int
}

func NewControlHandler(backends *Backends) *UIControlHandler {
	return &UIControlHandler{
		backends:    backends,
		auth:        backends.authenticator,
		userCounter: time.Now().Second() % 100, // semi-random start
	}
}

// Setting the current state. The state will only last for a while until
// we fall back to the idleScreen
func (u *UIControlHandler) setState(state UIState, timeout_in time.Duration) {
	u.state = state
	u.stateTimeout = time.Now().Add(timeout_in)
}

func (u *UIControlHandler) backToIdle() {
	u.state = StateIdle
	u.authUserCode = ""
	u.displayIdleScreen()
}

func (u *UIControlHandler) Init(t Terminal) {
	u.t = t
	// We sneakily replace the doorbell UI withourself
	// as we boldy claim to do better than the default.
	u.backends.doorbellUI = u
}

func (u *UIControlHandler) HandleShutdown() {
	// Back to some simple UI handler
	u.backends.doorbellUI = &SimpleDoorbellUI{
		actions: u.backends.physicalActions,
	}
}

func (u *UIControlHandler) HandleKeypress(key byte) {
	if key == '*' { // The '*' key is always 'Esc'-equivalent
		u.backToIdle()
		return
	}

	switch u.state {
	case StateWaitMemberCommand:
		switch key {
		case '1':
			u.t.WriteLCD(0, "Read new user RFID")
			u.t.WriteLCD(1, "[*] Cancel")
			u.setState(StateAddAwaitNewRFID, 30*time.Second)

		case '2':
			u.t.WriteLCD(0, "Read user RFID to update")
			u.t.WriteLCD(1, "[*] Cancel")
			u.setState(StateUpdateAwaitRFID, 30*time.Second)
		}

	case StateDoorbellRequest:
		if key == '9' {
			u.nextAllowdDoorbell = time.Now().Add(snoozedDoorbellRatelimit)
			u.t.WriteLCD(1, fmt.Sprintf("Snoozed for %d sec",
				snoozedDoorbellRatelimit/time.Second))
			u.stateTimeout = time.Now().Add(postDoorbellSnoozeDuration)
		}
	}
}

func (u *UIControlHandler) HandleRFID(rfid string) {
	switch u.state {
	case StateIdle:
		user := u.auth.FindUser(rfid)
		if user == nil {
			u.t.WriteLCD(0, "      Unknown RFID")
			u.t.WriteLCD(1, "Ask a member to register")
		} else {
			switch user.UserLevel {
			case LevelMember:
				u.authUserCode = rfid
				u.presentMemberActions(user)

			case LevelUser:
				u.displayUserInfo(user)
			}
		}

	case StateAddAwaitNewRFID:
		// Let's create some name that is somewhat unique to be
		// easy to find in the file later to edit.
		userPrefix := time.Now().Format("0102-15")
		u.userCounter++
		userName := fmt.Sprintf("<u%s%02d>",
			userPrefix, u.userCounter%100)
		newUser := User{
			Name:      userName,
			UserLevel: LevelUser}
		newUser.SetAuthCode(rfid)
		if ok, msg := u.auth.AddNewUser(u.authUserCode, newUser); ok {
			u.t.WriteLCD(0,
				fmt.Sprintf("Success! += %s", userName))
		} else {
			u.t.WriteLCD(0, "Trouble:"+msg)
		}
		u.t.WriteLCD(1, "[*] Done    [1] Add More")
		u.setState(StateWaitMemberCommand, 5*time.Second)

	case StateUpdateAwaitRFID:
		updateUser := u.auth.FindUser(rfid)
		if updateUser == nil {
			u.t.WriteLCD(0, "Unknown RFID")
		} else if updateUser.ExpiryDate(time.Now()).IsZero() {
			u.t.WriteLCD(0, fmt.Sprintf("%s does not expire", updateUser.Name))
		} else {
			// TODO: maybe ask for confirmation ?
			u.auth.UpdateUser(u.authUserCode, rfid,
				func(user *User) bool {
					user.ValidFrom = time.Now()
					return true
				})
			updateUser = u.auth.FindUser(rfid)
			newExp := updateUser.ExpiryDate(time.Now()).Format("Jan 02")
			u.t.WriteLCD(0, fmt.Sprintf("Extended to %s", newExp))
		}
		u.t.WriteLCD(1, "[*] Done [2] Update More")
		u.setState(StateWaitMemberCommand, 5*time.Second)

	case StateDoorbellRequest:
		// Opening doors is somewhat relaxed; if the person is inside
		// we assume they are allowed to open the door. TODO: revisit?
		if u.auth.FindUser(rfid) != nil {
			u.t.WriteLCD(1, "     -> Opening <-")
			u.backends.physicalActions.OpenDoor(u.doorbellTarget)
		} else {
			u.t.WriteLCD(1, "     (unknown RFID)")
		}
		u.stateTimeout = time.Now().Add(postDoorbellRFIDDuration)
	}
}

// We switch back to idle after some time, handled in this tick. Also, if we
// pick up request from other sub-systems and we are done with whatever we are
// doing
func (u *UIControlHandler) HandleTick() {
	if u.state != StateIdle && time.Now().After(u.stateTimeout) {
		u.backToIdle()
	}

	// Let's see if we need to switch to some externally
	// triggered state. We do that in HandleTick() so that
	// it is executed in the right thread.
	u.outOfThreadRequest.Lock()
	defer u.outOfThreadRequest.Unlock()

	if u.state == StateIdle && u.doorbellRequest != nil {
		u.startDoorbellRequest(u.doorbellRequest)
		u.doorbellRequest = nil
	}

	if u.state == StateIdle {
		u.displayIdleScreen()
	}
}

// Doorbell UI interface.
func (u *UIControlHandler) HandleDoorbell(which Target, message string) {
	if time.Now().After(u.nextAllowdDoorbell) {
		u.backends.physicalActions.RingBell(which)
		u.nextAllowdDoorbell = time.Now().Add(defaultDoorbellRatelimit)
		u.doorWhileSnoozedCount = 0
	} else {
		u.doorWhileSnoozedCount++ // Count snoozed counts
	}

	// Now trigger the UI request
	// TODO: this is icky. We are possibly running in different thread
	// context thus need do do some locking dance.
	// Maybe we should tie together cross-handler requests with channels
	// of some sort ?
	u.outOfThreadRequest.Lock()
	u.doorbellRequest = &UIDoorbellRequest{target: which, message: message}
	u.outOfThreadRequest.Unlock()
}

func (u *UIControlHandler) displayIdleScreen() {
	// TODO: do something fancy every now and then, some animation..
	now := time.Now()
	u.t.WriteLCD(0, "      Noisebridge")
	u.t.WriteLCD(1, now.Format("2006-01-02 [Mon] 15:04"))
}

func (u *UIControlHandler) presentMemberActions(member *User) {
	u.t.WriteLCD(0, fmt.Sprintf("Howdy %s", member.Name))
	u.t.WriteLCD(1, "[*]ESC [1]Add [2]Update")

	u.setState(StateWaitMemberCommand, 5*time.Second)
}

func (u *UIControlHandler) displayUserInfo(user *User) {
	// First line
	if user.HasContactInfo() {
		u.t.WriteLCD(0, "Hi "+user.Name)
	} else {
		// No contact info; this is a temporary ID that
		// expires after some time.
		exp := user.ExpiryDate(time.Now())
		days_left := exp.Sub(time.Now()) / (24 * time.Hour)
		if days_left <= 0 {
			// Already expired; show when that happend.
			u.t.WriteLCD(0, fmt.Sprintf("Exp %s",
				exp.Format("2006-01-02 15:04")))
		} else if days_left < 10 {
			// When it gets more urgent to renew, show when
			u.t.WriteLCD(0, fmt.Sprintf("%s (exp %dd)",
				user.Name, days_left))
		} else {
			// Just show (arbitrary generated) name.
			u.t.WriteLCD(0, user.Name)
		}
	}

	// Second line
	if user.InValidityPeriod(time.Now()) {
		from, to := user.AccessHours()
		u.t.WriteLCD(1, fmt.Sprintf("Open doors [%d:00-%d:00)",
			from, to))
	} else {
		u.t.WriteLCD(1, "Ask member to renew.")
	}

	u.setState(StateDisplayInfoMessage, 2*time.Second)
}

func (u *UIControlHandler) startDoorbellRequest(req *UIDoorbellRequest) {
	u.setState(StateDoorbellRequest, showDoorbellDuration)
	u.doorbellTarget = req.target
	to_display := ""
	if len(req.message) == 0 {
		to_display = fmt.Sprintf("%s %s %s",
			DoorBellCharacter, req.target, DoorBellCharacter)
	} else {
		to_display = fmt.Sprintf("%s %s %s %s",
			DoorBellCharacter, req.message, req.target,
			DoorBellCharacter)
	}

	// If someone is ringing like crazy, display that
	if u.doorWhileSnoozedCount > 0 {
		to_display += fmt.Sprintf("(%d)", u.doorWhileSnoozedCount)
	}

	fmt_len := int((24-len(to_display))/2) + len(to_display)
	if fmt_len > 24 {
		fmt_len = 24
	}
	u.t.WriteLCD(0, fmt.Sprintf("%*s", fmt_len, to_display))

	now := time.Now()
	// The snooze option always works, but we only show it when there is
	// some repeated annoyance going on to keep UI simple in the simple case
	if now.Sub(u.lastDoorbellRequest) < offerSnoozeWhenRepeatedRingsUnder {
		u.t.WriteLCD(1, "RFID => open | [9]Snooze")
	} else {
		u.t.WriteLCD(1, "RFID => open | [*] ESC")
	}

	u.lastDoorbellRequest = now
}
