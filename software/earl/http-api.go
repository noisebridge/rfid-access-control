// API to see events fly by.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ApiServer struct {
	bus    *ApplicationBus
	server *http.Server
}

// Similar to AppEvent, but json serialization hints and timestamp being
// a pointer to be able to omit it.
type JsonAppEvent struct {
	Ev      AppEventType `json:"ev"`
	Target  Target       `json:"target"`
	Source  string       `json:"source"`
	Msg     string       `json:"msg"`
	Value   int          `json:"value,omitempty"`
	Timeout *time.Time   `json:"timeout,omitempty"`
}

func JsonEventFromAppEvent(event *AppEvent) JsonAppEvent {
	jev := JsonAppEvent{
		Ev:     event.Ev,
		Target: event.Target,
		Source: event.Source,
		Msg:    event.Msg,
		Value:  event.Value,
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
	}
	newObject.server.Handler = newObject
	return newObject
}

func (a *ApiServer) Run() {
	a.server.ListenAndServe()
}

func flushResponse(out http.ResponseWriter) {
	if f, ok := out.(http.Flusher); ok {
		f.Flush()
	}
}

func (a *ApiServer) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "POST" {
		out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.URL.Path != "/api/events" {
		out.WriteHeader(http.StatusNotFound)
		return
	}

	req.ParseForm()
	cb := req.Form.Get("jsonp")
	if cb == "" {
		out.Header()["Content-Type"] = []string{"application/json"}
	} else {
		out.Header()["Content-Type"] = []string{"application/javascript"}
	}

	out.Write(
		[]byte("// Welcome to the Earl event API\n" +
			"// Plain /api/events for JSON, add ?jsonp=MyCallbackName for JSONP.\n"))
	flushResponse(out)

	appEvents := make(AppEventChannel, 3)
	a.bus.Subscribe(appEvents)
	defer a.bus.Unsubscribe(appEvents)

	for {
		event := <-appEvents
		json, err := json.Marshal(JsonEventFromAppEvent(event))
		if err != nil {
			continue
		}
		if cb != "" {
			out.Write([]byte(cb + "("))
		}
		_, err = out.Write(json)
		if err != nil {
			return
		}
		if cb != "" {
			out.Write([]byte(");"))
		}
		out.Write([]byte("\n"))
		flushResponse(out)
	}
}
