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

package resolver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/registry"
)

// fakeOCIRegistry is a minimal OCI registry stub that serves the tag list
// endpoint (GET /v2/<repository>/tags/list) with the given tags.
type fakeOCIRegistry struct {
	tags map[string][]string
}

func newFakeOCIRegistry(tags map[string][]string) *fakeOCIRegistry {
	return &fakeOCIRegistry{tags: tags}
}

func (f *fakeOCIRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	// Only the tag list endpoint is needed by the resolver.
	if strings.HasSuffix(path, "/tags/list") {
		repo := strings.TrimPrefix(path, "/v2/")
		repo = strings.TrimSuffix(repo, "/tags/list")
		tags := f.tags[repo]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name": repo,
			"tags": tags,
		})
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

// TestResolveOCIConstraintNoMatch reports an error when no OCI tag satisfies
// the version constraint, instead of silently writing the raw constraint
// string into the lock file.
func TestResolveOCIConstraintNoMatch(t *testing.T) {
	server := httptest.NewServer(newFakeOCIRegistry(map[string][]string{
		"my-registry/my-subchart": {"1.1.9"},
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	registryClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)

	r := New("testdata/chartpath", "testdata/repository", registryClient)

	reqs := []*chart.Dependency{
		{Name: "my-subchart", Repository: fmt.Sprintf("oci://%s/my-registry", host), Version: ">=1.1.10"},
	}

	_, err = r.Resolve(reqs, map[string]string{"my-subchart": "my-registry"})
	require.Error(t, err, "expected an error when no OCI tag satisfies the constraint")
	require.Contains(t, err.Error(), "can't get a valid version")
}

// TestResolveOCIConstraintMatch still resolves when a tag satisfies the
// constraint (guard against over-eager found=false).
func TestResolveOCIConstraintMatch(t *testing.T) {
	server := httptest.NewServer(newFakeOCIRegistry(map[string][]string{
		"my-registry/my-subchart": {"1.1.9", "1.1.17"},
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	registryClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)

	r := New("testdata/chartpath", "testdata/repository", registryClient)

	reqs := []*chart.Dependency{
		{Name: "my-subchart", Repository: fmt.Sprintf("oci://%s/my-registry", host), Version: ">=1.1.10"},
	}

	lock, err := r.Resolve(reqs, map[string]string{"my-subchart": "my-registry"})
	require.NoError(t, err)
	require.Len(t, lock.Dependencies, 1)
	require.Equal(t, "1.1.17", lock.Dependencies[0].Version)
}

// TestResolveOCIExplicitVersion pins an explicit (non-range) version without
// hitting the registry: the found=false change must not break the explicit
// version short-circuit.
func TestResolveOCIExplicitVersion(t *testing.T) {
	server := httptest.NewServer(newFakeOCIRegistry(map[string][]string{
		"my-registry/my-subchart": {"1.1.9", "1.1.17"},
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	registryClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)

	r := New("testdata/chartpath", "testdata/repository", registryClient)

	reqs := []*chart.Dependency{
		{Name: "my-subchart", Repository: fmt.Sprintf("oci://%s/my-registry", host), Version: "1.1.17"},
	}

	lock, err := r.Resolve(reqs, map[string]string{"my-subchart": "my-registry"})
	require.NoError(t, err)
	require.Len(t, lock.Dependencies, 1)
	require.Equal(t, "1.1.17", lock.Dependencies[0].Version)
}
