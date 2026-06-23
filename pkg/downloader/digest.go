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
	"fmt"
	"strings"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/provenance"
)

// formatDigest returns a digest string in the canonical sha256: form.
func formatDigest(hex string) string {
	if hex == "" {
		return ""
	}
	if strings.HasPrefix(hex, "sha256:") {
		return hex
	}
	return "sha256:" + hex
}

// digestEqual compares two digests regardless of sha256: prefix.
func digestEqual(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return stripDigestAlgorithm(a) == stripDigestAlgorithm(b)
}

// digestFile computes the SHA256 digest of a chart archive file.
func digestFile(path string) (string, error) {
	hex, err := provenance.DigestFile(path)
	if err != nil {
		return "", err
	}
	return formatDigest(hex), nil
}

// recordDependencyDigest computes and stores the tarball digest on a lock dependency entry.
func recordDependencyDigest(dep *chart.Dependency, tarballPath string) error {
	digest, err := digestFile(tarballPath)
	if err != nil {
		return err
	}
	if dep.Digest != "" && !digestEqual(dep.Digest, digest) {
		return fmt.Errorf("chart.lock digest mismatch for %s-%s: expected %s, got %s", dep.Name, dep.Version, formatDigest(dep.Digest), digest)
	}
	dep.Digest = digest
	return nil
}

// lockNeedsWrite reports whether the new lock differs from the old lock in metadata
// or per-dependency content digests.
func lockNeedsWrite(old, updated *chart.Lock) bool {
	if old == nil {
		return true
	}
	if old.Digest != updated.Digest {
		return true
	}
	return !dependencyDigestsEqual(old.Dependencies, updated.Dependencies)
}

func dependencyDigestsEqual(a, b []*chart.Dependency) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] == nil || b[i] == nil {
			if a[i] != b[i] {
				return false
			}
			continue
		}
		if a[i].Name != b[i].Name || !digestEqual(a[i].Digest, b[i].Digest) {
			return false
		}
	}
	return true
}
