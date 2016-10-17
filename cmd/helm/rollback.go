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

	"k8s.io/helm/pkg/helm"
)

const rollbackDesc = `
This command rolls back a release to the previous revision.
The argument of the rollback command is the name of a release.
`

type rollbackCmd struct {
	name         string
	version      int32
	dryRun       bool
	disableHooks bool
	out          io.Writer
	client       helm.Interface
}

func newRollbackCmd(c helm.Interface, out io.Writer) *cobra.Command {
	rollback := &rollbackCmd{
		out:    out,
		client: c,
	}

	cmd := &cobra.Command{
		Use:               "rollback [RELEASE]",
		Short:             "roll back a release to a previous revision",
		Long:              rollbackDesc,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "release name"); err != nil {
				return err
			}
			rollback.name = args[0]
			rollback.client = ensureHelmClient(rollback.client)
			return rollback.run()
		},
	}

	f := cmd.Flags()
	f.Int32Var(&rollback.version, "revision", 0, "revision to deploy")
	f.BoolVar(&rollback.dryRun, "dry-run", false, "simulate a rollback")
	f.BoolVar(&rollback.disableHooks, "no-hooks", false, "prevent hooks from running during rollback")

	return cmd
}

func (r *rollbackCmd) run() error {
	_, err := r.client.RollbackRelease(
		r.name,
		helm.RollbackDryRun(r.dryRun),
		helm.RollbackDisableHooks(r.disableHooks),
		helm.RollbackVersion(r.version),
	)
	if err != nil {
		return prettyError(err)
	}

	fmt.Fprintf(r.out, "Rollback was a success! Happy Helming!\n")

	return nil
}
