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

package action

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
	"golang.org/x/term"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// Repositories that have been permanently deleted and no longer work
var deprecatedRepos = map[string]string{
	"//kubernetes-charts.storage.googleapis.com":           "https://charts.helm.sh/stable",
	"//kubernetes-charts-incubator.storage.googleapis.com": "https://charts.helm.sh/incubator",
}

type RepoAddOptions struct {
	Name                 string
	URL                  string
	Username             string
	Password             string
	PasswordFromStdinOpt bool
	PassCredentialsAll   bool
	ForceUpdate          bool
	AllowDeprecatedRepos bool

	CertFile              string
	KeyFile               string
	CaFile                string
	InsecureSkipTLSverify bool

	RepoFile  string
	RepoCache string

	// Deprecated, but cannot be removed until Helm 4
	DeprecatedNoUpdate bool
}

func (o *RepoAddOptions) Run(settings *cli.EnvSettings, out io.Writer) error {
	// Block deprecated repos
	if !o.AllowDeprecatedRepos {
		for oldURL, newURL := range deprecatedRepos {
			if strings.Contains(o.URL, oldURL) {
				return fmt.Errorf("repo %q is no longer available; try %q instead", o.URL, newURL)
			}
		}
	}

	// Ensure the file directory exists as it is required for file locking
	err := os.MkdirAll(filepath.Dir(o.RepoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Acquire a file lock for process synchronization
	repoFileExt := filepath.Ext(o.RepoFile)
	var lockPath string
	if len(repoFileExt) > 0 && len(repoFileExt) < len(o.RepoFile) {
		lockPath = strings.TrimSuffix(o.RepoFile, repoFileExt) + ".lock"
	} else {
		lockPath = o.RepoFile + ".lock"
	}
	fileLock := flock.New(lockPath)
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(o.RepoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	if o.Username != "" && o.Password == "" {
		if o.PasswordFromStdinOpt {
			passwordFromStdin, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			password := strings.TrimSuffix(string(passwordFromStdin), "\n")
			password = strings.TrimSuffix(password, "\r")
			o.Password = password
		} else {
			fd := int(os.Stdin.Fd())
			fmt.Fprint(out, "Password: ")
			password, err := term.ReadPassword(fd)
			fmt.Fprintln(out)
			if err != nil {
				return err
			}
			o.Password = string(password)
		}
	}

	c := repo.Entry{
		Name:                  o.Name,
		URL:                   o.URL,
		Username:              o.Username,
		Password:              o.Password,
		PassCredentialsAll:    o.PassCredentialsAll,
		CertFile:              o.CertFile,
		KeyFile:               o.KeyFile,
		CAFile:                o.CaFile,
		InsecureSkipTLSverify: o.InsecureSkipTLSverify,
	}

	// Check if the repo name is legal
	if strings.Contains(o.Name, "/") {
		return errors.Errorf("repository name (%s) contains '/', please specify a different name without '/'", o.Name)
	}

	// If the repo exists do one of two things:
	// 1. If the configuration for the name is the same continue without error
	// 2. When the config is different require --force-update
	if !o.ForceUpdate && f.Has(o.Name) {
		existing := f.Get(o.Name)
		if c != *existing {

			// The input coming in for the name is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository name (%s) already exists, please specify a different name", o.Name)
		}

		// The add is idempotent so do nothing
		fmt.Fprintf(out, "%q already exists with the same configuration, skipping\n", o.Name)
		return nil
	}

	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if o.RepoCache != "" {
		r.CachePath = o.RepoCache
	}
	if _, err := r.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", o.URL)
	}

	f.Update(&c)

	if err := f.WriteFile(o.RepoFile, 0644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%q has been added to your repositories\n", o.Name)
	return nil
}
