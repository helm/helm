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

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/timeconv"
)

var statusHelp = `
This command shows the status of a named release.
`

var statusCommand = &cobra.Command{
	Use:               "status [flags] RELEASE_NAME",
	Short:             "displays the status of the named release",
	Long:              statusHelp,
	RunE:              status,
	PersistentPreRunE: setupConnection,
}

func init() {
	RootCommand.AddCommand(statusCommand)
}

func status(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errReleaseRequired
	}

	res, err := helm.GetReleaseStatus(args[0])
	if err != nil {
		return prettyError(err)
	}

	fmt.Printf("Last Deployed: %s\n", timeconv.String(res.Info.LastDeployed))
	fmt.Printf("Status: %s\n", res.Info.Status.Code)
	if res.Info.Status.Details != nil {
		fmt.Printf("Details: %s\n", res.Info.Status.Details)
	}

	return nil
}
