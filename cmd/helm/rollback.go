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

package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
)

const rollbackDesc = `
This command rolls back a release to a previous revision.

The first argument of the rollback command is the name of a release, and the
second is a revision (version) number. To see revision numbers, run
'helm history RELEASE'.
`

func newRollbackCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewRollback(cfg)

	cmd := &cobra.Command{
		Use:   "rollback [RELEASE] [REVISION]",
		Short: "roll back a release to a previous revision",
		Long:  rollbackDesc,
		Args:  require.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := client.Run(args[0])
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Rollback was a success! Happy Helming!\n")

			return nil
		},
	}

	client.AddFlags(cmd.Flags())

	return cmd
}
