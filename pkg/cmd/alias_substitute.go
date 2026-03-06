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

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/registry"
)

const aliasSubstituteDesc = `
Set or remove a registry substitution.
`

func newAliasSubstituteCmd(_ *action.Configuration, _ io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "substitute URL [URL]",
		Short:             "configure a OCI registry URL substitution",
		Long:              aliasSubstituteDesc,
		Args:              require.MinimumNArgs(1),
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, args []string) error {
			substitution := args[0]
			var replacement *string
			if len(args) > 1 {
				replacement = &args[1]
			}

			err := setSubstitution(settings.RegistryAliasConfig, substitution, replacement)

			return err
		},
	}

	return cmd
}

func setSubstitution(aliasesFile, substitution string, replacement *string) error {
	a, err := registry.LoadAliasesFile(aliasesFile)
	if err != nil && !isNotExist(err) {
		return fmt.Errorf("failed to load aliases: %w", err)
	}

	if replacement != nil {
		a.SetSubstitution(substitution, *replacement)
	} else {
		a.RemoveSubstitution(substitution)
	}

	if err := a.WriteAliasesFile(aliasesFile, 0o644); err != nil {
		return err
	}

	return nil
}
