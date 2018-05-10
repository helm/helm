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
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
)

const rollbackDesc = `
This command rolls back a release to a previous revision.

The first argument of the rollback command is the name of a release, and the
second is a revision (version) number. To see revision numbers, run
'helm history RELEASE'.
`

type rollbackOptions struct {
	name         string
	revision     int
	dryRun       bool
	recreate     bool
	force        bool
	disableHooks bool
	client       helm.Interface
	timeout      int64
	wait         bool
}

func newRollbackCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &rollbackOptions{client: c}

	cmd := &cobra.Command{
		Use:   "rollback [flags] [RELEASE] [REVISION]",
		Short: "roll back a release to a previous revision",
		Long:  rollbackDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "release name", "revision number"); err != nil {
				return err
			}

			o.name = args[0]

			v64, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return errors.Wrapf(err, "invalid revision number '%q'", args[1])
			}

			o.revision = int(v64)
			o.client = ensureHelmClient(o.client, false)
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.dryRun, "dry-run", false, "simulate a rollback")
	f.BoolVar(&o.recreate, "recreate-pods", false, "performs pods restart for the resource if applicable")
	f.BoolVar(&o.force, "force", false, "force resource update through delete/recreate if needed")
	f.BoolVar(&o.disableHooks, "no-hooks", false, "prevent hooks from running during rollback")
	f.Int64Var(&o.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&o.wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout")

	return cmd
}

func (o *rollbackOptions) run(out io.Writer) error {
	_, err := o.client.RollbackRelease(
		o.name,
		helm.RollbackDryRun(o.dryRun),
		helm.RollbackRecreate(o.recreate),
		helm.RollbackForce(o.force),
		helm.RollbackDisableHooks(o.disableHooks),
		helm.RollbackVersion(o.revision),
		helm.RollbackTimeout(o.timeout),
		helm.RollbackWait(o.wait))
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Rollback was a success! Happy Helming!\n")

	return nil
}
