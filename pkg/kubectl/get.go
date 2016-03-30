/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package kubectl

// Get returns Kubernetes resources
func (r RealRunner) Get(stdin []byte, ns string) ([]byte, error) {
	args := []string{"get", "-f", "-"}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	assignStdin(cmd, stdin)

	return cmd.CombinedOutput()
}

// GetByKind gets resources by kind, name(optional), and namespace(optional)
func (r RealRunner) GetByKind(kind, name, ns string) (string, error) {
	args := []string{"get", kind}

	if name != "" {
		args = append(args, name)
	}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	o, err := cmd.CombinedOutput()
	return string(o), err
}

// Get returns the commands to kubectl
func (r PrintRunner) Get(stdin []byte, ns string) ([]byte, error) {
	args := []string{"get", "-f", "-"}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	assignStdin(cmd, stdin)

	return []byte(cmd.String()), nil
}

// GetByKind gets resources by kind, name(optional), and namespace(optional)
func (r PrintRunner) GetByKind(kind, name, ns string) (string, error) {
	args := []string{"get", kind}

	if name != "" {
		args = append([]string{name}, args...)
	}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	return cmd.String(), nil
}
