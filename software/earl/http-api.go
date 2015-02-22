// API to see events fly by.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

type ApiServer struct {
	bus    *ApplicationBus
	server *http.Server

	// Remember the last event for each type. Already JSON prepared
	eventChannel   AppEventChannel
	lastEvents     map[AppEventType]*JsonAppEvent
	lastEventsLock sync.Mutex
}

// Similar to AppEvent, but json serialization hints and timestamp being
// a pointer to be able to omit it.
type JsonAppEvent struct {
	// An event is historic, if it had been recorded prior to the API
	// conneect
	IsHistoricEvent bool `json:",omitempty"`

	Timestamp time.Time    `json:"timestamp"`
	Ev        AppEventType `json:"type"`
	Target    Target       `json:"target"`
	Source    string       `json:"source"`
	Msg       string       `json:"msg"`
	Value     int          `json:"value,omitempty"`
	Timeout   *time.Time   `json:"timeout,omitempty"`
}

func JsonEventFromAppEvent(event *AppEvent) *JsonAppEvent {
	jev := &JsonAppEvent{
		Timestamp: event.Timestamp,
		Ev:        event.Ev,
		Target:    event.Target,
		Source:    event.Source,
		Msg:       event.Msg,
		Value:     event.Value,
	}
	if !event.Timeout.IsZero() {
		jev.Timeout = &event.Timeout
	}
	return jev
}

func NewApiServer(bus *ApplicationBus, port int) *ApiServer {
	newObject := &ApiServer{
		bus: bus,
		server: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
			// JSON events listeners should be kept open for a while
			WriteTimeout: 3600 * time.Second,
		},
		eventChannel: make(AppEventChannel),
		lastEvents:   make(map[AppEventType]*JsonAppEvent),
	}
	newObject.server.Handler = newObject
	bus.Subscribe(newObject.eventChannel)
	go newObject.collectLastEvents()
	return newObject
}

func (a *ApiServer) Run() {
	a.server.ListenAndServe()
}

func (a *ApiServer) collectLastEvents() {
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

type EventList []*JsonAppEvent

func (el EventList) Len() int { return len(el) }
func (el EventList) Less(i, j int) bool {
	return el[i].Timestamp.Before(el[j].Timestamp)
}
func (el EventList) Swap(i, j int) { el[i], el[j] = el[j], el[i] }

func (a *ApiServer) getHistory() []*JsonAppEvent {
	result := EventList{}
	a.lastEventsLock.Lock()
	for _, ev := range a.lastEvents {
		result = append(result, ev)
	}
	a.lastEventsLock.Unlock()
	sort.Sort(result) // Show old events first
	return result
}

func flushResponse(out http.ResponseWriter) {
	if f, ok := out.(http.Flusher); ok {
		f.Flush()
	}
}

func (event *JsonAppEvent) writeJSONEvent(out http.ResponseWriter, jsonp_callback string) bool {
	json, err := json.Marshal(event)
	if err != nil {
		// Funny event, let's just ignore.
		return true
	}
	if jsonp_callback != "" {
		out.Write([]byte(jsonp_callback + "("))
	}
	_, err = out.Write(json)
	if err != nil {
		return false
	}
	if jsonp_callback != "" {
		out.Write([]byte(");"))
	}
	out.Write([]byte("\n"))
	flushResponse(out)
	return true
}

func (a *ApiServer) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "POST" {
		out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.URL.Path != "/api/events" {
		out.WriteHeader(http.StatusNotFound)
		out.Write([]byte("Nothing to see here. " +
			"The cool stuff is happening at /api/events"))
		return
	}

	req.ParseForm()
	cb := req.Form.Get("callback")
	if cb == "" {
		out.Header()["Content-Type"] = []string{"application/json"}
	} else {
		out.Header()["Content-Type"] = []string{"application/javascript"}
	}

	// Make browsers happy.
	allowOrigin := req.Header.Get("Origin")
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	out.Header()["Access-Control-Allow-Origin"] = []string{allowOrigin}

	for _, event := range a.getHistory() {
		if !event.writeJSONEvent(out, cb) {
			break
		}
	}
	flushResponse(out)

	// TODO: for JSONP, do we essentially have to close the connection after
	// we emit an event, otherwise the browser never knows when things
	// finish ?
	appEvents := make(AppEventChannel, 3)
	a.bus.Subscribe(appEvents)
	for {
		event := <-appEvents
		if !JsonEventFromAppEvent(event).writeJSONEvent(out, cb) {
			break
		}
	}
	a.bus.Unsubscribe(appEvents)
}
