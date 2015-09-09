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

	// For annoying people...
	offerSilenceWhenRepeatedRingsUnder = 2 * time.Second
	silenceDoorbellIncrement           = 60 * time.Second
	maxSilenceDoorbell                 = 5 * 60 * time.Second
)

const (
	// We programmed the LCD to show a doorbell pictogram
	DoorBellCharacter = "\001"
)

type UIControlHandler struct {
	backends *Backends
	auth     Authenticator // shortcut, copy of the pointer in backends

	t Terminal

	authUserCode string // current active member code

	state        UIState   // state of our state machine
	stateTimeout time.Time // timeout of current state

	userCounter int // counter to generate new user names.

	// We allow rate-limiting of the doorbell.
	lastDoorbellRequest time.Time // To know when to offer hush.
	doorbellTarget      Target
	endDoorbellHush     time.Time // Incremented

	// Stuff collected from events we see, mostly to
	// display on our idle screen.
	hushedDoorbellTimeout  time.Time      // Received from event.
	observedDoorOpenStatus map[Target]int // watching events fly by.
	actionMessage          string
	actionMessageTimeout   time.Time
}

func NewControlHandler(backends *Backends) *UIControlHandler {
	return &UIControlHandler{
		backends:               backends,
		auth:                   backends.authenticator,
		userCounter:            time.Now().Second() % 100, // semi-random start
		observedDoorOpenStatus: make(map[Target]int),
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

func (u *UIControlHandler) HandleShutdown() {}

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
			// Each press increments by one minute, up to a maximum time.
			if u.endDoorbellHush.Before(time.Now()) {
				u.endDoorbellHush = time.Now()
			}
			u.endDoorbellHush = u.endDoorbellHush.Add(silenceDoorbellIncrement)
			if u.endDoorbellHush.After(time.Now().Add(maxSilenceDoorbell)) {
				u.endDoorbellHush = time.Now().Add(maxSilenceDoorbell)
			}
			silenceMsg := fmt.Sprintf("Bell silenced %dsec",
				u.endDoorbellHush.Sub(time.Now())/time.Second)
			u.postDoorbellHush("Hush pressed on control-terminal; " + silenceMsg)
			u.t.WriteLCD(0, silenceMsg)
			// Fall back soon.
			u.setState(StateDoorbellRequest, 3*time.Second)
		}
		if key == '5' {
			// inform people that think [5] works (it worked once)
			u.t.WriteLCD(1, "Open only with RFID.")
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

			default:
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
		// we assume they are allowed to open the door.
		// TODO: revisit and allow memmbers once their density increases ?
		if u.auth.FindUser(rfid) != nil {
			u.openDoorAndShow(u.doorbellTarget, "via RFID on control")
			u.backToIdle()
		}
	}
}

// We switch back to idle after some time, handled in this tick. Also, if we
// pick up request from other sub-systems and we are done with whatever we are
// doing
func (u *UIControlHandler) HandleTick() {
	if u.state != StateIdle && time.Now().After(u.stateTimeout) {
		u.backToIdle()
	}

	if u.state == StateIdle {
		u.displayIdleScreen()
	}
}

// We hook into a number of app events as we want to display the status,
// or even offer to deal with it, such as the doorbell.
func (u *UIControlHandler) HandleAppEvent(event *AppEvent) {
	switch event.Ev {
	case AppDoorbellTriggerEvent:
		// We interrupt whatever we are doing now, as this is
		// more important:
		u.startDoorOpenUI(event.Target, event.Msg)
	case AppOpenRequest:
		u.actionMessage = "Opening " + string(event.Target)
		u.actionMessageTimeout = time.Now().Add(2 * time.Second)
	case AppHushBellRequest:
		u.hushedDoorbellTimeout = event.Timeout
	case AppDoorSensorEvent:
		u.observedDoorOpenStatus[event.Target] = event.Value
		if event.Value == 1 {
			u.actionMessage = "" // No need to show 'Open' anymore
		}
	}
	u.HandleTick() // If we're in idle, update status right away.
}

// Create a string from the observed door status. Only mention open
// doors, as these are important. "gate, upstairs : open"
func (u *UIControlHandler) getDoorStatusString() string {
	result := ""
	for key, value := range u.observedDoorOpenStatus {
		if value > 0 {
			if len(result) > 0 {
				result += ", "
			}
			result += string(key)
		}
	}
	if len(result) > 0 {
		result += " : open"
	}
	return result
}

func (u *UIControlHandler) displayIdleScreen() {
	now := time.Now()

	// -- Status message line
	// Let's see if there is anything interesting to display in
	// the status screen, otherwise fall back to 'Noisebridge'
	if u.hushedDoorbellTimeout.After(now) {
		u.t.WriteLCD(0, fmt.Sprintf("Bell silenced %dsec",
			u.hushedDoorbellTimeout.Sub(now)/time.Second))
	} else if doorStatus := u.getDoorStatusString(); doorStatus != "" {
		u.t.WriteLCD(0, doorStatus)
	} else {
		// Default, nothing else to display
		u.t.WriteLCD(0, "      Noisebridge")
	}

	// -- Action message line
	if u.actionMessage != "" && now.Before(u.actionMessageTimeout) {
		u.t.WriteLCD(1, u.actionMessage)
	} else {
		u.t.WriteLCD(1, now.Format("2006-01-02 [Mon] 15:04"))
	}
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

func (u *UIControlHandler) startDoorOpenUI(target Target, message string) {
	now := time.Now()

	u.setState(StateDoorbellRequest, showDoorbellDuration)
	u.doorbellTarget = target
	to_display := ""
	if len(message) == 0 {
		to_display = fmt.Sprintf("%s %s %s",
			DoorBellCharacter, target, DoorBellCharacter)
	} else {
		to_display = fmt.Sprintf("%s %s %s %s",
			DoorBellCharacter, target, message, DoorBellCharacter)
	}

	fmt_len := int((24-len(to_display))/2) + len(to_display)
	if fmt_len > 24 {
		fmt_len = 24
	}
	u.t.WriteLCD(0, fmt.Sprintf("%*s", fmt_len, to_display))

	// The hush option always works, but we only show it when there is
	// some repeated annoyance going on to keep UI simple in the simple case
	if now.Sub(u.lastDoorbellRequest) < offerSilenceWhenRepeatedRingsUnder {
		u.t.WriteLCD(1, "RFID:Open | [9] Silence!")
	} else {
		u.t.WriteLCD(1, "RFID:Open | [*] ESC")
	}

	u.lastDoorbellRequest = now
}

func (u *UIControlHandler) postDoorbellHush(msg string) {
	u.backends.appEventBus.Post(&AppEvent{
		Ev:      AppHushBellRequest,
		Target:  u.doorbellTarget,
		Source:  u.t.GetTerminalName(),
		Msg:     msg,
		Timeout: u.endDoorbellHush,
	})
}

func (u *UIControlHandler) resetDoorHush() {
	if u.endDoorbellHush.After(time.Now()) {
		u.endDoorbellHush = time.Now().Add(-time.Second) // Expire immediately.
		u.postDoorbellHush("(unhush)")
	}
}

func (u *UIControlHandler) openDoorAndShow(where Target, msg string) {
	u.backends.appEventBus.Post(&AppEvent{
		Ev:     AppOpenRequest,
		Target: where,
		Source: u.t.GetTerminalName(),
		Msg:    msg,
	})
	u.resetDoorHush()

	// Note: We will receive this request for opening ourself and will
	// update the LCD. Why not here directly ? Because we want to also
	// show door opening actions triggered externally.
	u.backToIdle()
}
