package main

import (
	"log"
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
func sortHooks(files map[string]string) (hs []*release.Hook, generic map[string]string) {
	hs = []*release.Hook{}
	generic = map[string]string{}

	for n, c := range files {
		var sh simpleHead
		err := yaml.Unmarshal([]byte(c), &sh)

		if err != nil {
			log.Printf("YAML parse error on %s: %s (skipping)", n, err)
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
	return
}
