package format

import (
	"fmt"
	"os"
)

// This is all just placeholder.

func Error(msg string, v ...interface{}) {
	msg = "[ERROR]" + msg + "\n"
	fmt.Fprintf(os.Stderr, msg, v...)
}

func Info(msg string, v ...interface{}) {
	msg = "[INFO]" + msg + "\n"
	fmt.Fprintf(os.Stdout, msg, v...)
}

func Msg(msg string, v ...interface{}) {
	fmt.Fprintf(os.Stdout, msg, v...)
}
