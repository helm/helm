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

package tiller

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
)

var events = map[string]release.Hook_Event{
	hooks.PreInstall:         release.Hook_PRE_INSTALL,
	hooks.PostInstall:        release.Hook_POST_INSTALL,
	hooks.PreDelete:          release.Hook_PRE_DELETE,
	hooks.PostDelete:         release.Hook_POST_DELETE,
	hooks.PreUpgrade:         release.Hook_PRE_UPGRADE,
	hooks.PostUpgrade:        release.Hook_POST_UPGRADE,
	hooks.PreRollback:        release.Hook_PRE_ROLLBACK,
	hooks.PostRollback:       release.Hook_POST_ROLLBACK,
	hooks.ReleaseTestSuccess: release.Hook_RELEASE_TEST_SUCCESS,
	hooks.ReleaseTestFailure: release.Hook_RELEASE_TEST_FAILURE,
}

// manifest represents a manifest file, which has a name and some content.
type manifest struct {
	name    string
	content string
	head    *util.SimpleHead
}

// sortManifests takes a map of filename/YAML contents and sorts them into hook types.
//
// The resulting hooks struct will be populated with all of the generated hooks.
// Any file that does not declare one of the hook types will be placed in the
// 'generic' bucket.
//
// To determine hook type, this looks for a YAML structure like this:
//
//  kind: SomeKind
//  apiVersion: v1
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
func sortManifests(files map[string]string, apis chartutil.VersionSet, sort SortOrder) ([]*release.Hook, []manifest, error) {
	hs := []*release.Hook{}
	generic := []manifest{}

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

		var sh util.SimpleHead
		err := yaml.Unmarshal([]byte(c), &sh)

		if err != nil {
			e := fmt.Errorf("YAML parse error on %s: %s", n, err)
			return hs, generic, e
		}

		if sh.Version != "" && !apis.Has(sh.Version) {
			return hs, generic, fmt.Errorf("apiVersion %q in %s is not available", sh.Version, n)
		}

		if sh.Metadata == nil || sh.Metadata.Annotations == nil || len(sh.Metadata.Annotations) == 0 {
			generic = append(generic, manifest{name: n, content: c, head: &sh})
			continue
		}

		hookTypes, ok := sh.Metadata.Annotations[hooks.HookAnno]
		if !ok {
			generic = append(generic, manifest{name: n, content: c, head: &sh})
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
	return hs, sortByKind(generic, sort), nil
}
