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

	"k8s.io/helm/pkg/version"
)

const versionDesc = `
Show the version for Helm.

This will print a representation the version of Helm.
The output will look something like this:

Client: &version.Version{SemVer:"v2.0.0", GitCommit:"ff52399e51bb880526e9cd0ed8386f6433b74da1", GitTreeState:"clean"}

- SemVer is the semantic version of the release.
- GitCommit is the SHA for the commit that this version was built from.
- GitTreeState is "clean" if there are no local code changes when this binary was
  built, and "dirty" if the binary was built from locally modified code.
`

type versionCmd struct {
	out      io.Writer
	short    bool
	template string
}

func newVersionCmd(out io.Writer) *cobra.Command {
	version := &versionCmd{out: out}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "print the client version information",
		Long:  versionDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return version.run()
		},
	}
	f := cmd.Flags()
	f.BoolVar(&version.short, "short", false, "print the version number")
	f.StringVar(&version.template, "template", "", "template for version string format")

	return cmd
}

func (v *versionCmd) run() error {
	// Store map data for template rendering
	data := map[string]interface{}{}

	cv := version.GetVersionProto()
	if v.template != "" {
		data["Client"] = cv
		return tpl(v.template, data, v.out)
	}
	fmt.Fprintf(v.out, "Client: %s\n", formatVersion(cv, v.short))
	return nil
}

func formatVersion(v *version.Version, short bool) string {
	if short {
		return fmt.Sprintf("%s+g%s", v.SemVer, v.GitCommit[:7])
	}
	return fmt.Sprintf("%#v", v)
}
