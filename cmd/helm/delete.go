/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"k8s.io/helm/pkg/helm"
)

const deleteDesc = `
This command takes a release name, and then deletes the release from Kubernetes.
It removes all of the resources associated with the last release of the chart.

Use the '--dry-run' flag to see which releases will be deleted without actually
deleting them.
`

type deleteOptions struct {
	disableHooks bool  // --no-hooks
	dryRun       bool  // --dry-run
	purge        bool  // --purge
	timeout      int64 // --timeout

	// args
	name string

	client helm.Interface
}

func newDeleteCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &deleteOptions{client: c}

	cmd := &cobra.Command{
		Use:        "delete RELEASE_NAME [...]",
		Aliases:    []string{"del"},
		SuggestFor: []string{"remove", "rm"},
		Short:      "given a release name, delete the release from Kubernetes",
		Long:       deleteDesc,
		Args:       require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.client = ensureHelmClient(o.client, false)

			for i := 0; i < len(args); i++ {
				o.name = args[i]
				if err := o.run(out); err != nil {
					return err
				}

				fmt.Fprintf(out, "release \"%s\" deleted\n", o.name)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.dryRun, "dry-run", false, "simulate a delete")
	f.BoolVar(&o.disableHooks, "no-hooks", false, "prevent hooks from running during deletion")
	f.BoolVar(&o.purge, "purge", false, "remove the release from the store and make its name free for later use")
	f.Int64Var(&o.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")

	return cmd
}

func (o *deleteOptions) run(out io.Writer) error {
	opts := []helm.DeleteOption{
		helm.DeleteDryRun(o.dryRun),
		helm.DeleteDisableHooks(o.disableHooks),
		helm.DeletePurge(o.purge),
		helm.DeleteTimeout(o.timeout),
	}
	res, err := o.client.DeleteRelease(o.name, opts...)
	if res != nil && res.Info != "" {
		fmt.Fprintln(out, res.Info)
	}

	return err
}
