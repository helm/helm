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
	"io"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/registry"
)

const aliasSetDesc = `
Set or remove an alias for an OCI registry.
`

func newAliasSetCmd(_ *action.Configuration, _ io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set NAME [URL]",
		Short:             "configure the named alias",
		Long:              aliasSetDesc,
		Args:              require.MinimumNArgs(1),
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, args []string) error {
			alias := args[0]
			var value *string
			if len(args) > 1 {
				value = &args[1]
			}

			err := setAlias(settings.RegistryAliasConfig, alias, value)

			return err
		},
	}

	return cmd
}

func setAlias(aliasesFile, alias string, value *string) error {
	if strings.Contains(alias, "/") {
		return fmt.Errorf("alias name (%s) contains '/', please specify a different name without '/'", alias)
	}

	a, err := registry.LoadAliasesFile(aliasesFile)
	if err != nil && !isNotExist(err) {
		return fmt.Errorf("failed to load aliases: %w", err)
	}

	if value != nil {
		a.SetAlias(alias, *value)
	} else {
		a.RemoveAlias(alias)
	}

	if err := a.WriteAliasesFile(aliasesFile, 0o644); err != nil {
		return err
	}

	return nil
}
