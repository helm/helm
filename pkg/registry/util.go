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

package registry // import "helm.sh/helm/v3/pkg/registry"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	orascontext "oras.land/oras-go/pkg/context"
	"oras.land/oras-go/pkg/registry"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// IsOCI determines whether or not a URL is to be treated as an OCI URL
func IsOCI(url string) bool {
	return strings.HasPrefix(url, fmt.Sprintf("%s://", OCIScheme))
}

// ContainsTag determines whether a tag is found in a provided list of tags
func ContainsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if tag == t {
			return true
		}
	}
	return false
}

func GetTagMatchingVersionOrConstraint(tags []string, versionString string) (string, error) {
	var constraint *semver.Constraints
	if versionString == "" {
		// If string is empty, set wildcard constraint
		constraint, _ = semver.NewConstraint("*")
	} else {
		// when customer input exact version, check whether have exact match
		// one first
		for _, v := range tags {
			if versionString == v {
				return v, nil
			}
		}

		// Otherwise set constraint to the string given
		var err error
		constraint, err = semver.NewConstraint(versionString)
		if err != nil {
			return "", err
		}
	}

	// Otherwise try to find the first available version matching the string,
	// in case it is a constraint
	for _, v := range tags {
		test, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		if constraint.Check(test) {
			return v, nil
		}
	}

	return "", errors.Errorf("Could not locate a version matching provided version string %s", versionString)
}

// extractChartMeta is used to extract a chart metadata from a byte array
func extractChartMeta(chartData []byte) (*chart.Metadata, error) {
	ch, err := loader.LoadArchive(bytes.NewReader(chartData))
	if err != nil {
		return nil, err
	}
	return ch.Metadata, nil
}

// ctx retrieves a fresh context.
// disable verbose logging coming from ORAS (unless debug is enabled)
func ctx(out io.Writer, debug bool) context.Context {
	if !debug {
		return orascontext.Background()
	}
	ctx := orascontext.WithLoggerFromWriter(context.Background(), out)
	orascontext.GetLogger(ctx).Logger.SetLevel(logrus.DebugLevel)
	return ctx
}

// parseReference will parse and validate the reference, and clean tags when
// applicable tags are only cleaned when plus (+) signs are present, and are
// converted to underscores (_) before pushing
// See https://github.com/helm/helm/issues/10166
func parseReference(raw string) (registry.Reference, error) {
	// The sole possible reference modification is replacing plus (+) signs
	// present in tags with underscores (_). To do this properly, we first
	// need to identify a tag, and then pass it on to the reference parser
	// NOTE: Passing immediately to the reference parser will fail since (+)
	// signs are an invalid tag character, and simply replacing all plus (+)
	// occurrences could invalidate other portions of the URI
	parts := strings.Split(raw, ":")
	if len(parts) > 1 && !strings.Contains(parts[len(parts)-1], "/") {
		tag := parts[len(parts)-1]

		if tag != "" {
			// Replace any plus (+) signs with known underscore (_) conversion
			newTag := strings.ReplaceAll(tag, "+", "_")
			raw = strings.ReplaceAll(raw, tag, newTag)
		}
	}

	return registry.ParseReference(raw)
}
