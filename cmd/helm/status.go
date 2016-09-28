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
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/timeconv"
)

var statusHelp = `
This command shows the status of a named release.
`

type statusCmd struct {
	release string
	out     io.Writer
	client  helm.Interface
	version int32
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

	cmd.PersistentFlags().Int32Var(&status.version, "revision", 0, "If set, display the status of the named release with revision")

	return cmd
}

func (s *statusCmd) run() error {
	res, err := s.client.ReleaseStatus(s.release, helm.StatusReleaseVersion(s.version))
	if err != nil {
		return prettyError(err)
	}

	PrintStatus(s.out, res)
	return nil
}

// PrintStatus prints out the status of a release. Shared because also used by
// install / upgrade
func PrintStatus(out io.Writer, res *services.GetReleaseStatusResponse) {
	if res.Info.LastDeployed != nil {
		fmt.Fprintf(out, "Last Deployed: %s\n", timeconv.String(res.Info.LastDeployed))
	}
	fmt.Fprintf(out, "Namespace: %s\n", res.Namespace)
	fmt.Fprintf(out, "Status: %s\n", res.Info.Status.Code)
	if res.Info.Status.Details != nil {
		fmt.Fprintf(out, "Details: %s\n", res.Info.Status.Details)
	}
	fmt.Fprintf(out, "\n")
	if len(res.Info.Status.Resources) > 0 {
		fmt.Fprintf(out, "Resources:\n%s\n", res.Info.Status.Resources)
	}
	if len(res.Info.Status.Notes) > 0 {
		fmt.Fprintf(out, "Notes:\n%s\n", res.Info.Status.Notes)
	}
}
