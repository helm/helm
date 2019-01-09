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

package tiller

import (
	"bytes"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/kube"
	relutil "k8s.io/helm/pkg/releaseutil"
)

// UninstallRelease deletes all of the resources associated with this release, and marks the release UNINSTALLED.
func (s *ReleaseServer) UninstallRelease(req *hapi.UninstallReleaseRequest) (*hapi.UninstallReleaseResponse, error) {
	if err := validateReleaseName(req.Name); err != nil {
		return nil, errors.Errorf("uninstall: Release name is invalid: %s", req.Name)
	}

	rels, err := s.Releases.History(req.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "uninstall: Release not loaded: %s", req.Name)
	}
	if len(rels) < 1 {
		return nil, errMissingRelease
	}

	relutil.SortByRevision(rels)
	rel := rels[len(rels)-1]

	// TODO: Are there any cases where we want to force a delete even if it's
	// already marked deleted?
	if rel.Info.Status == release.StatusUninstalled {
		if req.Purge {
			if err := s.purgeReleases(rels...); err != nil {
				return nil, errors.Wrap(err, "uninstall: Failed to purge the release")
			}
			return &hapi.UninstallReleaseResponse{Release: rel}, nil
		}
		return nil, errors.Errorf("the release named %q is already deleted", req.Name)
	}

	s.Log("uninstall: Deleting %s", req.Name)
	rel.Info.Status = release.StatusUninstalling
	rel.Info.Deleted = time.Now()
	rel.Info.Description = "Deletion in progress (or silently failed)"
	res := &hapi.UninstallReleaseResponse{Release: rel}

	if !req.DisableHooks {
		if err := s.execHook(rel.Hooks, rel.Name, rel.Namespace, hooks.PreDelete, req.Timeout); err != nil {
			return res, err
		}
	} else {
		s.Log("delete hooks disabled for %s", req.Name)
	}

	// From here on out, the release is currently considered to be in StatusUninstalling
	// state.
	if err := s.Releases.Update(rel); err != nil {
		s.Log("uninstall: Failed to store updated release: %s", err)
	}

	kept, errs := s.deleteRelease(rel)
	res.Info = kept

	if !req.DisableHooks {
		if err := s.execHook(rel.Hooks, rel.Name, rel.Namespace, hooks.PostDelete, req.Timeout); err != nil {
			errs = append(errs, err)
		}
	}

	rel.Info.Status = release.StatusUninstalled
	rel.Info.Description = "Uninstallation complete"

	if req.Purge {
		s.Log("purge requested for %s", req.Name)
		err := s.purgeReleases(rels...)
		return res, errors.Wrap(err, "uninstall: Failed to purge the release")
	}

	if err := s.Releases.Update(rel); err != nil {
		s.Log("uninstall: Failed to store updated release: %s", err)
	}

	if len(errs) > 0 {
		return res, errors.Errorf("uninstallation completed with %d error(s): %s", len(errs), joinErrors(errs))
	}
	return res, nil
}

func joinErrors(errs []error) string {
	es := make([]string, 0, len(errs))
	for _, e := range errs {
		es = append(es, e.Error())
	}
	return strings.Join(es, "; ")
}

func (s *ReleaseServer) purgeReleases(rels ...*release.Release) error {
	for _, rel := range rels {
		if _, err := s.Releases.Delete(rel.Name, rel.Version); err != nil {
			return err
		}
	}
	return nil
}

// deleteRelease deletes the release and returns manifests that were kept in the deletion process
func (s *ReleaseServer) deleteRelease(rel *release.Release) (kept string, errs []error) {
	caps, err := newCapabilities(s.discovery)
	if err != nil {
		return rel.Manifest, []error{errors.Wrap(err, "could not get apiVersions from Kubernetes")}
	}

	manifests := relutil.SplitManifests(rel.Manifest)
	_, files, err := SortManifests(manifests, caps.APIVersions, UninstallOrder)
	if err != nil {
		// We could instead just delete everything in no particular order.
		// FIXME: One way to delete at this point would be to try a label-based
		// deletion. The problem with this is that we could get a false positive
		// and delete something that was not legitimately part of this release.
		return rel.Manifest, []error{errors.Wrap(err, "corrupted release record. You must manually delete the resources")}
	}

	filesToKeep, filesToDelete := filterManifestsToKeep(files)
	if len(filesToKeep) > 0 {
		kept = summarizeKeptManifests(filesToKeep, s.KubeClient, rel.Namespace)
	}

	for _, file := range filesToDelete {
		b := bytes.NewBufferString(strings.TrimSpace(file.Content))
		if b.Len() == 0 {
			continue
		}
		if err := s.KubeClient.Delete(rel.Namespace, b); err != nil {
			s.Log("uninstall: Failed deletion of %q: %s", rel.Name, err)
			if err == kube.ErrNoObjectsVisited {
				// Rewrite the message from "no objects visited"
				err = errors.New("object not found, skipping delete")
			}
			errs = append(errs, err)
		}
	}
	return kept, errs
}
