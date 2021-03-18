// +build !windows

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

package main

import (
	"os"
	"os/user"
	"path/filepath"
)

func checkPerms() {
	// This function MUST NOT FAIL, as it is just a check for a common permissions problem.
	// If for some reason the function hits a stopping condition, it may panic. But only if
	// we can be sure that it is panicking because Helm cannot proceed.

	kc := settings.KubeConfig
	if kc == "" {
		kc = os.Getenv("KUBECONFIG")
	}
	if kc == "" {
		u, err := user.Current()
		if err != nil {
			// No idea where to find KubeConfig, so return silently. Many helm commands
			// can proceed happily without a KUBECONFIG, so this is not a fatal error.
			return
		}
		kc = filepath.Join(u.HomeDir, ".kube", "config")
	}
	fi, err := os.Stat(kc)
	if err != nil {
		// DO NOT error if no KubeConfig is found. Not all commands require one.
		return
	}

	perm := fi.Mode().Perm()
	if perm&0040 > 0 {
		warning("Kubernetes configuration file is group-readable. This is insecure. Location: %s", kc)
	}
	if perm&0004 > 0 {
		warning("Kubernetes configuration file is world-readable. This is insecure. Location: %s", kc)
	}
}
