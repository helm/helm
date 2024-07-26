package lint

import (
	"bufio"
	"fmt"
	"helm.sh/helm/v3/pkg/lint/support"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var Debug bool

type Ignorer struct {
	Patterns map[string][]string
}

func NewIgnorer(ignoreFilePath string) (*Ignorer, error) {
	patterns, err := parseFromFilePath(ignoreFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ignore file: %w", err)
	}

	return &Ignorer{ Patterns: patterns }, nil
}

func (i *Ignorer) MatchMessage(msg support.Message) bool {
	errText := msg.Err.Error()
	errorFullPath := extractFullPathFromError(errText)
	if len(errorFullPath) == 0 {
		debug("Unable to find a path for message, guess we'll keep it: %s", errText)
		return false
	}
	debug("Extracted full path: %s\n", errorFullPath)
	for ignorablePath, pathPatterns := range i.Patterns {
		cleanIgnorablePath := filepath.Clean(ignorablePath)
		if strings.Contains(errorFullPath, cleanIgnorablePath) {
			for _, pattern := range pathPatterns {
				if strings.Contains(msg.Err.Error(), pattern) {
					debug("Ignoring message: [%s] %s\n\n", errorFullPath, errText)
					return true
				}
			}
		}
	}
	debug("keeping unignored message: [%s]", errText)
	return false
}

func (i *Ignorer) MatchError(err error) bool {
	errText := err.Error()
	errorFullPath := extractFullPathFromError(errText)
	if len(errorFullPath) == 0 {
		debug("Unable to find a path for error, guess we'll keep it: %s", errText)
		return false
	}
	debug("Extracted full path: %s\n", errorFullPath)
	for ignorablePath, pathPatterns := range i.Patterns {
		cleanIgnorablePath := filepath.Clean(ignorablePath)
		if strings.Contains(errorFullPath, cleanIgnorablePath) {
			for _, pattern := range pathPatterns {
				if strings.Contains(err.Error(), pattern) {
					debug("Ignoring error: [%s] %s\n\n", errorFullPath, errText)
					return true
				}
			}
		}
	}
	debug("keeping unignored error: [%s]", errText)
	return false
}

func (i *Ignorer) FilterIgnoredErrors(errors []error) []error {
	filteredErrors := make([]error, 0)
	for _, err := range errors {
		if !i.MatchError(err) {
			filteredErrors = append(filteredErrors, err)
		}
	}
	return filteredErrors
}

func (i *Ignorer) FilterIgnoredMessages(messages []support.Message) []support.Message {
	filteredMessages := make([]support.Message, 0)
	for _, msg := range messages {
		if !i.MatchMessage(msg) {
			filteredMessages = append(filteredMessages, msg)
		}
	}
	return filteredMessages
}

// TODO: figure out & fix or remove
func extractFullPathFromError(errorString string) string {
	parts := strings.Split(errorString, ":")
	if len(parts) > 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func parseFromFilePath(filePath string) (map[string][]string, error) {
	patterns := make(map[string][]string)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Printf("Failed to close ignore file at [%s]: %v", filePath, err)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) > 1 {
				// Check if the key already exists and append to its slice
				patterns[parts[0]] = append(patterns[parts[0]], parts[1])
			} else if len(parts) == 1 {
				// Add an empty pattern if only the path is present
				patterns[parts[0]] = append(patterns[parts[0]], "")
			}
		}
	}

	return patterns, scanner.Err()
}

// TODO: DELETE
var logger = log.New(os.Stderr, "[debug] ", log.Lshortfile)
func debug(format string, v ...interface{}) {
	if Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		logger.Output(2, fmt.Sprintf(format, v...))
	}
}
// END TODO: DELETE


/* TODO HIP-0019
- find ignore file path for a subchart
- add a chart or two for the end to end tests via testdata like in pkg/lint/lint_test.go
- review debug / output patterns across the helm project

Later/never
- XDG support
- helm config file support
- ignore file validation
-
*/
