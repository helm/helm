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

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
)

const fetchDesc = `
Retrieve a package from a package repository, and download it locally.

This is useful for fetching packages to inspect, modify, or repackage. It can
also be used to perform cryptographic verification of a chart without installing
the chart.

There are options for unpacking the chart after download. This will create a
directory for the chart and uncomparess into that directory.

If the --verify flag is specified, the requested chart MUST have a provenance
file, and MUST pass the verification process. Failure in any part of this will
result in an error, and the chart will not be saved locally.
`

type fetchCmd struct {
	untar    bool
	untardir string
	chartRef string

	verify  bool
	keyring string

	out io.Writer
}

func newFetchCmd(out io.Writer) *cobra.Command {
	fch := &fetchCmd{out: out}

	cmd := &cobra.Command{
		Use:   "fetch [flags] [chart URL | repo/chartname] [...]",
		Short: "download a chart from a repository and (optionally) unpack it in local directory",
		Long:  fetchDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("This command needs at least one argument, url or repo/name of the chart.")
			}
			for i := 0; i < len(args); i++ {
				fch.chartRef = args[i]
				if err := fch.run(); err != nil {
					return err
				}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&fch.untar, "untar", false, "If set to true, will untar the chart after downloading it.")
	f.StringVar(&fch.untardir, "untardir", ".", "If untar is specified, this flag specifies where to untar the chart.")
	f.BoolVar(&fch.verify, "verify", false, "Verify the package against its signature.")
	f.StringVar(&fch.keyring, "keyring", defaultKeyring(), "The keyring containing public keys.")

	return cmd
}

func (f *fetchCmd) run() error {
	pname := f.chartRef
	if filepath.Ext(pname) != ".tgz" {
		pname += ".tgz"
	}

	return downloadAndSaveChart(pname, f.untar, f.untardir, f.verify, f.keyring)
}

// downloadAndSaveChart fetches a chart over HTTP, and then (if verify is true) verifies it.
//
// If untar is true, it also unpacks the file into untardir.
func downloadAndSaveChart(pname string, untar bool, untardir string, verify bool, keyring string) error {
	buf, err := downloadChart(pname, verify, keyring)
	if err != nil {
		return err
	}
	return saveChart(pname, buf, untar, untardir)
}

func downloadChart(pname string, verify bool, keyring string) (*bytes.Buffer, error) {
	r, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return bytes.NewBuffer(nil), err
	}

	// get download url
	u, err := mapRepoArg(pname, r.Repositories)
	if err != nil {
		return bytes.NewBuffer(nil), err
	}

	href := u.String()
	buf, err := fetchChart(href)
	if err != nil {
		return buf, err
	}

	if verify {
		basename := filepath.Base(pname)
		sigref := href + ".prov"
		sig, err := fetchChart(sigref)
		if err != nil {
			return buf, fmt.Errorf("provenance data not downloaded from %s: %s", sigref, err)
		}
		if err := ioutil.WriteFile(basename+".prov", sig.Bytes(), 0755); err != nil {
			return buf, fmt.Errorf("provenance data not saved: %s", err)
		}
		if err := verifyChart(basename, keyring); err != nil {
			return buf, err
		}
	}

	return buf, nil
}

// verifyChart takes a path to a chart archive and a keyring, and verifies the chart.
//
// It assumes that a chart archive file is accompanied by a provenance file whose
// name is the archive file name plus the ".prov" extension.
func verifyChart(path string, keyring string) error {
	// For now, error out if it's not a tar file.
	if fi, err := os.Stat(path); err != nil {
		return err
	} else if fi.IsDir() {
		return errors.New("unpacked charts cannot be verified")
	} else if !isTar(path) {
		return errors.New("chart must be a tgz file")
	}

	provfile := path + ".prov"
	if _, err := os.Stat(provfile); err != nil {
		return fmt.Errorf("could not load provenance file %s: %s", provfile, err)
	}

	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return fmt.Errorf("failed to load keyring: %s", err)
	}
	ver, err := sig.Verify(path, provfile)
	if flagDebug {
		for name := range ver.SignedBy.Identities {
			fmt.Printf("Signed by %q\n", name)
		}
	}
	return err
}

// defaultKeyring returns the expanded path to the default keyring.
func defaultKeyring() string {
	return os.ExpandEnv("$HOME/.gnupg/pubring.gpg")
}

// isTar tests whether the given file is a tar file.
//
// Currently, this simply checks extension, since a subsequent function will
// untar the file and validate its binary format.
func isTar(filename string) bool {
	return strings.ToLower(filepath.Ext(filename)) == ".tgz"
}

// saveChart saves a chart locally.
func saveChart(name string, buf *bytes.Buffer, untar bool, untardir string) error {
	if untar {
		return chartutil.Expand(untardir, buf)
	}

	p := strings.Split(name, "/")
	return saveChartFile(p[len(p)-1], buf)
}

// fetchChart retrieves a chart over HTTP.
func fetchChart(href string) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)

	resp, err := http.Get(href)
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

// mapRepoArg figures out which format the argument is given, and creates a fetchable
// url from it.
func mapRepoArg(arg string, r map[string]string) (*url.URL, error) {
	// See if it's already a full URL.
	u, err := url.ParseRequestURI(arg)
	if err == nil {
		// If it has a scheme and host and path, it's a full URL
		if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
			return u, nil
		}
		return nil, fmt.Errorf("Invalid chart url format: %s", arg)
	}
	// See if it's of the form: repo/path_to_chart
	p := strings.Split(arg, "/")
	if len(p) > 1 {
		if baseURL, ok := r[p[0]]; ok {
			if !strings.HasSuffix(baseURL, "/") {
				baseURL = baseURL + "/"
			}
			return url.ParseRequestURI(baseURL + strings.Join(p[1:], "/"))
		}
		return nil, fmt.Errorf("No such repo: %s", p[0])
	}
	return nil, fmt.Errorf("Invalid chart url format: %s", arg)
}

func saveChartFile(c string, r io.Reader) error {
	// Grab the chart name that we'll use for the name of the file to download to.
	out, err := os.Create(c)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	return err
}
