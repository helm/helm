package dm

import (
	"github.com/deis/helm-dm/pkg/kubectl"
)

// Uninstall uses kubectl to uninstall the base DM.
//
// Returns the string output received from the operation, and an error if the
// command failed.
func Uninstall(runner kubectl.Runner) (string, error) {
	o, err := runner.Delete("dm", "Namespace")
	return string(o), err
}
