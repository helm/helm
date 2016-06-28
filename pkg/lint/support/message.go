/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package support

import "fmt"

// Severity indicatest the severity of a Message.
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

// Linter encapsulates a linting run of a particular chart.
type Linter struct {
	Messages []Message
	// The highest severity of all the failing lint rules
	HighestSeverity int
	ChartDir        string
}

// LintError describes an error encountered while linting.
type LintError interface {
	error
}

// String prints a string representation of this Message.
//
// Implements fmt.Stringer.
func (m Message) String() string {
	return fmt.Sprintf("[%s] %s", sev[m.Severity], m.Text)
}

// RunLinterRule returns true if the validation passed
func (l *Linter) RunLinterRule(severity int, lintError LintError) bool {
	// severity is out of bound
	if severity < 0 || severity >= len(sev) {
		return false
	}

	if lintError != nil {
		l.Messages = append(l.Messages, Message{Text: lintError.Error(), Severity: severity})

		if severity > l.HighestSeverity {
			l.HighestSeverity = severity
		}
	}
	return lintError == nil
}
