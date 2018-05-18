package events

const (
	EventChartLoad   = "chart-load"
	EventPreRender   = "pre-render"
	EventPostRender  = "post-render"
	EventPreInstall  = "pre-install"
	EventPostInstall = "post-install"
)

// EventHandler is a function capabile of responding to an event.
type EventHandler func(*Context) error

// registry is the storage for events and handlers.
type registry map[string][]EventHandler

// Emitter provides a way to register, manage, and execute events.
type Emitter struct {
	reg registry
}

// New creates a new event Emitter with no registered events.
func New() *Emitter {
	return &Emitter{
		reg: registry{},
	}
}

// Emit executes the given event with the given context.
//
// The first time an error occurs, this will cease the event cycle and return the
// error.
func (e *Emitter) Emit(event string, ctx *Context) error {
	fns := e.reg[event]
	for _, fn := range fns {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// On binds an EventHandler to an event.
// If an event already has EventHandlers, this will be appended to the end of the
// list.
func (e *Emitter) On(event string, fn EventHandler) {
	handlers, ok := e.reg[event]
	if !ok {
		e.reg[event] = []EventHandler{fn}
		return
	}
	e.reg[event] = append(handlers, fn)
}

// Handlers returns all of the EventHandlers for the given event.
//
// If no handlers are registered for this event, then an empty array is returned.
// This is not an error condition, because it is perfectly normal for an event to
// have no registered handlers.
func (e *Emitter) Handlers(event string) []EventHandler {
	handlers, ok := e.reg[event]
	if !ok {
		return []EventHandler{}
	}
	return handlers
}

// SetHandlers sets all of the handlers for a particular event.
//
// This will overwrite the existing event handlers for this event. It is allowed
// to set this to an empty list.
func (e *Emitter) SetHandlers(event string, listeners []EventHandler) {
	e.reg[event] = listeners
}

// Len returns the number of event handlers registered for the given event.
func (e *Emitter) Len(event string) int {
	return len(e.reg[event])
}

// Events returns the names of all of the events that have been registered.
// Note that this does not ensure that these events have registered handlers.
// See SetHandlers above.
func (e *Emitter) Events() []string {
	h := make([]string, 0, len(e.reg))
	for name := range e.reg {
		h = append(h, name)
	}
	return h
}
