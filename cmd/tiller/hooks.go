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

package main

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/proto/hapi/release"
)

// hookAnno is the label name for a hook
const hookAnno = "helm.sh/hook"

const (
	preInstall  = "pre-install"
	postInstall = "post-install"
	preDelete   = "pre-delete"
	postDelete  = "post-delete"
	preUpgrade  = "pre-upgrade"
	postUpgrade = "post-upgrade"
)

var events = map[string]release.Hook_Event{
	preInstall:  release.Hook_PRE_INSTALL,
	postInstall: release.Hook_POST_INSTALL,
	preDelete:   release.Hook_PRE_DELETE,
	postDelete:  release.Hook_POST_DELETE,
	preUpgrade:  release.Hook_PRE_UPGRADE,
	postUpgrade: release.Hook_POST_UPGRADE,
}

type simpleHead struct {
	Kind     string `json:"kind,omitempty"`
	Metadata *struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata,omitempty"`
}

// sortHooks takes a map of filename/YAML contents and sorts them into hook types.
//
// The resulting hooks struct will be populated with all of the generated hooks.
// Any file that does not declare one of the hook types will be placed in the
// 'generic' bucket.
//
// To determine hook type, this looks for a YAML structure like this:
//
//  kind: SomeKind
// 	metadata:
//		annotations:
//			helm.sh/hook: pre-install
//
// Where HOOK_NAME is one of the known hooks.
//
// If a file declares more than one hook, it will be copied into all of the applicable
// hook buckets. (Note: label keys are not unique within the labels section).
//
// Files that do not parse into the expected format are simply placed into a map and
// returned.
func sortHooks(files map[string]string) ([]*release.Hook, map[string]string, error) {
	hs := []*release.Hook{}
	generic := map[string]string{}

	for n, c := range files {
		// Skip partials. We could return these as a separate map, but there doesn't
		// seem to be any need for that at this time.
		if strings.HasPrefix(path.Base(n), "_") {
			continue
		}
		// Skip empty files, and log this.
		if len(strings.TrimSpace(c)) == 0 {
			log.Printf("info: manifest %q is empty. Skipping.", n)
			continue
		}

		var sh simpleHead
		err := yaml.Unmarshal([]byte(c), &sh)

		if err != nil {
			e := fmt.Errorf("YAML parse error on %s: %s", n, err)
			return hs, generic, e
		}

		if sh.Metadata == nil || sh.Metadata.Annotations == nil || len(sh.Metadata.Annotations) == 0 {
			generic[n] = c
			continue
		}

		hookTypes, ok := sh.Metadata.Annotations[hookAnno]
		if !ok {
			generic[n] = c
			continue
		}
		h := &release.Hook{
			Name:     sh.Metadata.Name,
			Kind:     sh.Kind,
			Path:     n,
			Manifest: c,
			Events:   []release.Hook_Event{},
		}

		isHook := false
		for _, hookType := range strings.Split(hookTypes, ",") {
			hookType = strings.ToLower(strings.TrimSpace(hookType))
			e, ok := events[hookType]
			if ok {
				isHook = true
				h.Events = append(h.Events, e)
			}
		}

		if !isHook {
			log.Printf("info: skipping unknown hook: %q", hookTypes)
			continue
		}
		hs = append(hs, h)
	}
	return hs, generic, nil
}
