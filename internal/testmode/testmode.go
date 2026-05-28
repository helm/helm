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

// Package testmode exposes a compile-time test-mode signal that production
// code paths may consult when they need to behave differently in a `go
// test` binary. The package has no imports and no init side effects, so it
// can be safely linked from release builds: branches gated on IsTestMode()
// are dead-code-eliminated by the compiler.
package testmode

// IsTestMode reports whether the binary was built with -tags helmtest.
// General-purpose signal that the binary was built for tests; consult it
// from production code paths that need to behave differently under test.
// Backed by a compile-time const (see mode_on.go / mode_off.go) so branches
// gated on it dead-code-eliminate in release builds.
func IsTestMode() bool {
	return testMode
}
