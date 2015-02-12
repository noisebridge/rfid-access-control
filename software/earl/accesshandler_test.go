package main

import (
	"strings"
	"testing"
	"time"
)

type ACKey struct {
	code   string
	target Target
}

// Implements Athenticator interface.
type MockAuthenticator struct {
	allow map[ACKey]AuthResult
}

func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		allow: make(map[ACKey]AuthResult)}
}

func (a *MockAuthenticator) AuthUser(code string, target Target) (AuthResult, string) {
	result, ok := a.allow[ACKey{code, target}]
	if !ok {
		return AuthFail, "User does not exist"
	}
	if result == AuthOk {
		return AuthOk, ""
	}
	return result, "MockAuthenticator says: some failure occured"
}

func (a *MockAuthenticator) AddNewUser(authentication_user string, user User) (bool, string) {
	return false, ""
}
func (a *MockAuthenticator) FindUser(code string) *User {
	// Return dummy user as accesshandler likes to independently find it.
	return &User{
		UserLevel: "member",
	}
}
func (a *MockAuthenticator) UpdateUser(auth_code string, user_code string, updater_fun ModifyFun) (bool, string) {
	return false, ""
}

func (a *MockAuthenticator) DeleteUser(auth_code string, user_code string) (bool, string) {
	return false, ""
}

type Buzz struct {
	toneCode string
	duration time.Duration
}

// Implements Terminal interface.
type MockTerminal struct {
	t      *testing.T
	colors string
	buzzes []Buzz
	lcd    [2]string
}

func NewMockTerminal(t *testing.T) *MockTerminal {
	ret := &MockTerminal{t: t}
	return ret
}

func (term *MockTerminal) GetTerminalName() string {
	return "mock"
}

func (term *MockTerminal) ShowColor(colors string) {
	term.colors = term.colors + colors
}

func (term *MockTerminal) BuzzSpeaker(toneCode string, duration time.Duration) {
	term.buzzes = append(term.buzzes, Buzz{toneCode, duration})
}

func (term *MockTerminal) WriteLCD(row int, text string) {
	term.lcd[row] = text
}

func (term *MockTerminal) expectColor(color string) {
	if !strings.Contains(term.colors, color) {
		term.t.Errorf("Expecting color '%v', but seeing colors '%v'", color, term.colors)
	}
}

func (term *MockTerminal) expectBuzz(buzz Buzz) {
	if len(term.buzzes) == 0 {
		term.t.Errorf("Expecting buzz %v but heard nothing", buzz)
		return
	}
	curBuzz := term.buzzes[0]
	term.buzzes = term.buzzes[1:]
	if curBuzz != buzz {
		term.t.Errorf("Expecting buzz %v but heard buzz %v", buzz, curBuzz)
	}
}

type TestFixture struct {
	tester             *testing.T
	mockauth           *MockAuthenticator
	mockterm           *MockTerminal
	expectEventChannel AppEventChannel
	termEventChannel   AppEventChannel
	mockbackends       *Backends

	handlerUnderTest *AccessHandler
}

func NewTestFixture(t *testing.T) *TestFixture {
	appBus := NewApplicationBus()
	// events sent to the terminal.
	termEventChannel := make(AppEventChannel, 10)
	appBus.Subscribe(termEventChannel)

	// events also recorded to match against expectations.
	expectEventChannel := make(AppEventChannel, 10)
	appBus.Subscribe(expectEventChannel)

	auth := NewMockAuthenticator()
	term := NewMockTerminal(t)
	backends := &Backends{
		authenticator: auth,
		appEventBus:   appBus,
	}

	testHandler := NewAccessHandler(backends)
	testHandler.Init(term)

	return &TestFixture{
		tester:             t,
		mockauth:           auth,
		mockterm:           term,
		termEventChannel:   termEventChannel,
		expectEventChannel: expectEventChannel,
		mockbackends:       backends,
		handlerUnderTest:   testHandler,
	}
}

func (f *TestFixture) FlushAllAppEvents() {
	f.mockbackends.appEventBus.Flush()
	for {
		select {
		// Events accumulated for the term: give it to handle
		case event := <-f.termEventChannel:
			f.handlerUnderTest.HandleAppEvent(event)
		case <-time.After(0):
			return // done.
		}
	}
}

func (f *TestFixture) ExpectEvent(ev AppEventType, target Target) {
	f.FlushAllAppEvents()
	select {
	case event := <-f.expectEventChannel:
		if event.Ev != ev || event.Target != target {
			f.tester.Errorf("Expecting event %d:%s, but got %d:%s\n",
				ev, target, event.Ev, event.Target)
		}
	case <-time.After(0):
		f.tester.Errorf("Expecting event %d:%s, but nothing in queue\n",
			ev, target)
	}
}

func (f *TestFixture) ExpectNoMoreEvents() {
	f.FlushAllAppEvents()
	select {
	case event := <-f.expectEventChannel:
		f.tester.Errorf("Didn't expect event but got %d:%s\n",
			event.Ev, event.Target)
	case <-time.After(0):
		// Good.
	}
}

func PressKeys(h *AccessHandler, keys string) {
	for _, key := range keys {
		h.HandleKeypress(byte(key))
	}
}

func TestValidAccessCode(t *testing.T) {
	testFixture := NewTestFixture(t)
	testFixture.mockauth.allow[ACKey{"123456", Target("mock")}] = AuthOk
	PressKeys(testFixture.handlerUnderTest, "123456#")
	testFixture.FlushAllAppEvents()

	testFixture.mockterm.expectColor("G")
	testFixture.mockterm.expectBuzz(Buzz{"H", 500})
	testFixture.ExpectEvent(AppOpenRequest, Target("mock"))
	testFixture.ExpectNoMoreEvents()
}

func TestInvalidAccessCode(t *testing.T) {
	testFixture := NewTestFixture(t)
	testFixture.mockauth.allow[ACKey{"123456", Target("mock")}] = AuthOk
	PressKeys(testFixture.handlerUnderTest, "654321#")
	testFixture.FlushAllAppEvents()

	testFixture.mockterm.expectColor("R")
	testFixture.mockterm.expectBuzz(Buzz{"L", 200})
	testFixture.ExpectNoMoreEvents()
}

func TestExpiredAccessCode(t *testing.T) {
	testFixture := NewTestFixture(t)
	testFixture.mockauth.allow[ACKey{"123456", Target("mock")}] = AuthExpired
	PressKeys(testFixture.handlerUnderTest, "123456#")
	testFixture.FlushAllAppEvents()

	testFixture.mockterm.expectColor("B") // 'nighttime'
	testFixture.mockterm.expectBuzz(Buzz{"L", 200})
	testFixture.ExpectEvent(AppDoorbellTriggerEvent, Target("mock"))
	testFixture.ExpectNoMoreEvents()
}

func TestKeypadDoorbell(t *testing.T) {
	testFixture := NewTestFixture(t)
	// Just a single '#' should ring the bell.
	PressKeys(testFixture.handlerUnderTest, "#")
	testFixture.FlushAllAppEvents()

	testFixture.ExpectEvent(AppDoorbellTriggerEvent, Target("mock"))
	testFixture.ExpectNoMoreEvents()
}

func TestKeypadTimeout(t *testing.T) {
	testFixture := NewTestFixture(t)
	testFixture.mockauth.allow[ACKey{"123456", Target("mock")}] = AuthOk
	mockClock := &MockClock{}
	testFixture.handlerUnderTest.clock = mockClock

	PressKeys(testFixture.handlerUnderTest, "123456")   // missing #
	mockClock.now = mockClock.now.Add(60 * time.Second) // >> keypad timeout
	testFixture.handlerUnderTest.HandleTick()
	testFixture.FlushAllAppEvents()

	testFixture.mockterm.expectBuzz(Buzz{"L", 500}) // timeout buzz
	testFixture.ExpectNoMoreEvents()
}

func TestRFIDDebounce(t *testing.T) {
	testFixture := NewTestFixture(t)
	testFixture.mockauth.allow[ACKey{"rfid-123", Target("mock")}] = AuthOk
	mockClock := &MockClock{}
	testFixture.handlerUnderTest.clock = mockClock

	testFixture.handlerUnderTest.HandleRFID("rfid-123")
	testFixture.FlushAllAppEvents()
	testFixture.ExpectEvent(AppOpenRequest, Target("mock"))

	// A quickly coming same RFID should not open the door again
	testFixture.handlerUnderTest.HandleRFID("rfid-123")
	testFixture.FlushAllAppEvents()
	testFixture.ExpectNoMoreEvents()

	// .. but after some de-bounce time, this should work again.
	mockClock.now = mockClock.now.Add(10 * time.Second)
	testFixture.handlerUnderTest.HandleRFID("rfid-123")
	testFixture.ExpectEvent(AppOpenRequest, Target("mock"))
}

// test ideas:
//  - too short code: don't buzz
