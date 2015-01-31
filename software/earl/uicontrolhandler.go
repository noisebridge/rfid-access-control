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
	"strings"
	"time"
)

type UIState int

const (
	StateIdle               = iota // When there is nothing to do; idle screen.
	StateDisplayInfoMessage        // Interrupt idle screen and show info message
	StateWaitMemberCommand         // Member showed RFID; awaiting instruction
	StateAddAwaitNewRFID           // Member adds new user: wait for new user RFID
	StateUpdateAwaitRFID           // Member updates user: wait for new user RFID
)

type UIControlHandler struct {
	backends *Backends
	auth     Authenticator // shortcut, copy of the pointer in backends

	t Terminal

	authUserCode string // current active member code

	state        UIState   // state of our state machine
	stateTimeout time.Time // timeout of current state

	userCounter int
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
}
func (u *UIControlHandler) ShutdownHandler() {}

func (u *UIControlHandler) HandleKeypress(key byte) {
	if key == '*' { // The '*' key is always 'Esc'-equivalent
		u.backToIdle()
		return
	}
	if u.state == StateWaitMemberCommand && key == '1' {
		u.t.WriteLCD(0, "Read new user RFID")
		u.t.WriteLCD(1, "[*] Cancel")
		u.setState(StateAddAwaitNewRFID, 30*time.Second)
		return
	}
	if u.state == StateWaitMemberCommand && key == '2' {
		u.t.WriteLCD(0, "Read user RFID to update")
		u.t.WriteLCD(1, "[*] Cancel")
		u.setState(StateUpdateAwaitRFID, 30*time.Second)
		return
	}
}

func (u *UIControlHandler) HandleRFID(rfid string) {
	// The ID comes as "<length> <code>". Get the code.
	rfid = strings.TrimSpace(strings.Split(rfid, " ")[1])

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
	}

}

func (u *UIControlHandler) HandleTick() {
	if u.state != StateIdle && time.Now().After(u.stateTimeout) {
		u.backToIdle()
	}
	if u.state == StateIdle {
		u.displayIdleScreen()
	}
}

// Doorbell UI interface.
func (u *UIControlHandler) HandleDoorbell(which Target, message string) {
	// TODO: implement.
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
