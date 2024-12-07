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
	"fmt"
	"io"
	"log"

	"github.com/spf13/cobra"
	k8sLabels "k8s.io/apimachinery/pkg/labels"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

type metadataWriter struct {
	metadata *action.Metadata
}

func newGetMetadataCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var outfmt output.Format
	client := action.NewGetMetadata(cfg)

	cmd := &cobra.Command{
		Use:   "metadata RELEASE_NAME",
		Short: "This command fetches metadata for a given release",
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return noMoreArgsComp()
			}
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			releaseMetadata, err := client.Run(args[0])
			if err != nil {
				return err
			}
			return outfmt.Write(out, &metadataWriter{releaseMetadata})
		},
	}

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "specify release revision")
	err := cmd.RegisterFlagCompletionFunc("revision", func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return compListRevisions(toComplete, cfg, args[0])
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	})

	if err != nil {
		log.Fatal(err)
	}

	bindOutputFlag(cmd, &outfmt)

	return cmd
}

func (w metadataWriter) WriteTable(out io.Writer) error {
	_, _ = fmt.Fprintf(out, "NAME: %v\n", w.metadata.Name)
	_, _ = fmt.Fprintf(out, "CHART: %v\n", w.metadata.Chart)
	_, _ = fmt.Fprintf(out, "VERSION: %v\n", w.metadata.Version)
	_, _ = fmt.Fprintf(out, "APP_VERSION: %v\n", w.metadata.AppVersion)
	_, _ = fmt.Fprintf(out, "ANNOTATIONS: %v\n", k8sLabels.Set(w.metadata.Annotations).String())
	_, _ = fmt.Fprintf(out, "DEPENDENCIES: %v\n", w.metadata.FormattedDepNames())
	_, _ = fmt.Fprintf(out, "NAMESPACE: %v\n", w.metadata.Namespace)
	_, _ = fmt.Fprintf(out, "REVISION: %v\n", w.metadata.Revision)
	_, _ = fmt.Fprintf(out, "STATUS: %v\n", w.metadata.Status)
	_, _ = fmt.Fprintf(out, "DEPLOYED_AT: %v\n", w.metadata.DeployedAt)

	return nil
}

func (w metadataWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, w.metadata)
}

func (w metadataWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, w.metadata)
}
