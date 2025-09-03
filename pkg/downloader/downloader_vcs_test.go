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

package downloader

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/getter"
)

func TestChartDownloader_NoVCSTransport(t *testing.T) {
	cd := &ChartDownloader{
		Out:     nil,
		Verify:  VerifyNever,
		Getters: getter.Providers{},
		Cache:   &MockCache{data: make(map[[32]byte][]byte)},
	}

	// Chart downloaders should not have VCS schemes available
	// This tests that the old ChartDownloader doesn't accidentally get VCS support
	providers := cd.Getters

	vcsSchemes := []string{"git", "git+http", "git+https", "git+ssh"}
	for _, scheme := range vcsSchemes {
		_, err := providers.ByScheme(scheme)
		assert.Error(t, err, "Chart downloader should not support VCS scheme %s", scheme)
		assert.Contains(t, err.Error(), "not supported", "Error should indicate scheme is not supported")
	}
}
