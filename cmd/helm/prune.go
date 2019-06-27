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

	"k8s.io/helm/pkg/helm"
)

const pruneDesc = `
This command purges all deleted releases, cleaning them from removing them permanently so they can't be rolled back.
Internally, it performs the 'helm list --deleted' command to find all releases with the stats 'Status_DELETED',
and then runs the 'helm delete --purge' command on those releases.

The prune command is designed to encourage a workflow of not immediately purging releases.
This gives users the opportunity to 'rollback' a delete if necessary, and then 'prune' their releases
when they are sure that they are not necessary anymore. 
`

type pruneCmd struct {
	disableHooks bool
	timeout      int64

	out    io.Writer
	client helm.Interface
}

func newPruneCmd(c helm.Interface, out io.Writer) *cobra.Command {
	prune := &pruneCmd{
		out:    out,
		client: c,
	}

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Purges deleted releases",
		Long:    pruneDesc,
		PreRunE: func(_ *cobra.Command, _ []string) error { return setupConnection() },
		RunE: func(cmd *cobra.Command, args []string) error {
			prune.client = ensureHelmClient(prune.client)

			if err := prune.run(); err != nil {
				return err
			}

			fmt.Fprintf(out, "Releases pruned\n")
			return nil
		},
	}

	f := cmd.Flags()
	settings.AddFlagsTLS(f)
	f.BoolVar(&prune.disableHooks, "no-hooks", false, "Prevent hooks from running during deletion")
	f.Int64Var(&prune.timeout, "timeout", 300, "Time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")

	// set defaults from environment
	settings.InitTLS(f)

	return cmd
}

func (d *pruneCmd) run() error {
	opts := []helm.DeleteOption{
		helm.DeleteDisableHooks(d.disableHooks),
		helm.DeleteTimeout(d.timeout),
	}
	_, errs := d.client.PruneReleases(opts...)
	for _, err := range errs {
		if err != nil {
			return prettyError(err)
		}
	}
	return nil
}
