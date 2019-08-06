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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/getter"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
)

type repoAddOptions struct {
	name     string
	url      string
	username string
	password string
	noupdate bool

	certFile string
	keyFile  string
	caFile   string
}

func newRepoAddCmd(out io.Writer) *cobra.Command {
	o := &repoAddOptions{}

	cmd := &cobra.Command{
		Use:   "add [NAME] [URL]",
		Short: "add a chart repository",
		Args:  require.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.name = args[0]
			o.url = args[1]

			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.username, "username", "", "chart repository username")
	f.StringVar(&o.password, "password", "", "chart repository password")
	f.BoolVar(&o.noupdate, "no-update", false, "raise error if repo is already registered")
	f.StringVar(&o.certFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&o.keyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")

	return cmd
}

func (o *repoAddOptions) run(out io.Writer) error {
	if err := addRepository(o.name, o.url, o.username, o.password, o.certFile, o.keyFile, o.caFile, o.noupdate); err != nil {
		return err
	}
	fmt.Fprintf(out, "%q has been added to your repositories\n", o.name)
	return nil
}

func addRepository(name, url, username, password string, certFile, keyFile, caFile string, noUpdate bool) error {
	f, err := repo.LoadFile(helmpath.RepositoryFile())
	if err != nil {
		return err
	}

	if noUpdate && f.Has(name) {
		return errors.Errorf("repository name (%s) already exists, please specify a different name", name)
	}

	c := repo.Entry{
		Name:     name,
		URL:      url,
		Username: username,
		Password: password,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
	}

	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if err := r.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", url)
	}

	f.Update(&c)

	return f.WriteFile(helmpath.RepositoryFile(), 0644)
}
