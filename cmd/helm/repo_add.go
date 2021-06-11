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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// Repositories that have been permanently deleted and no longer work
var deprecatedRepos = map[string]string{
	"//kubernetes-charts.storage.googleapis.com":           "https://charts.helm.sh/stable",
	"//kubernetes-charts-incubator.storage.googleapis.com": "https://charts.helm.sh/incubator",
}

type repoAddOptions struct {
	name                 string
	url                  string
	username             string
	password             string
	passCredentialsAll   bool
	forceUpdate          bool
	allowDeprecatedRepos bool

	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSverify bool

	repoFile  string
	repoCache string

	// Deprecated, but cannot be removed until Helm 4
	deprecatedNoUpdate bool
}

func newRepoAddCmd(out io.Writer) *cobra.Command {
	o := &repoAddOptions{}

	cmd := &cobra.Command{
		Use:               "add [NAME] [URL]",
		Short:             "add a chart repository",
		Args:              require.ExactArgs(2),
		ValidArgsFunction: noCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.name = args[0]
			o.url = args[1]
			o.repoFile = settings.RepositoryConfig
			o.repoCache = settings.RepositoryCache

			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.username, "username", "", "chart repository username")
	f.StringVar(&o.password, "password", "", "chart repository password")
	f.BoolVar(&o.forceUpdate, "force-update", false, "replace (overwrite) the repo if it already exists")
	f.BoolVar(&o.deprecatedNoUpdate, "no-update", false, "Ignored. Formerly, it would disabled forced updates. It is deprecated by force-update.")
	f.StringVar(&o.certFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&o.keyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	f.BoolVar(&o.insecureSkipTLSverify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the repository")
	f.BoolVar(&o.allowDeprecatedRepos, "allow-deprecated-repos", false, "by default, this command will not allow adding official repos that have been permanently deleted. This disables that behavior")
	f.BoolVar(&o.passCredentialsAll, "pass-credentials", false, "pass credentials to all domains")

	return cmd
}

func (o *repoAddOptions) run(out io.Writer) error {
	// Block deprecated repos
	if !o.allowDeprecatedRepos {
		for oldURL, newURL := range deprecatedRepos {
			if strings.Contains(o.url, oldURL) {
				return fmt.Errorf("repo %q is no longer available; try %q instead", o.url, newURL)
			}
		}
	}

	// Ensure the file directory exists as it is required for file locking
	err := os.MkdirAll(filepath.Dir(o.repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Acquire a file lock for process synchronization
	fileLock := flock.New(strings.Replace(o.repoFile, filepath.Ext(o.repoFile), ".lock", 1))
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(o.repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	if o.username != "" && o.password == "" {
		fd := int(os.Stdin.Fd())
		fmt.Fprint(out, "Password: ")
		password, err := term.ReadPassword(fd)
		fmt.Fprintln(out)
		if err != nil {
			return err
		}
		o.password = string(password)
	}

	c := repo.Entry{
		Name:                  o.name,
		URL:                   o.url,
		Username:              o.username,
		Password:              o.password,
		PassCredentialsAll:    o.passCredentialsAll,
		CertFile:              o.certFile,
		KeyFile:               o.keyFile,
		CAFile:                o.caFile,
		InsecureSkipTLSverify: o.insecureSkipTLSverify,
	}

	// If the repo exists do one of two things:
	// 1. If the configuration for the name is the same continue without error
	// 2. When the config is different require --force-update
	if !o.forceUpdate && f.Has(o.name) {
		existing := f.Get(o.name)
		if c != *existing {

			// The input coming in for the name is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository name (%s) already exists, please specify a different name", o.name)
		}

		// The add is idempotent so do nothing
		fmt.Fprintf(out, "%q already exists with the same configuration, skipping\n", o.name)
		return nil
	}

	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if o.repoCache != "" {
		r.CachePath = o.repoCache
	}
	if _, err := r.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", o.url)
	}

	f.Update(&c)

	if err := f.WriteFile(o.repoFile, 0644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%q has been added to your repositories\n", o.name)
	return nil
}
