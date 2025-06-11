package action

import (
	"errors"
	"io"
	"testing"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
	"helm.sh/helm/v4/pkg/time"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	ErrNotReachable     = errors.New("kubernetes cluster is not reachable")
	ErrInvalidManifest  = errors.New("invalid manifest")
	ErrResourceNotFound = errors.New("resource not found")
)

type mockKubeClient struct {
	reachable bool
	buildErr  error
	getErr    error
	resources kube.ResourceList
	getResp   map[string][]interface{}
}

var _ kube.InterfaceResources = &mockKubeClient{}

func (m *mockKubeClient) IsReachable() error {
	if !m.reachable {
		return ErrNotReachable
	}
	return nil
}

func (m *mockKubeClient) Build(manifest io.Reader, validate bool) (kube.ResourceList, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return m.resources, nil
}

func (m *mockKubeClient) Get(resources kube.ResourceList, related bool) (map[string][]runtime.Object, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return map[string][]runtime.Object{"Service": {}}, nil
}

func (m *mockKubeClient) BuildTable(manifest io.Reader, validate bool) (kube.ResourceList, error) {
	return m.Build(manifest, validate)
}

func (m *mockKubeClient) Create(resources kube.ResourceList) (*kube.Result, error) {
	return &kube.Result{Created: resources}, nil
}

func (m *mockKubeClient) Delete(resources kube.ResourceList) (*kube.Result, []error) {
	return &kube.Result{Deleted: resources}, nil
}

func (m *mockKubeClient) Update(original, target kube.ResourceList, force bool) (*kube.Result, error) {
	return &kube.Result{Updated: target}, nil
}

func (m *mockKubeClient) GetWaiter(ws kube.WaitStrategy) (kube.Waiter, error) {
	return nil, nil
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name           string
		releaseName    string
		version        int
		reachable      bool
		buildErr       error
		getErr         error
		expectedErr    bool
		expectedStatus *release.Release
	}{
		{
			name:        "successful status check",
			releaseName: "test-release",
			version:     1,
			reachable:   true,
			expectedStatus: &release.Release{
				Name:      "test-release",
				Namespace: "default",
				Info: &release.Info{
					Status:        release.StatusDeployed,
					FirstDeployed: time.Now(),
					LastDeployed:  time.Now(),
				},
				Version:  1,
				Chart:    &chart.Chart{},
				Manifest: "apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service",
			},
		},
		{
			name:        "unreachable cluster",
			releaseName: "test-release",
			version:     1,
			reachable:   false,
			expectedErr: true,
		},
		{
			name:        "build error",
			releaseName: "test-release",
			version:     1,
			reachable:   true,
			buildErr:    ErrInvalidManifest,
			expectedErr: true,
		},
		{
			name:        "get error",
			releaseName: "test-release",
			version:     1,
			reachable:   true,
			getErr:      ErrResourceNotFound,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock storage driver
			store := storage.Init(driver.NewMemory())

			// Create a mock kube client
			mockClient := &mockKubeClient{
				reachable: tt.reachable,
				buildErr:  tt.buildErr,
				getErr:    tt.getErr,
			}

			// Create configuration
			cfg := &Configuration{
				Releases:     store,
				KubeClient:   mockClient,
				Capabilities: chartutil.DefaultCapabilities,
			}

			// Create status action
			status := NewStatus(cfg)
			status.Version = tt.version

			// Store test release if needed
			if tt.expectedStatus != nil {
				err := store.Create(tt.expectedStatus)
				if err != nil {
					t.Fatalf("Failed to create test release: %v", err)
				}
			}

			// Run status check
			got, err := status.Run(tt.releaseName)

			// Check error
			if tt.expectedErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify release
			if got == nil {
				t.Error("expected release but got nil")
				return
			}

			if got.Name != tt.expectedStatus.Name {
				t.Errorf("expected release name %q, got %q", tt.expectedStatus.Name, got.Name)
			}
		})
	}
}
