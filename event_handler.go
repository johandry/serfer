package serfer

import (
	"fmt"
	"sync"
)

// EventHandler responds to a Serfer event
type EventHandler interface {
	HandleEvent(e Event)
}

// EventHandlerFunc is an adapter to allow the use of ordinary functions as event
// handlers. If f is a function, EventHandlerFunc(f) is an EventHandler that calls f.
type EventHandlerFunc func(Event)

// HandleEvent calls f(e)
func (f EventHandlerFunc) HandleEvent(e Event) {
	f(e)
}

// NotFound is the default function assigned to an event
func NotFound(e Event) {
	// log.Printf("[WARN] serfer: Not found event handler for event %s", e.String())
}

// NotFoundEventHandler returns a default handler
func NotFoundEventHandler() EventHandler {
	return EventHandlerFunc(NotFound)
}

// Handler is a event handler multiplexer.
// It match the event name with a list of registered patterns and calls the
// handler that matches with the name exactly.
// Partial matching of patters with names is not considered in Serfer but it
// could be implemented in the code that uses Serfer.
type Handler struct {
	mu    sync.RWMutex
	entry map[string]map[string]handlerEntry
}

type handlerEntry struct {
	h         []EventHandler
	name      string
	eventType string
}

func notFoundHandlerEntry() handlerEntry {
	var hNotFound EventHandlerFunc = NotFound
	ehs := make([]EventHandler, 1)
	ehs[0] = hNotFound
	return handlerEntry{
		h:         ehs,
		name:      "",
		eventType: "",
	}
}

func newEntries() map[string]map[string]handlerEntry {
	return map[string]map[string]handlerEntry{
		"": {
			"": notFoundHandlerEntry(),
		},
	}
}

// NewHandler creates and returns a new Handler
func NewHandler() *Handler {
	return &Handler{
		entry: newEntries(),
	}
}

// DefaultHandler is the default Handler used by Serfer
var DefaultHandler = &defaultHandler
var defaultHandler Handler

func (h *Handler) match(eventType, name string) *handlerEntry {
	// log.Printf("[DEBUG] serfer: match() - Looking for a match for event %s(%s)", eventType, name)
	// (1) Try with the exact requested type and name
	if he, ok := h.entry[eventType][name]; ok {
		return &he
	}

	// (2) Try for the generic event for the requested type
	if he, ok := h.entry[eventType][""]; ok {
		return &he
	}

	// (3) Return the generic event for any type and name
	if he, ok := h.entry[""][""]; ok {
		return &he
	}

	// (4) This shouldn't happen: return the generic event for any type and name
	he := notFoundHandlerEntry()
	return &he
}

// EventHandler returns the event handler to use for the given event as well as
// it's type and name. It uses the event type and name to find it. It always
// returns a non-nil handler, if no event matches, then it returns the NotFound
// event handler.
func (h *Handler) EventHandler(e Event) (eventHandler []EventHandler, eventType string, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// log.Printf("[DEBUG] serfer: EventHandler() - Received event of type %s(%d)", e.EventType(), e.EventType())
	var he *handlerEntry

	if ue, ok := Event2UserEvent(e); ok {
		he = h.match("user", ue.Name)
	}

	if q, ok := Event2Query(e); ok {
		he = h.match("query", q.Name)
	}

	if _, t, ok := Event2MemberEvent(e); ok {
		he = h.match("member", t)
	}

	if he != nil {
		// log.Printf("[DEBUG] serfer: EventHandler() - Found a match for event, returning %s(%s)", he.eventType, he.name)
		return he.h, he.eventType, he.name
	}

	ehNotFound := make([]EventHandler, 1)
	ehNotFound[0] = NotFoundEventHandler()

	return ehNotFound, "", ""
}

// HandleEvent dispatches the events to the handler that matches with the event
// type and name ... or to the generic handler if it's not found.
func (h *Handler) HandleEvent(e Event) {
	ehs, _, _ := h.EventHandler(e)
	// log.Printf("[DEBUG] serfer: HandleEvent() - Found event handlers: %#v\n", ehs)
	for _, eh := range ehs {
		// log.Printf("[DEBUG] serfer: HandleEvent() - Handling event: %+v\n", eh)
		eh.HandleEvent(e)
	}
}

// General events Event Handlers:

// EventHandle register the event handler for the given event type and name
// In case of error it will panic
func (h *Handler) EventHandle(etype string, name string, eventHandler EventHandler) {
	if eventHandler == nil {
		panic("serfer: nil event handler")
	}
	if !validEventType(etype) {
		panic(fmt.Sprintf("serfer: unknown event type %q", etype))
	}
	// The event type '*' is the same as an empty event type '' and the latest
	// should be used
	if etype == "*" {
		etype = ""
	}
	// TODO: Do we really want this? or overwrite the event or add another event with same name?
	// Only the default entry can be overwritten
	// if _, ok := h.entry[etype][name]; ok && len(etype) != 0 && len(name) != 0 {
	// 	panic(fmt.Sprintf("serfer: multiple registrations for %s named %q", etype, name))
	// }
	if h.entry == nil {
		h.entry = newEntries()
	}

	var ehs []EventHandler
	if _, ok := h.entry[etype][name]; ok {
		ehs = append(h.entry[etype][name].h, eventHandler)
	} else {
		ehs = []EventHandler{eventHandler}
	}

	if _, ok := h.entry[etype]; !ok {
		h.entry[etype] = make(map[string]handlerEntry)
	}
	h.entry[etype][name] = handlerEntry{
		eventType: etype,
		name:      name,
		h:         ehs,
	}
}

// EventHandleFunc register the event handler function for the given event type
// and name
func (h *Handler) EventHandleFunc(etype string, name string, eventHandler func(Event)) {
	h.EventHandle(etype, name, EventHandlerFunc(eventHandler))
}

// EventHandle register the event handler for the given event type and name in
// the DefaultHandler
func EventHandle(etype string, name string, eventHandler EventHandler) {
	DefaultHandler.EventHandle(etype, name, eventHandler)
}

// EventHandleFunc register the event handler function for the given event type
// and name in the DefaultHandler
func EventHandleFunc(etype string, name string, eventHandler func(Event)) {
	DefaultHandler.EventHandleFunc(etype, name, eventHandler)
}

// User-events Event Handlers:

// UserEventHandle register the event handler for the given user event
func (h *Handler) UserEventHandle(name string, eventHandler EventHandler) {
	h.EventHandle("user", name, eventHandler)
}

// UserEventHandleFunc register the event handler function for the given user
// event
func (h *Handler) UserEventHandleFunc(name string, eventHandler func(Event)) {
	h.EventHandleFunc("user", name, eventHandler)
}

// UserEventHandle register the event handler for the given user event in
// the DefaultHandler
func UserEventHandle(name string, eventHandler EventHandler) {
	DefaultHandler.UserEventHandle(name, eventHandler)
}

// UserEventHandleFunc register the event handler function for the given user
// event in the DefaultHandler
func UserEventHandleFunc(name string, eventHandler func(Event)) {
	DefaultHandler.UserEventHandleFunc(name, eventHandler)
}

// Queries Event Handlers:

// QueryEventHandle register the event handler for the given query
func (h *Handler) QueryEventHandle(name string, eventHandler EventHandler) {
	h.EventHandle("query", name, eventHandler)
}

// QueryEventHandleFunc register the event handler function for the given query
func (h *Handler) QueryEventHandleFunc(name string, eventHandler func(Event)) {
	h.EventHandleFunc("query", name, eventHandler)
}

// QueryEventHandle register the event handler for the given query in
// the DefaultHandler
func QueryEventHandle(name string, eventHandler EventHandler) {
	DefaultHandler.QueryEventHandle(name, eventHandler)
}

// QueryEventHandleFunc register the event handler function for the given query
// in the DefaultHandler
func QueryEventHandleFunc(name string, eventHandler func(Event)) {
	DefaultHandler.QueryEventHandleFunc(name, eventHandler)
}

// Membership Event Handlers:

// MemberEventHandle register the event handler for the given member event
func (h *Handler) MemberEventHandle(name string, eventHandler EventHandler) {
	h.EventHandle("member", name, eventHandler)
}

// MemberEventHandleFunc register the event handler function for the given
// member event
func (h *Handler) MemberEventHandleFunc(name string, eventHandler func(Event)) {
	h.EventHandleFunc("member", name, eventHandler)
}

// MemberEventHandle register the event handler for the given member event in
// the DefaultHandler
func MemberEventHandle(name string, eventHandler EventHandler) {
	DefaultHandler.MemberEventHandle(name, eventHandler)
}

// MemberEventHandleFunc register the event handler function for the given
// member event in the DefaultHandler
func MemberEventHandleFunc(name string, eventHandler func(Event)) {
	DefaultHandler.MemberEventHandleFunc(name, eventHandler)
}
