package ignore

import (
	"fmt"
	"helm.sh/helm/v3/pkg/lint/support"
	"log"
)

// DefaultIgnoreFileName is the name of the lint ignore file
const DefaultIgnoreFileName = ".helmlintignore"

var debugFn func(format string, v ...interface{})

type Ignorer struct {
	ChartPath string
	Matchers  []MatchesErrors
}

func NewIgnorer(chartPath string, lintIgnorePath string, debugLogFn func(string, ...interface{})) (*Ignorer, error) {
	debugFn = debugLogFn
	matchers, err := LoadFromFilePath(chartPath, lintIgnorePath)
	if err != nil {
		return nil, err
	}

	return &Ignorer{ChartPath: chartPath, Matchers: matchers}, nil
}

// FilterMessages Verify what messages can be kept in the output, using also the error as a verification
func (i *Ignorer) FilterMessages(messages []support.Message) []support.Message {
	out := make([]support.Message, 0, len(messages))
	for _, msg := range messages {
		if i.ShouldKeepMessage(msg) {
			out = append(out, msg)
		}
	}
	return out
}

func (i *Ignorer) ShouldKeepMessage(msg support.Message) bool {
	return i.ShouldKeepError(msg.Err)
}

// ShouldKeepError is used to verify if the error associated with the message need to be kept, or it can be ignored, called by FilterMessages and in the pkg/action/lint.go Run main function
func (i *Ignorer) ShouldKeepError(err error) bool {
	// if any of our Matchers match the rule, we can discard it
	for _, rule := range i.Matchers {
		if rule.Match(err) {
			debug("lint ignore rule matched this error, we should suppress it.", "errText", err.Error(), rule.LogAttrs())
			return false
		}
	}

	// if we can't find a reason to discard it, we keep it
	debug("no lint ignore rules matched this error, we should NOT suppress it.", "errText", err.Error())
	return true
}

var defaultDebugFn = func(format string, v ...interface{}) {
	format = fmt.Sprintf("[debug] %s\n", format)
	_ = log.Output(2, fmt.Sprintf(format, v...))
}
