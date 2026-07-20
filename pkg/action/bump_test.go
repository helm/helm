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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestNewBump(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewBump(cfg)

	assert.NotNil(t, client)
	require.Equal(t, cfg, client.cfg)
}

func TestBump_Run_Major(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Major bump with valid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("major", "")
		require.NoError(t, err)
		require.Equal(t, "2.0.0", result)
	})

	t.Run("Major bump with invalid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("major", "")
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("Major bump with invalid major version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.a"
		result, err := client.Run("major", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Minor(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Minor bump with valid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("minor", "")
		require.NoError(t, err)
		require.Equal(t, "1.3.0", result)
	})

	t.Run("Minor bump with invalid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("minor", "")
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("Minor bump with invalid minor version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.a.3"
		result, err := client.Run("minor", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Patch(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Patch bump with valid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("patch", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.4", result)
	})

	t.Run("Patch bump with invalid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("patch", "")
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("Patch bump with invalid patch version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.a"
		result, err := client.Run("patch", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Stable(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Stable bump with pre-release version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-alpha"
		result, err := client.Run("stable", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3", result)
	})

	t.Run("Stable bump with pre-release version with number", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-alpha.1"
		result, err := client.Run("stable", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3", result)
	})

	t.Run("Stable bump with invalid version format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("stable", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Alpha(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Alpha bump with version without pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("alpha", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-alpha.1", result)
	})

	t.Run("Alpha bump with version with pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-alpha"
		result, err := client.Run("alpha", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-alpha.1", result)
	})

	t.Run("Alpha bump with version with pre-release number", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-alpha.1"
		result, err := client.Run("alpha", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-alpha.2", result)
	})

	t.Run("Alpha bump with invalid version format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("alpha", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Beta(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Beta bump with version without pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("beta", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-beta.1", result)
	})

	t.Run("Beta bump with version with pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-beta"
		result, err := client.Run("beta", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-beta.1", result)
	})

	t.Run("Beta bump with version with pre-release number", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-beta.1"
		result, err := client.Run("beta", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-beta.2", result)
	})

	t.Run("Beta bump with invalid version format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("beta", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_RC(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("RC bump with version without pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("rc", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-rc.1", result)
	})

	t.Run("RC bump with version with pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-rc"
		result, err := client.Run("rc", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-rc.1", result)
	})

	t.Run("RC bump with version with pre-release number", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-rc.1"
		result, err := client.Run("rc", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-rc.2", result)
	})

	t.Run("RC bump with invalid version format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("rc", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Post(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Post bump with version without pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("post", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-post.1", result)
	})

	t.Run("Post bump with version with post-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-post"
		result, err := client.Run("post", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-post.1", result)
	})

	t.Run("Post bump with version with pre-release number", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-post.1"
		result, err := client.Run("post", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-post.2", result)
	})

	t.Run("Post bump with invalid version format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("post", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_Dev(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Dev bump with version without pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("dev", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-dev.1", result)
	})

	t.Run("Dev bump with version with pre-release", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-dev"
		result, err := client.Run("dev", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-dev.1", result)
	})

	t.Run("Dev bump with version with pre-release number", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3-dev.1"
		result, err := client.Run("dev", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.3-dev.2", result)
	})

	t.Run("Dev bump with invalid version format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2"
		result, err := client.Run("dev", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_Run_ExplicitVersion(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	t.Run("Explicit version with valid format", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("2.0.1", "")
		require.NoError(t, err)
		require.Equal(t, "2.0.1", result)
	})

	t.Run("Explicit version with post version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("1.5.3-post.1", "")
		require.NoError(t, err)
		require.Equal(t, "1.5.3-post.1", result)
	})
}

func TestBump_Run_InvalidVersion(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	// We cannot test actual bumping since chart loading is not implemented,
	// but we can test error handling for invalid bump types
	t.Run("Invalid bump type", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("invalid", "")
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestBump_IsValidVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		version  string
		expected bool
	}{
		{"1.2.3", true},
		{"1.2.3-alpha", true},
		{"1.2.3-beta.1", true},
		{"1.2.3-rc.2", true},
		{"1.2.3-post.1", true},
		{"1.2.3-dev.1", true},
		{"1.2", false},
		{"invalid", false},
		{"1.2.3.4", false},
		{"1.a.3", false},
	}

	for _, tc := range testCases {
		t.Run("Version "+tc.version, func(t *testing.T) {
			result := isValidVersion(tc.version)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestBump_Run_WithDefault(t *testing.T) {
	t.Parallel()

	cfg := actionConfigFixture(t)
	client := NewBump(cfg)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
	}

	// Test that it properly handles the default case
	t.Run("Patch bump with valid version", func(t *testing.T) {
		client.chart.Metadata.Version = "1.2.3"
		result, err := client.Run("patch", "")
		require.NoError(t, err)
		require.Equal(t, "1.2.4", result)
		require.Equal(t, "patch", client.bump)
	})
}
