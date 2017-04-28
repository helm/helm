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
	"io"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"fmt"
	"k8s.io/helm/pkg/proto/hapi/release"
)

var logsHelp = `
This command gets logs for a named release
`

type logsCmd struct {
	release string
	out     io.Writer
	client  helm.Interface
	version int32
}

func newLogsCmd(client helm.Interface, out io.Writer) *cobra.Command {
	logs := &logsCmd{
		out:    out,
		client: client,
	}

	cmd := &cobra.Command{
		Use:               "logs [flags] RELEASE_NAME",
		Short:             "Streams logs for the given release",
		Long:              logsHelp,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			logs.release = args[0]
			if logs.client == nil {
				logs.client = helm.NewClient(helm.Host(settings.TillerHost))
			}
			return logs.run()
		},
	}

	return cmd
}

func (l *logsCmd) run() error {
	done := make(chan struct{})
	stream, err := l.client.ReleaseLogs(l.release, release.LogLevel_DEBUG, done, release.LogSource_SYSTEM, release.LogSource_POD)

	fmt.Println("Listening for logs")
	for {
		select {
		case l, ok := <-stream:
			if !ok {
				return nil
			}
			fmt.Println(l)
		}
	}

	if err != nil {
		done <- struct{}{}
		return prettyError(err)
	}

	return nil
}

