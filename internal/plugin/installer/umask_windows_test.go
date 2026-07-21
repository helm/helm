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

//go:build windows

package installer

// posixPermsSupported reports whether the platform honors POSIX file
// permission bits, allowing tests to assert on extracted file modes.
// Windows does not, so permission assertions are skipped there.
const posixPermsSupported = false

// processUmask returns 0 on Windows, which has no umask concept.
func processUmask() int {
	return 0
}
