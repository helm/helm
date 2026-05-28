//go:build helmtest

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

package version

// This file is only compiled when the `helmtest` build tag is set (applied by
// the Makefile to all test invocations). It seeds the testing-version
// sentinels so that production code paths in this package and in
// pkg/chart/common substitute stable values instead of attempting to read
// build info from a `go test` binary (which has no module info and would
// panic during package init).

func init() {
	KubeVersionMajorTesting = 1
	KubeVersionMinorTesting = 20
}
