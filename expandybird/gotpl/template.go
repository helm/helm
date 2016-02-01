package action

import (
	"io"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/kubernetes/deployment-manager/log"
)

// Render renders a template and values into an output stream.
//
// tpl should be a string template.
func Render(out io.Writer, tpl string, vals interface{}) error {
	t, err := template.New("helmTpl").Funcs(sprig.TxtFuncMap()).Parse(tpl)
	if err != nil {
		return err
	}

	log.Debug("Vals: %#v", vals)

	if err := t.ExecuteTemplate(out, "helmTpl", vals); err != nil {
		return err
	}
	return nil
}
