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
	"k8s.io/helm/pkg/timeconv"
)

var statusHelp = `
This command shows the status of a named release.
`

type statusCmd struct {
	release string
	out     io.Writer
	client  helm.Interface
}

func newStatusCmd(client helm.Interface, out io.Writer) *cobra.Command {
	status := &statusCmd{
		out:    out,
		client: client,
	}
	cmd := &cobra.Command{
		Use:               "status [flags] RELEASE_NAME",
		Short:             "displays the status of the named release",
		Long:              statusHelp,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			status.release = args[0]
			if status.client == nil {
				status.client = helm.NewClient(helm.Host(tillerHost))
			}
			return status.run()
		},
	}
	return cmd
}

func (s *statusCmd) run() error {
	res, err := s.client.ReleaseStatus(s.release)
	if err != nil {
		return prettyError(err)
	}

	fmt.Fprintf(s.out, "Last Deployed: %s\n", timeconv.String(res.Info.LastDeployed))
	fmt.Fprintf(s.out, "Status: %s\n", res.Info.Status.Code)
	if res.Info.Status.Details != nil {
		fmt.Fprintf(s.out, "Details: %s\n", res.Info.Status.Details)
	}
	return nil
}
