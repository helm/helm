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

package plugin

// InvokeExecError is returned when a plugin invocation returns a non-zero status/exit code
// - subprocess plugin: child process exit code
// - extism plugin: wasm function return code
type InvokeExecError struct {
	ExitCode int   // Exit code from plugin code execution
	Err      error // Underlying error
}

// Error implements the error interface
func (e *InvokeExecError) Error() string {
	return e.Err.Error()
}
