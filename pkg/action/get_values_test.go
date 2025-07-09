package action

import (
	"io"
	"testing"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

func TestNewGetValues(t *testing.T) {
	cfg := &Configuration{}
	gv := NewGetValues(cfg)
	if gv.cfg != cfg {
		t.Errorf("Expected cfg to be %v, got %v", cfg, gv.cfg)
	}
}

func TestGetValues_Run(t *testing.T) {
	tests := []struct {
		name      string
		release   *release.Release
		allValues bool
		version   int
		wantErr   bool
	}{
		{
			name: "get default values",
			release: &release.Release{
				Name:    "test-release",
				Version: 1,
				Config: map[string]interface{}{
					"key": "value",
				},
				Chart: &chart.Chart{
					Metadata: &chart.Metadata{Name: "test-chart", Version: "0.1.0"},
					Values: map[string]interface{}{
						"default": "value",
					},
				},
				Info: &release.Info{},
			},
			allValues: false,
			version:   1,
			wantErr:   false,
		},
		{
			name: "get all values",
			release: &release.Release{
				Name:    "test-release",
				Version: 1,
				Config: map[string]interface{}{
					"key": "value",
				},
				Chart: &chart.Chart{
					Metadata: &chart.Metadata{Name: "test-chart", Version: "0.1.0"},
					Values: map[string]interface{}{
						"default": "value",
					},
				},
				Info: &release.Info{},
			},
			allValues: true,
			version:   1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memDriver := driver.NewMemory()
			store := storage.Init(memDriver)
			// Insert the release into storage
			if err := store.Create(tt.release); err != nil {
				t.Fatalf("failed to insert release: %v", err)
			}

			cfg := &Configuration{
				KubeClient: &MockKubeClient{},
				Releases:   store,
			}
			gv := NewGetValues(cfg)
			gv.AllValues = tt.allValues
			gv.Version = tt.version

			got, err := gv.Run("test-release")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValues.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.allValues {
				expected, _ := chartutil.CoalesceValues(tt.release.Chart, tt.release.Config)
				if !compareMaps(got, expected) {
					t.Errorf("GetValues.Run() = %v, want %v", got, expected)
				}
			} else {
				if !compareMaps(got, tt.release.Config) {
					t.Errorf("GetValues.Run() = %v, want %v", got, tt.release.Config)
				}
			}
		})
	}
}

// MockKubeClient is a minimal mock implementation of the kube.Interface
// Only IsReachable is implemented for this test.
type MockKubeClient struct{}

func (m *MockKubeClient) IsReachable() error                               { return nil }
func (m *MockKubeClient) Create(kube.ResourceList) (*kube.Result, error)   { return nil, nil }
func (m *MockKubeClient) Delete(kube.ResourceList) (*kube.Result, []error) { return nil, nil }
func (m *MockKubeClient) Update(kube.ResourceList, kube.ResourceList, bool) (*kube.Result, error) {
	return nil, nil
}
func (m *MockKubeClient) Build(io.Reader, bool) (kube.ResourceList, error) { return nil, nil }
func (m *MockKubeClient) GetWaiter(kube.WaitStrategy) (kube.Waiter, error) { return nil, nil }

// Helper function to compare maps
func compareMaps(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
