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
	"io"
	"path/filepath"
	"strings"

	"helm.sh/helm/internal/experimental/registry"
	"helm.sh/helm/pkg/helmpath"

	"helm.sh/helm/internal/experimental/tuf"
)

// ChartSign performs a chart sign operation
type ChartSign struct {
	cfg         *Configuration
	trustDir    string
	trustServer string
	ref         string
	tlscacert   string
	rootKey     string
}

// NewChartSign creates a new ChartSign object with given configuration and trust info
func NewChartSign(cfg *Configuration, trustDir, trustServer, ref, tlscacert, rootKey string) *ChartSign {
	return &ChartSign{
		cfg:         cfg,
		trustDir:    trustDir,
		trustServer: trustServer,
		ref:         ref,
		tlscacert:   tlscacert,
		rootKey:     rootKey,
	}
}

// Run executes the chart sign and push operation
func (a *ChartSign) Run(out io.Writer, ref string) error {

	c, err := registry.NewCache(
		registry.CacheOptWriter(out),
		registry.CacheOptRoot(filepath.Join(helmpath.Registry(), registry.CacheRootDir)))

	r, err := registry.ParseReference(ref)
	if err != nil {
		return err
	}

	cs, err := c.FetchReference(r)
	if err != nil {
		return err
	}

	file := filepath.Join(helmpath.Registry(), registry.CacheRootDir, "blobs", "sha256", strings.Split(cs.Digest.String(), ":")[1])
	_, err = tuf.SignAndPublish(a.trustDir, a.trustServer, ref, file, a.tlscacert, a.rootKey, nil)
	return err
}
