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

package downloader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
)

// VerificationStrategy describes a strategy for determining whether to verify a chart.
type VerificationStrategy int

const (
	// VerifyNever will skip all verification of a chart.
	VerifyNever VerificationStrategy = iota
	// VerifyIfPossible will attempt a verification, it will not error if verification
	// data is missing. But it will not stop processing if verification fails.
	VerifyIfPossible
	// VerifyAlways will always attempt a verification, and will fail if the
	// verification fails.
	VerifyAlways
	// VerifyLater will fetch verification data, but not do any verification.
	// This is to accommodate the case where another step of the process will
	// perform verification.
	VerifyLater
)

// ChartDownloader handles downloading a chart.
//
// It is capable of performing verifications on charts as well.
type ChartDownloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// HelmHome is the $HELM_HOME.
	HelmHome helmpath.Home
}

// DownloadTo retrieves a chart. Depending on the settings, it may also download a provenance file.
//
// If Verify is set to VerifyNever, the verification will be nil.
// If Verify is set to VerifyIfPossible, this will return a verification (or nil on failure), and print a warning on failure.
// If Verify is set to VerifyAlways, this will return a verification or an error if the verification fails.
// If Verify is set to VerifyLater, this will download the prov file (if it exists), but not verify it.
//
// For VerifyNever and VerifyIfPossible, the Verification may be empty.
//
// Returns a string path to the location where the file was downloaded and a verification
// (if provenance was verified), or an error if something bad happened.
func (c *ChartDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	u, r, err := c.ResolveChartVersion(ref, version)
	if err != nil {
		return "", nil, err
	}

	data, err := download(u.String(), r)
	if err != nil {
		return "", nil, err
	}

	name := filepath.Base(u.Path)
	destfile := filepath.Join(dest, name)
	if err := ioutil.WriteFile(destfile, data.Bytes(), 0655); err != nil {
		return destfile, nil, err
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if c.Verify > VerifyNever {
		body, err := download(u.String()+".prov", r)
		if err != nil {
			if c.Verify == VerifyAlways {
				return destfile, ver, fmt.Errorf("Failed to fetch provenance %q", u.String()+".prov")
			}
			fmt.Fprintf(c.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
			return destfile, ver, nil
		}
		provfile := destfile + ".prov"
		if err := ioutil.WriteFile(provfile, body.Bytes(), 0655); err != nil {
			return destfile, nil, err
		}

		if c.Verify != VerifyLater {
			ver, err = VerifyChart(destfile, c.Keyring)
			if err != nil {
				// Fail always in this case, since it means the verification step
				// failed.
				return destfile, ver, err
			}
		}
	}
	return destfile, ver, nil
}

// ResolveChartVersion resolves a chart reference to a URL.
//
// A reference may be an HTTP URL, a 'reponame/chartname' reference, or a local path.
//
// A version is a SemVer string (1.2.3-beta.1+f334a6789).
//
// 	- For fully qualified URLs, the version will be ignored (since URLs aren't versioned)
//	- For a chart reference
//		* If version is non-empty, this will return the URL for that version
//		* If version is empty, this will return the URL for the latest version
// 		* If no version can be found, an error is returned
func (c *ChartDownloader) ResolveChartVersion(ref, version string) (*url.URL, *repo.ChartRepository, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid chart URL format: %s", ref)
	}

	rf, err := repo.LoadRepositoriesFile(c.HelmHome.RepositoryFile())
	if err != nil {
		return nil, nil, err
	}

	var (
		chartName string
		rc        *repo.Entry
	)
	if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
		// If it has a scheme and host and path, it's a full URL
		p := strings.SplitN(strings.TrimLeft(u.Path, "/"), "-", 2)
		if len(p) < 2 {
			return nil, nil, fmt.Errorf("Seems that chart path is not in form of repo_url/path_to_chart, got: %s", u)
		}
		chartName = p[0]
		u.Path = ""
		rc, err = pickChartRepositoryConfigByURL(u.String(), rf.Repositories)
		if err != nil {
			return nil, nil, err
		}
	} else {
		// See if it's of the form: repo/path_to_chart
		p := strings.SplitN(u.Path, "/", 2)
		if len(p) < 2 {
			return nil, nil, fmt.Errorf("Non-absolute URLs should be in form of repo_name/path_to_chart, got: %s", u)
		}

		repoName := p[0]
		chartName = p[1]
		rc, err = pickChartRepositoryConfigByName(repoName, rf.Repositories)
		if err != nil {
			return nil, nil, err
		}
	}

	r, err := repo.NewChartRepository(rc)
	if err != nil {
		return nil, nil, err
	}

	// Next, we need to load the index, and actually look up the chart.
	i, err := repo.LoadIndexFile(c.HelmHome.CacheIndex(r.Config.Name))
	if err != nil {
		return nil, nil, fmt.Errorf("no cached repo found. (try 'helm repo update'). %s", err)
	}

	cv, err := i.Get(chartName, version)
	if err != nil {
		return nil, nil, fmt.Errorf("chart %q not found in %s index. (try 'helm repo update'). %s", chartName, r.Config.Name, err)
	}

	if len(cv.URLs) == 0 {
		return nil, nil, fmt.Errorf("chart %q has no downloadable URLs", ref)
	}

	// TODO: Seems that picking first URL is not fully correct
	u, err = url.Parse(cv.URLs[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid chart URL format: %s", ref)
	}

	return u, r, nil
}

// VerifyChart takes a path to a chart archive and a keyring, and verifies the chart.
//
// It assumes that a chart archive file is accompanied by a provenance file whose
// name is the archive file name plus the ".prov" extension.
func VerifyChart(path string, keyring string) (*provenance.Verification, error) {
	// For now, error out if it's not a tar file.
	if fi, err := os.Stat(path); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("unpacked charts cannot be verified")
	} else if !isTar(path) {
		return nil, errors.New("chart must be a tgz file")
	}

	provfile := path + ".prov"
	if _, err := os.Stat(provfile); err != nil {
		return nil, fmt.Errorf("could not load provenance file %s: %s", provfile, err)
	}

	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load keyring: %s", err)
	}
	return sig.Verify(path, provfile)
}

// download performs a Get from repo.Getter and returns the body.
func download(href string, r repo.Getter) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)

	resp, err := r.Get(href)
	if err != nil {
		return buf, err
	}
	if resp.StatusCode != 200 {
		return buf, fmt.Errorf("Failed to fetch %s : %s", href, resp.Status)
	}

	_, err = io.Copy(buf, resp.Body)
	resp.Body.Close()
	return buf, err
}

// isTar tests whether the given file is a tar file.
//
// Currently, this simply checks extension, since a subsequent function will
// untar the file and validate its binary format.
func isTar(filename string) bool {
	return strings.ToLower(filepath.Ext(filename)) == ".tgz"
}

func pickChartRepositoryConfigByName(name string, cfgs []*repo.Entry) (*repo.Entry, error) {
	for _, rc := range cfgs {
		if rc.Name == name {
			if rc.URL == "" {
				return nil, fmt.Errorf("no URL found for repository %s", name)
			}
			return rc, nil
		}
	}
	return nil, fmt.Errorf("repo %s not found", name)
}

func pickChartRepositoryConfigByURL(u string, cfgs []*repo.Entry) (*repo.Entry, error) {
	for _, rc := range cfgs {
		if rc.URL == u {
			return rc, nil
		}
	}
	return nil, fmt.Errorf("repo with URL %s not found", u)
}
