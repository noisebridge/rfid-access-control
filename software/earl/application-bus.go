// Central hub to exchange application level Events.
//
// Many situations cannot be handled locally by terminal event handlers, but
// have to be delegated somewhere else (e.g. Doorbell button pressed
// somewhere should all be handled by the LCD control terminal and also by the
// physical doorbell tone-generator). Or the reverse: opening a door due to
// some separate event should be reflected in the terminals' green light as well.
//
// The http-api provides these events via JSON for interested listeners.
// Also it is possible to trigger these events artificially, e.g. through
// some web-service.
//
// Since each of these handlers are possibly running in different threads, we
// separate thread-boundaries with channels and have each handler deal with
// things in their own pace.
//
// The appliation bus allows interested parties to Subscribe() to events that
// are being Post()ed to the bus.
package main

import (
	"time"
)

type AppEventType string

const (
	// Entrance handling events.
	AppDoorbellTriggerEvent = AppEventType("trigger-bell") // Doorbell triggered for target
	AppDoorSensorEvent      = AppEventType("door-sensor")  // Target door opened/closed
	AppOpenRequest          = AppEventType("open")         // Request to open door for target.
	AppHushBellRequest      = AppEventType("hush-bell")    // Request to snooze bell until given timeout

	// User management events.
	AppUserAdded   = AppEventType("user-added")
	AppUserUpdated = AppEventType("user-updated")
	AppUserDeleted = AppEventType("user-deleted")

	// terminal/lifetime handling
	AppEarlStarted        = AppEventType("earl-started")
	AppTerminalConnect    = AppEventType("terminal-connect")
	AppTerminalDisconnect = AppEventType("terminal-disconnect")

	applicationBusInternalFlush = AppEventType("internal-flush")
)

// We keep it simple and somewhat un-typed: an event is identified by an
// enumeration, and optional parameters are passed alongside.
type AppEvent struct {
	// Required parameters
	Timestamp time.Time // Automatically set on Post if not set.
	Ev        AppEventType

	Target Target // The target for which this event is meant.
	Source string // Mostly FYI: what subsystem generated it
	Msg    string // FYI, good to display to a human user

	// Optional paramters, depending on context.
	Value   int
	Timeout time.Time
}

type AppEventChannel chan *AppEvent
type ApplicationBus struct {
	receivers        map[AppEventChannel]bool
	syncedOperations chan func()
	isRunning        bool
}

func NewApplicationBus() *ApplicationBus {
	bus := &ApplicationBus{
		receivers:        make(map[AppEventChannel]bool),
		syncedOperations: make(chan func(), 1),
		isRunning:        true,
	}
	go bus.run()
	return bus
}

func (b *ApplicationBus) Post(event *AppEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	b.syncedOperations <- func() {
		for channel, _ := range b.receivers {
			if event.Ev != applicationBusInternalFlush {
				channel <- event
			}
		}
	}
}

func (b *ApplicationBus) Flush() {
	// Since we have a syncedOperations channel of length 1, this will
	// make sure that we wait until the previous operation is through.
	b.Post(&AppEvent{Ev: applicationBusInternalFlush})
}

func (b *ApplicationBus) Subscribe(channel AppEventChannel) {
	b.syncedOperations <- func() { b.receivers[channel] = true }
}

func (b *ApplicationBus) Unsubscribe(channel AppEventChannel) {
	b.syncedOperations <- func() { delete(b.receivers, channel) }
}

func (b *ApplicationBus) Shutdown() {
	b.syncedOperations <- func() { b.isRunning = false }
}

func (b *ApplicationBus) run() {
	for b.isRunning {
		op := <-b.syncedOperations
		op()
	}
}
