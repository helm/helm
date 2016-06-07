package lint

import "fmt"

// Severity indicatest the severity of a Message.
type Severity int

const (
	// UnknownSev indicates that the severity of the error is unknown, and should not stop processing.
	UnknownSev = iota
	// InfoSev indicates information, for example missing values.yaml file
	InfoSev
	// WarningSev indicates that something does not meet code standards, but will likely function.
	WarningSev
	// ErrorSev indicates that something will not likely function.
	ErrorSev
)

// sev matches the *Sev states.
var sev = []string{"UNKNOWN", "INFO", "WARNING", "ERROR"}

// Message is a linting output message
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
