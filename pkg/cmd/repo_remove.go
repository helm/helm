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

package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/repo/v1"
)

type repoRemoveOptions struct {
	names     []string
	repoFile  string
	repoCache string
}

func newRepoRemoveCmd(out io.Writer) *cobra.Command {
	o := &repoRemoveOptions{}

	cmd := &cobra.Command{
		Use:     "remove [REPO1 [REPO2 ...]]",
		Aliases: []string{"rm"},
		Short:   "remove one or more chart repositories",
		Args:    require.MinimumNArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListRepos(toComplete, args), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			o.repoFile = settings.RepositoryConfig
			o.repoCache = settings.RepositoryCache
			o.names = args
			return o.run(out)
		},
	}
	return cmd
}

func (o *repoRemoveOptions) run(out io.Writer) error {
	r, err := repo.LoadFile(o.repoFile)
	if isNotExist(err) || len(r.Repositories) == 0 {
		return errors.New("no repositories configured")
	}

	for _, name := range o.names {
		if !r.Remove(name) {
			return fmt.Errorf("no repo named %q found", name)
		}
		if err := r.WriteFile(o.repoFile, 0600); err != nil {
			return err
		}

		if err := removeRepoCache(o.repoCache, name); err != nil {
			return err
		}
		fmt.Fprintf(out, "%q has been removed from your repositories\n", name)
	}

	return nil
}

func removeRepoCache(root, name string) error {
	idx := filepath.Join(root, helmpath.CacheChartsFile(name))
	if _, err := os.Stat(idx); err == nil {
		os.Remove(idx)
	}

	idx = filepath.Join(root, helmpath.CacheIndexFile(name))
	if _, err := os.Stat(idx); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("can't remove index file %s: %w", idx, err)
	}
	return os.Remove(idx)
}
