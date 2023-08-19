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

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
)

const rollbackDesc = `
This command rolls back a release to a previous revision.

The first argument of the rollback command is the name of a release, and the
second is a revision (version) number. If this argument is omitted, it will
roll back to the previous release.

To see revision numbers, run 'helm history RELEASE'.
`

func newRollbackCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewRollback(cfg)

	cmd := &cobra.Command{
		Use:   "rollback <RELEASE> [REVISION]",
		Short: "roll back a release to a previous revision",
		Long:  rollbackDesc,
		Args:  require.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListReleases(toComplete, args, cfg)
			}

			if len(args) == 1 {
				return compListRevisions(toComplete, cfg, args[0])
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				ver, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("could not convert revision to a number: %v", err)
				}
				client.Version = ver
			}

			if err := client.Run(args[0]); err != nil {
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
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "prevent hooks from running during rollback")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.Wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment, StatefulSet, or ReplicaSet are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVar(&client.WaitForJobs, "wait-for-jobs", false, "if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVar(&client.CleanupOnFail, "cleanup-on-fail", false, "allow deletion of new resources created in this rollback when rollback fails")
	f.IntVar(&client.MaxHistory, "history-max", settings.MaxHistory, "limit the maximum number of revisions saved per release. Use 0 for no limit")

	return cmd
}
