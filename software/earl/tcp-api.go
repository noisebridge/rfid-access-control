package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
)

const (
	CONN_HOST = "0.0.0.0"
	CONN_TYPE = "tcp"
)

type TcpServer struct {
	bus *ApplicationBus

	// Remember the last event for each type. Already JSON prepared
	eventChannel   AppEventChannel
	lastEvents     map[AppEventType]*JsonAppEvent
	lastEventsLock sync.Mutex
	port           int
}

func NewTcpServer(bus *ApplicationBus, port int) *TcpServer {
	newObject := &TcpServer{
		bus:          bus,
		eventChannel: make(AppEventChannel),
		lastEvents:   make(map[AppEventType]*JsonAppEvent),
		port:         port,
	}
	bus.Subscribe(newObject.eventChannel)
	go newObject.collectLastEvents()
	return newObject
}

func (a *TcpServer) collectLastEvents() {
	for {
		ev := <-a.eventChannel
		// Remember the last event of each type.
		a.lastEventsLock.Lock()
		jsonified := JsonEventFromAppEvent(ev)
		jsonified.IsHistoricEvent = true
		a.lastEvents[ev.Ev] = jsonified
		a.lastEventsLock.Unlock()
	}
}

func (a *TcpServer) getHistory() []*JsonAppEvent {
	result := EventList{}
	a.lastEventsLock.Lock()
	for _, ev := range a.lastEvents {
		result = append(result, ev)
	}
	a.lastEventsLock.Unlock()
	sort.Sort(result) // Show old events first
	return result
}

func (a *TcpServer) ListenAndServe() {
	listener, err := net.Listen(CONN_TYPE, fmt.Sprintf("%s:%d", CONN_HOST, a.port))
	if err != nil {
		fmt.Println("TcpServer error listening: ", err.Error())
		return
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("TcpServer error accepting: ", err.Error())
			os.Exit(1)
		}
		go a.handleTcpConnection(conn)
	}
}

func (a *TcpServer) Run() {
	a.ListenAndServe()
}

func (event *JsonAppEvent) writeJSONEventToTCP(conn net.Conn) bool {
	json, err := json.Marshal(event)
	if err != nil {
		// Funny event, let's just ignore.
		return true
	}
	_, err = conn.Write(json)
	if err != nil {
		return false
	}
	conn.Write([]byte("\n"))
	return true
}

func (a *TcpServer) handleTcpConnection(conn net.Conn) {
	defer conn.Close()

	// Write out the historical events
	for _, event := range a.getHistory() {
		if !event.writeJSONEventToTCP(conn) {
			break
		}
	}

	// Subscribe and write out new events as published
	appEvents := make(AppEventChannel, 3)
	a.bus.Subscribe(appEvents)
	for {
		event := <-appEvents
		if !JsonEventFromAppEvent(event).writeJSONEventToTCP(conn) {
			break
		}
	}
	a.bus.Unsubscribe(appEvents)
}
