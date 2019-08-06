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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
)

func newRepoRemoveCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [NAME]",
		Aliases: []string{"rm"},
		Short:   "remove a chart repository",
		Args:    require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeRepoLine(out, args[0])
		},
	}

	return cmd
}

func removeRepoLine(out io.Writer, name string) error {
	repoFile := helmpath.RepositoryFile()
	r, err := repo.LoadFile(repoFile)
	if err != nil {
		return err
	}

	if !r.Remove(name) {
		return errors.Errorf("no repo named %q found", name)
	}
	if err := r.WriteFile(repoFile, 0644); err != nil {
		return err
	}

	if err := removeRepoCache(name); err != nil {
		return err
	}

	fmt.Fprintf(out, "%q has been removed from your repositories\n", name)

	return nil
}

func removeRepoCache(name string) error {
	if _, err := os.Stat(helmpath.CacheIndex(name)); err == nil {
		err = os.Remove(helmpath.CacheIndex(name))
		if err != nil {
			return err
		}
	}
	return nil
}
