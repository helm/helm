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
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/version"
)

type versionCmd struct {
	out        io.Writer
	client     helm.Interface
	clientOnly bool
	serverOnly bool
}

func newVersionCmd(c helm.Interface, out io.Writer) *cobra.Command {
	version := &versionCmd{
		client: c,
		out:    out,
	}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "print the client/server version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !version.clientOnly {
				// We do this manually instead of in PreRun because we only
				// need a tunnel if server version is requested.
				setupConnection(cmd, args)
			}
			version.client = ensureHelmClient(version.client)
			return version.run()
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&version.clientOnly, "client-only", "c", false, "if set does not query Tiller version")
	f.BoolVarP(&version.serverOnly, "server-only", "s", false, "if set does not query Helm client version")

	return cmd
}

func (v *versionCmd) run() error {

	if v.clientOnly && v.serverOnly {
		return errors.New("cannot set both client-only and server-only")
	}

	if !v.serverOnly {
		cv := version.GetVersionProto()
		fmt.Fprintf(v.out, "Client: %#v\n", cv)
	}

	if v.clientOnly {
		return nil
	}

	resp, err := v.client.GetVersion()
	if err != nil {
		if grpc.Code(err) == codes.Unimplemented {
			return errors.New("server is too old to know its version")
		}
		return err
	}
	fmt.Fprintf(v.out, "Server: %#v\n", resp.Version)
	return nil
}
