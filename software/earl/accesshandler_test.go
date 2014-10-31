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
	allow map[ACKey]bool
}

func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		allow: make(map[ACKey]bool)}
}

func (a *MockAuthenticator) AuthUser(code string, target Target) (ok bool) {
	_, ok = a.allow[ACKey{code, target}]
	return
}
func (a *MockAuthenticator) AddNewUser(authentication_user string, user User) bool { return false }
func (a *MockAuthenticator) FindUser(code string) *User                            { return nil }

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
	term.colors = colors
}

func (term *MockTerminal) BuzzSpeaker(toneCode string, duration time.Duration) {
	term.buzzes = append(term.buzzes, Buzz{toneCode, duration})
}

func (term *MockTerminal) WriteLCD(row int, text string) {
	term.lcd[row] = text
}

func (term *MockTerminal) expectColor(color string) {
	if strings.Index(term.colors, color) == -1 {
		term.t.Errorf("Expecting color %v, but seeing colors %v", color, term.colors)
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

type MockDoorActions struct {
	t      *testing.T
	opened map[Target]bool
}

func NewMockDoorActions(t *testing.T) *MockDoorActions {
	return &MockDoorActions{
		t:      t,
		opened: make(map[Target]bool)}
}

func (doorActions *MockDoorActions) OpenDoor(target Target) {
	doorActions.opened[target] = true
}

func (doorActions *MockDoorActions) expectOpened(target Target) {
	opened, has := doorActions.opened[target]
	if !(has && opened) {
		doorActions.t.Errorf("Expected %v to be open", target)
	}
}

func PressKeys(h *AccessHandler, keys string) {
	for _, key := range keys {
		h.HandleKeypress(byte(key))
	}
}

func TestAccessCode(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = true
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(auth, doorActions)
	handler.Init(term)
	handler.clock = MockClock{}
	PressKeys(handler, "123456#")
	//	term.expectColor("G")
	term.expectBuzz(Buzz{"H", 500})
	doorActions.expectOpened(Target("mock"))
}

// test ideas:
//  - wrong code: buzz low and red light
//  - too short code: don't buzz
