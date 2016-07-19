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
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
)

const deleteDesc = `
This command takes a release name, and then deletes the release from Kubernetes.
It removes all of the resources associated with the last release of the chart.

Use the '--dry-run' flag to see which releases will be deleted without actually
deleting them.
`

type deleteCmd struct {
	name         string
	dryRun       bool
	disableHooks bool

	out    io.Writer
	client helm.Interface
}

func newDeleteCmd(c helm.Interface, out io.Writer) *cobra.Command {
	del := &deleteCmd{
		out:    out,
		client: c,
	}

	cmd := &cobra.Command{
		Use:               "delete [flags] RELEASE_NAME",
		Aliases:           []string{"del"},
		SuggestFor:        []string{"remove", "rm"},
		Short:             "given a release name, delete the release from Kubernetes",
		Long:              deleteDesc,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("command 'delete' requires a release name")
			}
			del.name = args[0]
			del.client = ensureHelmClient(del.client)
			return del.run()
		},
	}
	f := cmd.Flags()
	f.BoolVar(&del.dryRun, "dry-run", false, "simulate a delete")
	f.BoolVar(&del.disableHooks, "no-hooks", false, "prevent hooks from running during deletion")

	return cmd
}

func (d *deleteCmd) run() error {
	opts := []helm.DeleteOption{
		helm.DeleteDryRun(d.dryRun),
		helm.DeleteDisableHooks(d.disableHooks),
	}
	_, err := d.client.DeleteRelease(d.name, opts...)
	return prettyError(err)
}
