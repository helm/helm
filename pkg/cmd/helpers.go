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

package cmd

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
)

func addDryRunFlag(cmd *cobra.Command) {
	// --dry-run options with expected outcome:
	// - Not set means no dry run and server is contacted.
	// - Set with no value, a value of client, or a value of true and the server is not contacted
	// - Set with a value of false, none, or false and the server is contacted
	// The true/false part is meant to reflect some legacy behavior while none is equal to "".
	f := cmd.Flags()
	f.String(
		"dry-run",
		"none",
		`simulates the operation without persisting changes. Must be one of: "none" (default), "client", or "server". '--dry-run=none' executes the operation normally and persists changes (no simulation). '--dry-run=client' simulates the operation client-side only and avoids cluster connections. '--dry-run=server' simulates the operation on the server, requiring cluster connectivity.`)
	f.Lookup("dry-run").NoOptDefVal = "unset"
}

// Determine the `action.DryRunStrategy` given -dry-run=<value>` flag (or absence of)
// Legacy usage of the flag: boolean values, and `--dry-run` (without value) are supported, and log warnings emitted
func cmdGetDryRunFlagStrategy(cmd *cobra.Command, isTemplate bool) (action.DryRunStrategy, error) {

	f := cmd.Flag("dry-run")
	v := f.Value.String()

	switch v {
	case f.NoOptDefVal:
		slog.Warn(`--dry-run is deprecated and should be replaced with '--dry-run=client'`)
		return action.DryRunClient, nil
	case string(action.DryRunClient):
		return action.DryRunClient, nil
	case string(action.DryRunServer):
		return action.DryRunServer, nil
	case string(action.DryRunNone):
		if isTemplate {
			// Special case hack for `helm template`, which is always a dry run
			return action.DryRunNone, fmt.Errorf(`invalid dry-run value (%q). Must be "server" or "client"`, v)
		}
		return action.DryRunNone, nil
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return action.DryRunNone, fmt.Errorf(`invalid dry-run value (%q). Must be "none", "server", or "client"`, v)
	}

	if isTemplate && !b {
		// Special case hack for `helm template`, which is always a dry run
		return action.DryRunNone, fmt.Errorf(`invalid dry-run value (%q). Must be "server" or "client"`, v)
	}

	result := action.DryRunNone
	if b {
		result = action.DryRunClient
	}
	slog.Warn(fmt.Sprintf(`boolean '--dry-run=%v' flag is deprecated and must be replaced with '--dry-run=%s'`, v, result))

	return result, nil
}
