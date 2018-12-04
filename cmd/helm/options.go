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

package main // import "k8s.io/helm/cmd/helm"

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/strvals"
)

// -----------------------------------------------------------------------------
// Values Options

type valuesOptions struct {
	valueFiles   []string // --values
	values       []string // --set
	stringValues []string // --set-string
}

func (o *valuesOptions) addFlags(fs *pflag.FlagSet) {
	fs.StringSliceVarP(&o.valueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL(can specify multiple)")
	fs.StringArrayVar(&o.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	fs.StringArrayVar(&o.stringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
}

// mergeValues merges values from files specified via -f/--values and
// directly via --set or --set-string, marshaling them to YAML
func (o *valuesOptions) mergedValues() (map[string]interface{}, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range o.valueFiles {
		currentMap := map[string]interface{}{}

		bytes, err := readFile(filePath)
		if err != nil {
			return base, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return base, errors.Wrapf(err, "failed to parse %s", filePath)
		}
		// Merge with the previous map
		base = mergeValues(base, currentMap)
	}

	// User specified a value via --set
	for _, value := range o.values {
		if err := strvals.ParseInto(value, base); err != nil {
			return base, errors.Wrap(err, "failed parsing --set data")
		}
	}

	// User specified a value via --set-string
	for _, value := range o.stringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return base, errors.Wrap(err, "failed parsing --set-string data")
		}
	}

	return base, nil
}

// readFile load a file from stdin, the local directory, or a remote file with a url.
func readFile(filePath string) ([]byte, error) {
	if strings.TrimSpace(filePath) == "-" {
		return ioutil.ReadAll(os.Stdin)
	}
	u, _ := url.Parse(filePath)
	p := getter.All(settings)

	// FIXME: maybe someone handle other protocols like ftp.
	getterConstructor, err := p.ByScheme(u.Scheme)

	if err != nil {
		return ioutil.ReadFile(filePath)
	}

	getter, err := getterConstructor(filePath, "", "", "")
	if err != nil {
		return []byte{}, err
	}
	data, err := getter.Get(filePath)
	return data.Bytes(), err
}

// -----------------------------------------------------------------------------
// Chart Path Options

type chartPathOptions struct {
	caFile   string // --ca-file
	certFile string // --cert-file
	keyFile  string // --key-file
	keyring  string // --keyring
	password string // --password
	repoURL  string // --repo
	username string // --username
	verify   bool   // --verify
	version  string // --version
}

// defaultKeyring returns the expanded path to the default keyring.
func defaultKeyring() string {
	if v, ok := os.LookupEnv("GNUPGHOME"); ok {
		return filepath.Join(v, "pubring.gpg")
	}
	return os.ExpandEnv("$HOME/.gnupg/pubring.gpg")
}

func (o *chartPathOptions) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.version, "version", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	fs.BoolVar(&o.verify, "verify", false, "verify the package before installing it")
	fs.StringVar(&o.keyring, "keyring", defaultKeyring(), "location of public keys used for verification")
	fs.StringVar(&o.repoURL, "repo", "", "chart repository url where to locate the requested chart")
	fs.StringVar(&o.username, "username", "", "chart repository username where to locate the requested chart")
	fs.StringVar(&o.password, "password", "", "chart repository password where to locate the requested chart")
	fs.StringVar(&o.certFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	fs.StringVar(&o.keyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	fs.StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
}

func (o *chartPathOptions) locateChart(name string) (string, error) {
	return locateChartPath(o.repoURL, o.username, o.password, name, o.version, o.keyring, o.certFile, o.keyFile, o.caFile, o.verify)
}

// locateChartPath looks for a chart directory in known places, and returns either the full path or an error.
//
// This does not ensure that the chart is well-formed; only that the requested filename exists.
//
// Order of resolution:
// - relative to current working directory
// - if path is absolute or begins with '.', error out here
// - chart repos in $HELM_HOME
// - URL
//
// If 'verify' is true, this will attempt to also verify the chart.
func locateChartPath(repoURL, username, password, name, version, keyring,
	certFile, keyFile, caFile string, verify bool) (string, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)

	if _, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, err
		}
		if verify {
			if _, err := downloader.VerifyChart(abs, keyring); err != nil {
				return "", err
			}
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, errors.Errorf("path %q not found", name)
	}

	crepo := filepath.Join(settings.Home.Repository(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}

	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Out:      os.Stdout,
		Keyring:  keyring,
		Getters:  getter.All(settings),
		Username: username,
		Password: password,
	}
	if verify {
		dl.Verify = downloader.VerifyAlways
	}
	if repoURL != "" {
		chartURL, err := repo.FindChartInAuthRepoURL(repoURL, username, password, name, version,
			certFile, keyFile, caFile, getter.All(settings))
		if err != nil {
			return "", err
		}
		name = chartURL
	}

	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		os.MkdirAll(settings.Home.Archive(), 0744)
	}

	filename, _, err := dl.DownloadTo(name, version, settings.Home.Archive())
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		debug("Fetched %s to %s\n", name, filename)
		return lname, nil
	} else if settings.Debug {
		return filename, err
	}

	return filename, errors.Errorf("failed to download %q (hint: running `helm repo update` may help)", name)
}
