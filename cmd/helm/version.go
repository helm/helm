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
	"k8s.io/helm/pkg/version"
)

type versionCmd struct {
	out    io.Writer
	client helm.Interface
}

func newVersionCmd(c helm.Interface, out io.Writer) *cobra.Command {
	version := &versionCmd{
		client: c,
		out:    out,
	}
	cmd := &cobra.Command{
		Use:               "version",
		Short:             "print the client/server version information",
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			version.client = ensureHelmClient(version.client)
			return version.run()
		},
	}
	return cmd
}

func (v *versionCmd) run() error {
	// Regardless of whether we can talk to server or not, just print the client
	// version.
	cv := version.GetVersionProto()
	fmt.Fprintf(v.out, "Client: {SemVer: %s GitCommit: %s}\n", cv.SemVer, cv.GitCommit)

	resp, err := v.client.GetVersion()
	if err != nil {
		return err
	}
	fmt.Fprintf(v.out, "Server: {SemVer: %s GitCommit: %s}\n", resp.Version.SemVer, resp.Version.GitCommit)
	return nil
}
