/*
Copyright The Helm Authors.
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
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"helm.sh/helm/v3/cmd/helm/require"
)

const docsDesc = `
Generate documentation files for Helm.

This command can generate documentation for Helm in the following formats:

- Markdown
- Man pages

It can also generate bash autocompletions.
`

type docsOptions struct {
	dest          string
	docTypeString string
	topCmd        *cobra.Command
}

func newDocsCmd(out io.Writer) *cobra.Command {
	o := &docsOptions{}

	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate documentation as markdown or man pages",
		Long:   docsDesc,
		Hidden: true,
		Args:   require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.topCmd = cmd.Root()
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.dest, "dir", "./", "directory to which documentation is written")
	f.StringVar(&o.docTypeString, "type", "markdown", "the type of documentation to generate (markdown, man, bash)")

	return cmd
}

func (o *docsOptions) run(out io.Writer) error {
	switch o.docTypeString {
	case "markdown", "mdown", "md":
		return doc.GenMarkdownTree(o.topCmd, o.dest)
	case "man":
		manHdr := &doc.GenManHeader{Title: "HELM", Section: "1"}
		return doc.GenManTree(o.topCmd, manHdr, o.dest)
	case "bash":
		return o.topCmd.GenBashCompletionFile(filepath.Join(o.dest, "completions.bash"))
	default:
		return errors.Errorf("unknown doc type %q. Try 'markdown' or 'man'", o.docTypeString)
	}
}
