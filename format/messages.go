package format

import (
	"fmt"
	"os"

	"github.com/ghodss/yaml"
)

// This is all just placeholder.

// Err prints an error message to Stderr.
func Err(msg string, v ...interface{}) {
	msg = "[ERROR] " + msg + "\n"
	fmt.Fprintf(os.Stderr, msg, v...)
}

// Info prints an informational message to Stdout.
func Info(msg string, v ...interface{}) {
	msg = "[INFO] " + msg + "\n"
	fmt.Fprintf(os.Stdout, msg, v...)
}

// Msg prints a raw message to Stdout.
func Msg(msg string, v ...interface{}) {
	fmt.Fprintf(os.Stdout, msg, v...)
}

// Success is an achievement marked by pretty output.
func Success(msg string, v ...interface{}) {
	msg = "[Success] " + msg + "\n"
	fmt.Fprintf(os.Stdout, msg, v...)
}

// Warning emits a warning message.
func Warning(msg string, v ...interface{}) {
	msg = "[Warning] " + msg + "\n"
	fmt.Fprintf(os.Stdout, msg, v...)
}

func YAML(v interface{}) error {
	y, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("Failed to serialize to yaml: %s", v.(string))
	}

	Msg(string(y))
	return nil
}
