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
)

const completionDesc = `
Generate bash autocompletions script for Helm.

This command can generate shell autocompletions.

	$ helm completion

Can be sourced as such

	$ source <(helm completion)
`

type completionCmd struct {
	out    io.Writer
	topCmd *cobra.Command
}

func newCompletionCmd(out io.Writer, topCmd *cobra.Command) *cobra.Command {
	cc := &completionCmd{out: out, topCmd: topCmd}

	cmd := &cobra.Command{
		Use:    "completion",
		Short:  "Generate bash autocompletions script",
		Long:   completionDesc,
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc.run()
			return nil
		},
	}

	return cmd
}

func (c *completionCmd) run() error {

	return c.topCmd.GenBashCompletion(c.out)
}
