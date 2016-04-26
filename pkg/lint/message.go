package lint

import "fmt"

type Severity int

const (
	UnknownSev = iota
	WarningSev
	ErrorSev
)

var sev = []string{"INFO", "WARNING", "ERROR"}

type Message struct {
	// Severity is one of the *Sev constants
	Severity int
	// Text contains the message text
	Text string
}

// String prints a string representation of this Message.
//
// Implements fmt.Stringer.
func (m Message) String() string {
	return fmt.Sprintf("[%s] %s", sev[m.Severity], m.Text)
}
