/*
Copyright The Helm Authors.

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

package output

import (
	"github.com/fatih/color"

	release "helm.sh/helm/v4/pkg/release/v1"
)

// ColorizeStatus returns a colorized version of the status string based on the status value
func ColorizeStatus(status release.Status, noColor bool) string {
	// Disable color if requested
	if noColor {
		return status.String()
	}

	switch status {
	case release.StatusDeployed:
		return color.GreenString(status.String())
	case release.StatusFailed:
		return color.RedString(status.String())
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback, release.StatusUninstalling:
		return color.YellowString(status.String())
	case release.StatusUnknown:
		return color.RedString(status.String())
	default:
		// For uninstalled, superseded, and any other status
		return status.String()
	}
}

// ColorizeHeader returns a colorized version of a header string
func ColorizeHeader(header string, noColor bool) string {
	// Disable color if requested
	if noColor {
		return header
	}

	// Use bold for headers
	return color.New(color.Bold).Sprint(header)
}

// ColorizeNamespace returns a colorized version of a namespace string
func ColorizeNamespace(namespace string, noColor bool) string {
	// Disable color if requested
	if noColor {
		return namespace
	}

	// Use cyan for namespaces
	return color.CyanString(namespace)
}
