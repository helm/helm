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
	"time"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/hooks"
)

// RollbackRelease rolls back to a previous version of the given release.
func (s *ReleaseServer) RollbackRelease(req *hapi.RollbackReleaseRequest) (*release.Release, error) {
	s.Log("preparing rollback of %s", req.Name)
	currentRelease, targetRelease, err := s.prepareRollback(req)
	if err != nil {
		return nil, err
	}

	if !req.DryRun {
		s.Log("creating rolled back release for %s", req.Name)
		if err := s.Releases.Create(targetRelease); err != nil {
			return nil, err
		}
	}
	s.Log("performing rollback of %s", req.Name)
	res, err := s.performRollback(currentRelease, targetRelease, req)
	if err != nil {
		return res, err
	}

	if !req.DryRun {
		s.Log("updating status for rolled back release for %s", req.Name)
		if err := s.Releases.Update(targetRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

// prepareRollback finds the previous release and prepares a new release object with
// the previous release's configuration
func (s *ReleaseServer) prepareRollback(req *hapi.RollbackReleaseRequest) (*release.Release, *release.Release, error) {
	if err := validateReleaseName(req.Name); err != nil {
		return nil, nil, errors.Errorf("prepareRollback: Release name is invalid: %s", req.Name)
	}

	if req.Version < 0 {
		return nil, nil, errInvalidRevision
	}

	currentRelease, err := s.Releases.Last(req.Name)
	if err != nil {
		return nil, nil, err
	}

	previousVersion := req.Version
	if req.Version == 0 {
		previousVersion = currentRelease.Version - 1
	}

	s.Log("rolling back %s (current: v%d, target: v%d)", req.Name, currentRelease.Version, previousVersion)

	previousRelease, err := s.Releases.Get(req.Name, previousVersion)
	if err != nil {
		return nil, nil, err
	}

	// Store a new release object with previous release's configuration
	targetRelease := &release.Release{
		Name:      req.Name,
		Namespace: currentRelease.Namespace,
		Chart:     previousRelease.Chart,
		Config:    previousRelease.Config,
		Info: &release.Info{
			FirstDeployed: currentRelease.Info.FirstDeployed,
			LastDeployed:  time.Now(),
			Status:        release.StatusPendingRollback,
			Notes:         previousRelease.Info.Notes,
			// Because we lose the reference to previous version elsewhere, we set the
			// message here, and only override it later if we experience failure.
			Description: fmt.Sprintf("Rollback to %d", previousVersion),
		},
		Version:  currentRelease.Version + 1,
		Manifest: previousRelease.Manifest,
		Hooks:    previousRelease.Hooks,
	}

	return currentRelease, targetRelease, nil
}

func (s *ReleaseServer) performRollback(currentRelease, targetRelease *release.Release, req *hapi.RollbackReleaseRequest) (*release.Release, error) {

	if req.DryRun {
		s.Log("dry run for %s", targetRelease.Name)
		return targetRelease, nil
	}

	// pre-rollback hooks
	if !req.DisableHooks {
		if err := s.execHook(targetRelease.Hooks, targetRelease.Name, targetRelease.Namespace, hooks.PreRollback, req.Timeout); err != nil {
			return targetRelease, err
		}
	} else {
		s.Log("rollback hooks disabled for %s", req.Name)
	}

	c := bytes.NewBufferString(currentRelease.Manifest)
	t := bytes.NewBufferString(targetRelease.Manifest)
	if err := s.KubeClient.Update(targetRelease.Namespace, c, t, req.Force, req.Recreate, req.Timeout, req.Wait); err != nil {
		msg := fmt.Sprintf("Rollback %q failed: %s", targetRelease.Name, err)
		s.Log("warning: %s", msg)
		currentRelease.Info.Status = release.StatusSuperseded
		targetRelease.Info.Status = release.StatusFailed
		targetRelease.Info.Description = msg
		s.recordRelease(currentRelease, true)
		s.recordRelease(targetRelease, true)
		return targetRelease, err
	}

	// post-rollback hooks
	if !req.DisableHooks {
		if err := s.execHook(targetRelease.Hooks, targetRelease.Name, targetRelease.Namespace, hooks.PostRollback, req.Timeout); err != nil {
			return targetRelease, err
		}
	}

	deployed, err := s.Releases.DeployedAll(currentRelease.Name)
	if err != nil {
		return nil, err
	}
	// Supersede all previous deployments, see issue #2941.
	for _, r := range deployed {
		s.Log("superseding previous deployment %d", r.Version)
		r.Info.Status = release.StatusSuperseded
		s.recordRelease(r, true)
	}

	targetRelease.Info.Status = release.StatusDeployed

	return targetRelease, nil
}
