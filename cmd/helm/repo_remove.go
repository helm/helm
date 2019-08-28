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
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
)

type repoRemoveOptions struct {
	name      string
	repoFile  string
	repoCache string
}

func newRepoRemoveCmd(out io.Writer) *cobra.Command {
	o := &repoRemoveOptions{}
	cmd := &cobra.Command{
		Use:     "remove [NAME]",
		Aliases: []string{"rm"},
		Short:   "remove a chart repository",
		Args:    require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.repoFile = settings.RepositoryConfig
			o.repoCache = settings.RepositoryCache
			o.name = args[0]
			return o.run(out)
		},
	}

	return cmd
}

func (o *repoRemoveOptions) run(out io.Writer) error {
	r, err := repo.LoadFile(o.repoFile)
	if err != nil {
		return err
	}

	if !r.Remove(o.name) {
		return errors.Errorf("no repo named %q found", o.name)
	}
	if err := r.WriteFile(o.repoFile, 0644); err != nil {
		return err
	}

	if err := removeRepoCache(o.repoCache, o.name); err != nil {
		return err
	}

	fmt.Fprintf(out, "%q has been removed from your repositories\n", o.name)
	return nil
}

func removeRepoCache(root, name string) error {
	idx := filepath.Join(root, helmpath.CacheIndexFile(name))
	if _, err := os.Stat(idx); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "can't remove index file %s", idx)
	}
	return os.Remove(idx)
}
