package dm

import (
	"github.com/deis/helm-dm/kubectl"
)

// Uninstall uses kubectl to uninstall the base DM.
//
// Returns the string output received from the operation, and an error if the
// command failed.
func Uninstall(runner kubectl.Runner) (string, error) {
	o, err := runner.Delete("dm", "Namespace", "dm")
	return string(o), err
}
