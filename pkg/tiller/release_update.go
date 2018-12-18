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
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/hooks"
)

// UpdateRelease takes an existing release and new information, and upgrades the release.
func (s *ReleaseServer) UpdateRelease(req *hapi.UpdateReleaseRequest) (*release.Release, error) {
	if err := validateReleaseName(req.Name); err != nil {
		return nil, errors.Errorf("updateRelease: Release name is invalid: %s", req.Name)
	}
	s.Log("preparing update for %s", req.Name)
	currentRelease, updatedRelease, err := s.prepareUpdate(req)
	if err != nil {
		if req.Force {
			// Use the --force, Luke.
			return s.performUpdateForce(req)
		}
		return nil, err
	}

	s.Releases.MaxHistory = req.MaxHistory

	if !req.DryRun {
		s.Log("creating updated release for %s", req.Name)
		if err := s.Releases.Create(updatedRelease); err != nil {
			return nil, err
		}
	}

	s.Log("performing update for %s", req.Name)
	res, err := s.performUpdate(currentRelease, updatedRelease, req)
	if err != nil {
		return res, err
	}

	if !req.DryRun {
		s.Log("updating status for updated release for %s", req.Name)
		if err := s.Releases.Update(updatedRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

// prepareUpdate builds an updated release for an update operation.
func (s *ReleaseServer) prepareUpdate(req *hapi.UpdateReleaseRequest) (*release.Release, *release.Release, error) {
	if req.Chart == nil {
		return nil, nil, errMissingChart
	}

	// finds the deployed release with the given name
	currentRelease, err := s.Releases.Deployed(req.Name)
	if err != nil {
		return nil, nil, err
	}

	// determine if values will be reused
	if err := s.reuseValues(req, currentRelease); err != nil {
		return nil, nil, err
	}

	// finds the non-deleted release with the given name
	lastRelease, err := s.Releases.Last(req.Name)
	if err != nil {
		return nil, nil, err
	}

	// Increment revision count. This is passed to templates, and also stored on
	// the release object.
	revision := lastRelease.Version + 1

	ts := time.Now()
	options := chartutil.ReleaseOptions{
		Name:      req.Name,
		IsUpgrade: true,
	}

	caps, err := capabilities(s.discovery)
	if err != nil {
		return nil, nil, err
	}
	valuesToRender, err := chartutil.ToRenderValues(req.Chart, req.Values, options, caps)
	if err != nil {
		return nil, nil, err
	}

	hooks, manifestDoc, notesTxt, err := s.renderResources(req.Chart, valuesToRender, caps.APIVersions)
	if err != nil {
		return nil, nil, err
	}

	// Store an updated release.
	updatedRelease := &release.Release{
		Name:      req.Name,
		Namespace: currentRelease.Namespace,
		Chart:     req.Chart,
		Config:    req.Values,
		Info: &release.Info{
			FirstDeployed: currentRelease.Info.FirstDeployed,
			LastDeployed:  ts,
			Status:        release.StatusPendingUpgrade,
			Description:   "Preparing upgrade", // This should be overwritten later.
		},
		Version:  revision,
		Manifest: manifestDoc.String(),
		Hooks:    hooks,
	}

	if len(notesTxt) > 0 {
		updatedRelease.Info.Notes = notesTxt
	}
	err = validateManifest(s.KubeClient, currentRelease.Namespace, manifestDoc.Bytes())
	return currentRelease, updatedRelease, err
}

// performUpdateForce performs the same action as a `helm uninstall && helm install --replace`.
func (s *ReleaseServer) performUpdateForce(req *hapi.UpdateReleaseRequest) (*release.Release, error) {
	// find the last release with the given name
	oldRelease, err := s.Releases.Last(req.Name)
	if err != nil {
		return nil, err
	}

	newRelease, err := s.prepareRelease(&hapi.InstallReleaseRequest{
		Chart:        req.Chart,
		Values:       req.Values,
		DryRun:       req.DryRun,
		Name:         req.Name,
		DisableHooks: req.DisableHooks,
		Namespace:    oldRelease.Namespace,
		ReuseName:    true,
		Timeout:      req.Timeout,
		Wait:         req.Wait,
	})
	if err != nil {
		// On dry run, append the manifest contents to a failed release. This is
		// a stop-gap until we can revisit an error backchannel post-2.0.
		if req.DryRun && strings.HasPrefix(err.Error(), "YAML parse error") {
			err = errors.Wrap(err, newRelease.Manifest)
		}
		return newRelease, errors.Wrap(err, "failed update prepare step")
	}

	// From here on out, the release is considered to be in StatusUninstalling or StatusUninstalled
	// state. There is no turning back.
	oldRelease.Info.Status = release.StatusUninstalling
	oldRelease.Info.Deleted = time.Now()
	oldRelease.Info.Description = "Deletion in progress (or silently failed)"
	s.recordRelease(oldRelease, true)

	// pre-delete hooks
	if !req.DisableHooks {
		if err := s.execHook(oldRelease.Hooks, oldRelease.Name, oldRelease.Namespace, hooks.PreDelete, req.Timeout); err != nil {
			return newRelease, err
		}
	} else {
		s.Log("hooks disabled for %s", req.Name)
	}

	// delete manifests from the old release
	_, errs := s.deleteRelease(oldRelease)

	oldRelease.Info.Status = release.StatusUninstalled
	oldRelease.Info.Description = "Uninstallation complete"
	s.recordRelease(oldRelease, true)

	if len(errs) > 0 {
		return newRelease, errors.Errorf("upgrade --force successfully uninstalled the previous release, but encountered %d error(s) and cannot continue: %s", len(errs), joinErrors(errs))
	}

	// post-delete hooks
	if !req.DisableHooks {
		if err := s.execHook(oldRelease.Hooks, oldRelease.Name, oldRelease.Namespace, hooks.PostDelete, req.Timeout); err != nil {
			return newRelease, err
		}
	}

	// pre-install hooks
	if !req.DisableHooks {
		if err := s.execHook(newRelease.Hooks, newRelease.Name, newRelease.Namespace, hooks.PreInstall, req.Timeout); err != nil {
			return newRelease, err
		}
	}

	// update new release with next revision number so as to append to the old release's history
	newRelease.Version = oldRelease.Version + 1
	s.recordRelease(newRelease, false)
	if err := s.updateRelease(oldRelease, newRelease, req); err != nil {
		msg := fmt.Sprintf("Upgrade %q failed: %s", newRelease.Name, err)
		s.Log("warning: %s", msg)
		newRelease.Info.Status = release.StatusFailed
		newRelease.Info.Description = msg
		s.recordRelease(newRelease, true)
		return newRelease, err
	}

	// post-install hooks
	if !req.DisableHooks {
		if err := s.execHook(newRelease.Hooks, newRelease.Name, newRelease.Namespace, hooks.PostInstall, req.Timeout); err != nil {
			msg := fmt.Sprintf("Release %q failed post-install: %s", newRelease.Name, err)
			s.Log("warning: %s", msg)
			newRelease.Info.Status = release.StatusFailed
			newRelease.Info.Description = msg
			s.recordRelease(newRelease, true)
			return newRelease, err
		}
	}

	newRelease.Info.Status = release.StatusDeployed
	newRelease.Info.Description = "Upgrade complete"
	s.recordRelease(newRelease, true)

	return newRelease, nil
}

func (s *ReleaseServer) performUpdate(originalRelease, updatedRelease *release.Release, req *hapi.UpdateReleaseRequest) (*release.Release, error) {

	if req.DryRun {
		s.Log("dry run for %s", updatedRelease.Name)
		updatedRelease.Info.Description = "Dry run complete"
		return updatedRelease, nil
	}

	// pre-upgrade hooks
	if !req.DisableHooks {
		if err := s.execHook(updatedRelease.Hooks, updatedRelease.Name, updatedRelease.Namespace, hooks.PreUpgrade, req.Timeout); err != nil {
			return updatedRelease, err
		}
	} else {
		s.Log("update hooks disabled for %s", req.Name)
	}
	if err := s.updateRelease(originalRelease, updatedRelease, req); err != nil {
		msg := fmt.Sprintf("Upgrade %q failed: %s", updatedRelease.Name, err)
		s.Log("warning: %s", msg)
		updatedRelease.Info.Status = release.StatusFailed
		updatedRelease.Info.Description = msg
		s.recordRelease(originalRelease, true)
		s.recordRelease(updatedRelease, true)
		return updatedRelease, err
	}

	// post-upgrade hooks
	if !req.DisableHooks {
		if err := s.execHook(updatedRelease.Hooks, updatedRelease.Name, updatedRelease.Namespace, hooks.PostUpgrade, req.Timeout); err != nil {
			return updatedRelease, err
		}
	}

	originalRelease.Info.Status = release.StatusSuperseded
	s.recordRelease(originalRelease, true)

	updatedRelease.Info.Status = release.StatusDeployed
	updatedRelease.Info.Description = "Upgrade complete"

	return updatedRelease, nil
}

// updateRelease performs an update from current to target release
func (s *ReleaseServer) updateRelease(current, target *release.Release, req *hapi.UpdateReleaseRequest) error {
	c := bytes.NewBufferString(current.Manifest)
	t := bytes.NewBufferString(target.Manifest)
	return s.KubeClient.Update(target.Namespace, c, t, req.Force, req.Recreate, req.Timeout, req.Wait)
}
