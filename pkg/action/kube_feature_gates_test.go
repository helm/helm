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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// fakeRESTClientGetter is a minimal RESTClientGetter that only supports
// ToRESTConfig, enough to exercise code paths that build a Kubernetes
// clientset directly against a local httptest server.
type fakeRESTClientGetter struct {
	restConfig *rest.Config
}

func (f *fakeRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return f.restConfig, nil
}

func (f *fakeRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return nil, errors.New("not implemented")
}

// poisonRESTClientGetter fails the test if any of its methods are called,
// used to prove a code path never contacts the cluster.
type poisonRESTClientGetter struct {
	t *testing.T
}

func (p *poisonRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	p.t.Fatal("ToRESTConfig should not be called")
	return nil, nil
}

func (p *poisonRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	p.t.Fatal("ToDiscoveryClient should not be called")
	return nil, nil
}

func (p *poisonRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	p.t.Fatal("ToRESTMapper should not be called")
	return nil, nil
}

const sampleFeatureGateMetrics = `# HELP kubernetes_feature_enabled [BETA] This metric records the data about the stage and enablement of a k8s feature.
# TYPE kubernetes_feature_enabled gauge
kubernetes_feature_enabled{name="ClusterTrustBundle",stage="BETA"} 0
kubernetes_feature_enabled{name="SidecarContainers",stage=""} 1
`

func TestParseFeatureGateMetrics(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]bool
		wantErr bool
	}{
		{
			name: "parses enabled and disabled gates",
			raw:  sampleFeatureGateMetrics,
			want: map[string]bool{
				"ClusterTrustBundle": false,
				"SidecarContainers":  true,
			},
		},
		{
			name: "metric family absent",
			raw:  "# HELP unrelated_metric a metric we don't care about\n# TYPE unrelated_metric gauge\nunrelated_metric 1\n",
			want: map[string]bool{},
		},
		{
			name:    "malformed input",
			raw:     "kubernetes_feature_enabled{name=\"X\" 1\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFeatureGateMetrics([]byte(tt.raw))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// newFeatureGateTestServer returns an httptest server whose /metrics handler
// serves sampleFeatureGateMetrics, standing in for kube-apiserver.
func newFeatureGateTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(sampleFeatureGateMetrics))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestCheckKubeFeatureGates(t *testing.T) {
	t.Run("no component declared is a no-op", func(t *testing.T) {
		cfg := actionConfigFixture(t)
		require.NoError(t, cfg.checkKubeFeatureGates(t.Context(), nil))
	})

	t.Run("unsupported component only warns, never blocks", func(t *testing.T) {
		cfg := actionConfigFixture(t)
		// actionConfigFixture never sets RESTClientGetter; if this component were
		// (incorrectly) treated as supported, fetching would fail loudly instead
		// of being skipped outright.
		err := cfg.checkKubeFeatureGates(t.Context(), map[string]map[string]bool{
			"scheduler": {"ComponentFlagz": false},
		})
		require.NoError(t, err)
	})

	t.Run("no cluster configured blocks with a clear error", func(t *testing.T) {
		cfg := actionConfigFixture(t)
		err := cfg.checkKubeFeatureGates(t.Context(), map[string]map[string]bool{
			"apiserver": {"SidecarContainers": true},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apiserver: could not verify feature gates")
	})

	server := newFeatureGateTestServer(t)
	cfg := actionConfigFixture(t)
	cfg.RESTClientGetter = &fakeRESTClientGetter{restConfig: &rest.Config{Host: server.URL}}

	t.Run("matching gate state passes", func(t *testing.T) {
		err := cfg.checkKubeFeatureGates(t.Context(), map[string]map[string]bool{
			"apiserver": {
				"ClusterTrustBundle": false,
				"SidecarContainers":  true,
			},
		})
		require.NoError(t, err)
	})

	t.Run("mismatching gate state blocks with a clear error", func(t *testing.T) {
		err := cfg.checkKubeFeatureGates(t.Context(), map[string]map[string]bool{
			"apiserver": {"ClusterTrustBundle": true},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apiserver.ClusterTrustBundle=true (cluster reports false)")
	})

	t.Run("gate absent from scraped output blocks with a clear error", func(t *testing.T) {
		// Covers the issue's primary scenario: an older cluster (or one
		// predating the gate) that doesn't report it at all must not be
		// treated as satisfying the requirement.
		err := cfg.checkKubeFeatureGates(t.Context(), map[string]map[string]bool{
			"apiserver": {"SomeGateThisServerDoesNotReport": true},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apiserver.SomeGateThisServerDoesNotReport: not reported by the cluster")
	})
}

func TestCheckKubeFeatureGates_ClusterPredatesFeatureGateMetric(t *testing.T) {
	// Simulates a Kubernetes version older than 1.26, which doesn't expose
	// kubernetes_feature_enabled at all -- the issue's primary scenario of a
	// lower K8s version must not be treated as satisfying the requirement.
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("# HELP unrelated_metric a metric with nothing to do with feature gates\n# TYPE unrelated_metric gauge\nunrelated_metric 1\n"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := actionConfigFixture(t)
	cfg.RESTClientGetter = &fakeRESTClientGetter{restConfig: &rest.Config{Host: server.URL}}

	err := cfg.checkKubeFeatureGates(t.Context(), map[string]map[string]bool{
		"apiserver": {"SidecarContainers": true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiserver.SidecarContainers: not reported by the cluster")
}

func TestMergedKubeFeatureGates(t *testing.T) {
	t.Run("no dependencies returns the root chart's own requirements", func(t *testing.T) {
		ch := buildChart(withKubeFeatureGates(map[string]map[string]bool{
			"apiserver": {"SidecarContainers": true},
		}))
		merged, err := mergedKubeFeatureGates(ch)
		require.NoError(t, err)
		assert.Equal(t, map[string]map[string]bool{"apiserver": {"SidecarContainers": true}}, merged)
	})

	t.Run("combines root and dependency requirements for different gates", func(t *testing.T) {
		ch := buildChart(
			withKubeFeatureGates(map[string]map[string]bool{"apiserver": {"SidecarContainers": true}}),
			withDependency(withName("sub"), withKubeFeatureGates(map[string]map[string]bool{"kubelet": {"NodeSwap": false}})),
		)
		merged, err := mergedKubeFeatureGates(ch)
		require.NoError(t, err)
		assert.Equal(t, map[string]map[string]bool{
			"apiserver": {"SidecarContainers": true},
			"kubelet":   {"NodeSwap": false},
		}, merged)
	})

	t.Run("agreeing requirements for the same gate from multiple levels are fine", func(t *testing.T) {
		ch := buildChart(
			withKubeFeatureGates(map[string]map[string]bool{"apiserver": {"SidecarContainers": true}}),
			withDependency(withName("sub"), withKubeFeatureGates(map[string]map[string]bool{"apiserver": {"SidecarContainers": true}})),
		)
		merged, err := mergedKubeFeatureGates(ch)
		require.NoError(t, err)
		assert.Equal(t, map[string]map[string]bool{"apiserver": {"SidecarContainers": true}}, merged)
	})

	t.Run("conflicting requirements for the same gate error out", func(t *testing.T) {
		ch := buildChart(
			withName("root"),
			withKubeFeatureGates(map[string]map[string]bool{"apiserver": {"SidecarContainers": true}}),
			withDependency(withName("sub"), withKubeFeatureGates(map[string]map[string]bool{"apiserver": {"SidecarContainers": false}})),
		)
		_, err := mergedKubeFeatureGates(ch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apiserver.SidecarContainers")
	})
}
