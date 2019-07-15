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
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
)

const rollbackDesc = `
This command rolls back a release to a previous revision.

The first argument of the rollback command is the name of a release, and the
second is a revision (version) number. To see revision numbers, run
'helm history RELEASE'.
`

func newRollbackCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewUpgrade(cfg)

	cmd := &cobra.Command{
		Use:   "rollback [RELEASE] [REVISION]",
		Short: "roll back a release to a previous revision",
		Long:  rollbackDesc,
		Args:  require.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			version, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("could not convert revision to a number: %v", err)
			}

			releaseToRollbackTo, err := cfg.Releases.Get(args[0], version)
			if err != nil {
				return err
			}

			if err := client.ValueOptions.MergeValues(settings); err != nil {
				return err
			}

			_, err = client.Run(args[0], releaseToRollbackTo.Chart)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Rollback was a success! Happy Helming!\n")

			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.DryRun, "dry-run", false, "simulate a rollback")
	f.BoolVar(&client.Recreate, "recreate-pods", false, "performs pods restart for the resource if applicable")
	f.BoolVar(&client.Force, "force", false, "force resource update through delete/recreate if needed")
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "disable pre/post upgrade hooks")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.ResetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&client.ReuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' is specified, this is ignored.")
	f.BoolVar(&client.Wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.IntVar(&client.MaxHistory, "history-max", 0, "limit the maximum number of revisions saved per release. Use 0 for no limit.")

	return cmd
}
