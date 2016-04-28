package lint

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"
)

// Templates lints a chart's templates.
func Templates(basepath string) (messages []Message) {
	messages = []Message{}
	path := filepath.Join(basepath, "templates")
	if fi, err := os.Stat(path); err != nil {
		messages = append(messages, Message{Severity: WarningSev, Text: "No templates"})
		return
	} else if !fi.IsDir() {
		messages = append(messages, Message{Severity: ErrorSev, Text: "'templates' is not a directory"})
		return
	}

	tpl := template.New("tpl").Funcs(sprig.TxtFuncMap())

	err := filepath.Walk(basepath, func(name string, fi os.FileInfo, e error) error {
		// If an error is returned, we fail. Non-fatal errors should just be
		// added directly to messages.
		if e != nil {
			return e
		}
		if fi.IsDir() {
			return nil
		}

		data, err := ioutil.ReadFile(name)
		if err != nil {
			messages = append(messages, Message{
				Severity: ErrorSev,
				Text:     fmt.Sprintf("cannot read %s: %s", name, err),
			})
			return nil
		}

		// An error rendering a file should emit a warning.
		newtpl, err := tpl.Parse(string(data))
		if err != nil {
			messages = append(messages, Message{
				Severity: ErrorSev,
				Text:     fmt.Sprintf("error processing %s: %s", name, err),
			})
			return nil
		}
		tpl = newtpl
		return nil
	})

	if err != nil {
		messages = append(messages, Message{Severity: ErrorSev, Text: err.Error()})
	}

	return
}
