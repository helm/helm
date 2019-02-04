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
	"io"

	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/helmpath"
)

// refUpdateOptions describes a 'helm library/dependency update'
type refUpdateOptions struct {
	keyring     string // --keyring
	skipRefresh bool   // --skip-refresh
	verify      bool   // --verify

	// args
	chartpath string

	helmhome helmpath.Home
}

// run runs the full library update process.
func (o *refUpdateOptions) run(out io.Writer, lib bool) error {
	man := &downloader.Manager{
		Out:        out,
		ChartPath:  o.chartpath,
		HelmHome:   o.helmhome,
		Keyring:    o.keyring,
		SkipUpdate: o.skipRefresh,
		Getters:    getter.All(settings),
	}
	if o.verify {
		man.Verify = downloader.VerifyAlways
	}
	if settings.Debug {
		man.Debug = true
	}
	return man.Update(lib)
}
