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
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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

const defaultBumpType = "patch"

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
	// Determine new version based on bump type or explicit version
	b.bump = bumpType
	if b.bump == "" {
		// Default to "patch" if no version specified
		b.bump = defaultBumpType
	}

	currentVersion := strings.TrimSpace(string(cv))
	if !isValidVersion(currentVersion) {
		return "", fmt.Errorf("invalid original version: %s", currentVersion)
	}

	var newVersion string
	switch b.bump {
	case "major":
		newVersion, err = bumpMajor(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump major version: %w", err)
		}
	case "minor":
		newVersion, err = bumpMinor(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump minor version: %w", err)
		}
	case "patch":
		newVersion, err = bumpPatch(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump patch version: %w", err)
		}
	case "stable":
		newVersion, err = bumpStable(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump stable version: %w", err)
		}
	case "alpha":
		newVersion, err = bumpAlpha(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump alpha version: %w", err)
		}
	case "beta":
		newVersion, err = bumpBeta(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump beta version: %w", err)
		}
	case "rc":
		newVersion, err = bumpRC(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump rc version: %w", err)
		}
	case "post":
		newVersion, err = bumpPost(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump post version: %w", err)
		}
	case "dev":
		newVersion, err = bumpDev(currentVersion)
		if err != nil {
			return "", fmt.Errorf("failed to bump dev version: %w", err)
		}
	default:
		if !isValidVersion(b.bump) {
			return "", fmt.Errorf("invalid bump type or version: %s", b.bump)
		}
		newVersion = b.bump
	}

	// Update the chart metadata with the new version
	b.chart.Metadata.Version = newVersion

	// Save the updated chart to disk (this will update Chart.yaml)
	if chartpath != "" {
		err = util.SaveChartfile(filepath.Join(chartpath, "Chart.yaml"), b.chart.Metadata)
		if err != nil {
			return "", fmt.Errorf("failed to save updated chart: %w", err)
		}
	}

	return newVersion, nil
}

// bumpMajor increases the major version number (e.g., 1.2.3 -> 2.0.0)
func bumpMajor(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return "", errors.New("invalid version format for major bump")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", errors.New("invalid major version number")
	}

	newMajor := major + 1
	return fmt.Sprintf("%d.0.0", newMajor), nil
}

// bumpMinor increases the minor version number (e.g., 1.2.3 -> 1.3.0)
func bumpMinor(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return "", errors.New("invalid version format for minor bump")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", errors.New("invalid major version number")
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", errors.New("invalid minor version number")
	}

	newMinor := minor + 1
	return fmt.Sprintf("%d.%d.0", major, newMinor), nil
}

// bumpPatch increases the patch version number (e.g., 1.2.3 -> 1.2.4)
func bumpPatch(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return "", errors.New("invalid version format for patch bump")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", errors.New("invalid major version number")
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", errors.New("invalid minor version number")
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", errors.New("invalid patch version number")
	}

	newPatch := patch + 1
	return fmt.Sprintf("%d.%d.%d", major, minor, newPatch), nil
}

// bumpStable removes any pre-release suffix (e.g., 1.2.3-alpha -> 1.2.3)
func bumpStable(version string) (string, error) {
	// Remove any pre-release suffixes (like -alpha, -beta, etc.)
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-.+)?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return "", errors.New("invalid version format for stable bump")
	}
	return matches[1], nil
}

// bumpAlpha increases the pre-release version (e.g., 1.2.3-alpha -> 1.2.3-alpha.1)
func bumpAlpha(version string) (string, error) {
	// Check if version has an alpha suffix - simple regex that works with all cases
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-alpha(?:\.(\d+))?)?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return "", errors.New("invalid version format for alpha bump")
	}

	baseVersion := matches[1]
	var preRelease string
	if len(matches) >= 4 && matches[3] != "" {
		// Increment the pre-release number
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return "", errors.New("invalid alpha pre-release number")
		}
		preRelease = fmt.Sprintf(".%d", num+1)
	} else {
		// Start with .1 if no pre-release number exists
		preRelease = ".1"
	}

	return baseVersion + "-alpha" + preRelease, nil
}

// bumpBeta increases the pre-release version (e.g., 1.2.3-beta -> 1.2.3-beta.1)
func bumpBeta(version string) (string, error) {
	// Check if version has a beta suffix - simple regex that works with all cases
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-beta(?:\.(\d+))?)?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return "", errors.New("invalid version format for beta bump")
	}

	baseVersion := matches[1]
	var preRelease string
	if len(matches) >= 4 && matches[3] != "" {
		// Increment the pre-release number
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return "", errors.New("invalid beta pre-release number")
		}
		preRelease = fmt.Sprintf(".%d", num+1)
	} else {
		// Start with .1 if no pre-release number exists
		preRelease = ".1"
	}

	return baseVersion + "-beta" + preRelease, nil
}

// bumpRC increases the pre-release version (e.g., 1.2.3-rc -> 1.2.3-rc.1)
func bumpRC(version string) (string, error) {
	// Check if version has a rc suffix - simple regex that works with all cases
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-rc(?:\.(\d+))?)?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return "", errors.New("invalid version format for rc bump")
	}

	baseVersion := matches[1]
	var preRelease string
	if len(matches) >= 4 && matches[3] != "" {
		// Increment the pre-release number
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return "", errors.New("invalid rc pre-release number")
		}
		preRelease = fmt.Sprintf(".%d", num+1)
	} else {
		// Start with .1 if no pre-release number exists
		preRelease = ".1"
	}

	return baseVersion + "-rc" + preRelease, nil
}

// bumpPost increases the post-release version (e.g., 1.2.3-post -> 1.2.3-post.1)
func bumpPost(version string) (string, error) {
	// Check if version has a post-suffix
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-post(\.(\d+))?)?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return "", errors.New("invalid version format for post bump")
	}

	baseVersion := matches[1]
	var preRelease string
	if len(matches) >= 4 && matches[4] != "" {
		// Increment the pre-release number
		num, err := strconv.Atoi(matches[4])
		if err != nil {
			return "", errors.New("invalid post pre-release number")
		}
		preRelease = fmt.Sprintf(".%d", num+1)
	} else {
		// Start with .1 if no pre-release number exists
		preRelease = ".1"
	}

	return baseVersion + "-post" + preRelease, nil
}

// bumpDev increases the dev version (e.g., 1.2.3-dev -> 1.2.3-dev.1)
func bumpDev(version string) (string, error) {
	// Check if version has a dev suffix
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-dev(\.(\d+))?)?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return "", errors.New("invalid version format for dev bump")
	}

	baseVersion := matches[1]
	var preRelease string
	if len(matches) >= 4 && matches[4] != "" {
		// Increment the pre-release number
		num, err := strconv.Atoi(matches[4])
		if err != nil {
			return "", errors.New("invalid dev pre-release number")
		}
		preRelease = fmt.Sprintf(".%d", num+1)
	} else {
		// Start with .1 if no pre-release number exists
		preRelease = ".1"
	}

	return baseVersion + "-dev" + preRelease, nil
}

// isValidVersion checks if a string is a valid semantic version format
func isValidVersion(version string) bool {
	re := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+(\.\d+)?)?$`)
	return re.MatchString(version)
}
