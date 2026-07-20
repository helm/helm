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
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/chart/v2/util"
)

// Bump is the action for bumping a chart version.
//
// It provides the implementation of 'helm bump'.
type Bump struct {
	ChartPathOptions
	cfg *Configuration

	bump  string
	chart *chart.Chart
}

// NewBump creates a new Bump object with the given configuration.
func NewBump(cfg *Configuration) *Bump {
	return &Bump{
		cfg: cfg,
	}
}

// Run executes 'helm bump' against the given chart.
func (b *Bump) Run(bumpType string, chartpath string) (string, error) {
	if b.chart == nil {
		chrt, err := loader.Load(chartpath)
		if err != nil {
			return "", err
		}
		b.chart = chrt
	}
	cv, err := yaml.Marshal(b.chart.Metadata.Version)
	if err != nil {
		return "", err
	}
	if bumpType == "" {
		bumpType = "patch"
	}
	b.bump = bumpType

	currentVersion := strings.TrimSpace(string(cv))
	parsedVersion, err := semver.StrictNewVersion(currentVersion)
	if err != nil {
		return "", fmt.Errorf("invalid original version: %s", currentVersion)
	}

	var newVersion semver.Version

	version, err := semver.StrictNewVersion(b.bump)
	if err != nil {
		switch b.bump {
		case "major":
			newVersion = parsedVersion.IncMajor()
		case "minor":
			newVersion = parsedVersion.IncMinor()
		case "patch":
			newVersion = parsedVersion.IncPatch()
		case "stable":
			newVersion, _ = parsedVersion.SetPrerelease("")
		default:
			preRelease := parsedVersion.Prerelease()

			var preReleaseVersion int

			parts := strings.Split(preRelease, ".")

			if len(parts) == 1 {
				preReleaseVersion = 1
			} else {
				preReleaseVersion, _ = strconv.Atoi(parts[1])
				preReleaseVersion++
			}
			newPreReleaseString := fmt.Sprintf("%s.%d", bumpType, preReleaseVersion)
			newVersion, _ = parsedVersion.SetPrerelease(newPreReleaseString)
		}
	} else {
		newVersion = *version
	}

	b.chart.Metadata.Version = newVersion.String()

	// Save the updated chart to disk (this will update Chart.yaml)
	if chartpath != "" {
		err = util.SaveChartfile(filepath.Join(chartpath, "Chart.yaml"), b.chart.Metadata)
		if err != nil {
			return "", fmt.Errorf("failed to save updated chart: %w", err)
		}
	}

	return b.chart.Metadata.Version, nil
}
