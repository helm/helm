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
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/hooks"
	relutil "k8s.io/helm/pkg/releaseutil"
)

// InstallRelease installs a release and stores the release record.
func (s *ReleaseServer) InstallRelease(req *hapi.InstallReleaseRequest) (*release.Release, error) {
	s.Log("preparing install for %s", req.Name)
	rel, err := s.prepareRelease(req)
	if err != nil {
		// On dry run, append the manifest contents to a failed release. This is
		// a stop-gap until we can revisit an error backchannel post-2.0.
		if req.DryRun && strings.HasPrefix(err.Error(), "YAML parse error") {
			err = errors.Wrap(err, rel.Manifest)
		}
		return rel, errors.Wrap(err, "failed install prepare step")
	}

	s.Log("performing install for %s", req.Name)
	res, err := s.performRelease(rel, req)
	return res, errors.Wrap(err, "failed install perform step")
}

// prepareRelease builds a release for an install operation.
func (s *ReleaseServer) prepareRelease(req *hapi.InstallReleaseRequest) (*release.Release, error) {
	if req.Chart == nil {
		return nil, errMissingChart
	}

	name, err := s.uniqName(req.Name, req.ReuseName)
	if err != nil {
		return nil, err
	}

	caps, err := capabilities(s.discovery)
	if err != nil {
		return nil, err
	}

	revision := 1
	ts := time.Now()
	options := chartutil.ReleaseOptions{
		Name:      name,
		IsInstall: true,
	}
	valuesToRender, err := chartutil.ToRenderValuesCaps(req.Chart, req.Values, options, caps)
	if err != nil {
		return nil, err
	}

	hooks, manifestDoc, notesTxt, err := s.renderResources(req.Chart, valuesToRender, caps.APIVersions)
	if err != nil {
		// Return a release with partial data so that client can show debugging
		// information.
		rel := &release.Release{
			Name:      name,
			Namespace: req.Namespace,
			Chart:     req.Chart,
			Config:    req.Values,
			Info: &release.Info{
				FirstDeployed: ts,
				LastDeployed:  ts,
				Status:        release.StatusUnknown,
				Description:   fmt.Sprintf("Install failed: %s", err),
			},
			Version: 0,
		}
		if manifestDoc != nil {
			rel.Manifest = manifestDoc.String()
		}
		return rel, err
	}

	// Store a release.
	rel := &release.Release{
		Name:      name,
		Namespace: req.Namespace,
		Chart:     req.Chart,
		Config:    req.Values,
		Info: &release.Info{
			FirstDeployed: ts,
			LastDeployed:  ts,
			Status:        release.StatusPendingInstall,
			Description:   "Initial install underway", // Will be overwritten.
		},
		Manifest: manifestDoc.String(),
		Hooks:    hooks,
		Version:  revision,
	}
	if len(notesTxt) > 0 {
		rel.Info.Notes = notesTxt
	}

	err = validateManifest(s.KubeClient, req.Namespace, manifestDoc.Bytes())
	return rel, err
}

// performRelease runs a release.
func (s *ReleaseServer) performRelease(r *release.Release, req *hapi.InstallReleaseRequest) (*release.Release, error) {

	if req.DryRun {
		s.Log("dry run for %s", r.Name)
		r.Info.Description = "Dry run complete"
		return r, nil
	}

	// pre-install hooks
	if !req.DisableHooks {
		if err := s.execHook(r.Hooks, r.Name, r.Namespace, hooks.PreInstall, req.Timeout); err != nil {
			return r, err
		}
	} else {
		s.Log("install hooks disabled for %s", req.Name)
	}

	switch h, err := s.Releases.History(req.Name); {
	// if this is a replace operation, append to the release history
	case req.ReuseName && err == nil && len(h) >= 1:
		s.Log("name reuse for %s requested, replacing release", req.Name)
		// get latest release revision
		relutil.Reverse(h, relutil.SortByRevision)

		// old release
		old := h[0]

		// update old release status
		old.Info.Status = release.StatusSuperseded
		s.recordRelease(old, true)

		// update new release with next revision number
		// so as to append to the old release's history
		r.Version = old.Version + 1
		updateReq := &hapi.UpdateReleaseRequest{
			Wait:     req.Wait,
			Recreate: false,
			Timeout:  req.Timeout,
		}
		s.recordRelease(r, false)
		if err := s.updateRelease(old, r, updateReq); err != nil {
			msg := fmt.Sprintf("Release replace %q failed: %s", r.Name, err)
			s.Log("warning: %s", msg)
			old.Info.Status = release.StatusSuperseded
			r.Info.Status = release.StatusFailed
			r.Info.Description = msg
			s.recordRelease(old, true)
			s.recordRelease(r, true)
			return r, err
		}

	default:
		// nothing to replace, create as normal
		// regular manifests
		s.recordRelease(r, false)
		b := bytes.NewBufferString(r.Manifest)
		if err := s.KubeClient.Create(r.Namespace, b, req.Timeout, req.Wait); err != nil {
			msg := fmt.Sprintf("Release %q failed: %s", r.Name, err)
			s.Log("warning: %s", msg)
			r.Info.Status = release.StatusFailed
			r.Info.Description = msg
			s.recordRelease(r, true)
			return r, errors.Wrapf(err, "release %s failed", r.Name)
		}
	}

	// post-install hooks
	if !req.DisableHooks {
		if err := s.execHook(r.Hooks, r.Name, r.Namespace, hooks.PostInstall, req.Timeout); err != nil {
			msg := fmt.Sprintf("Release %q failed post-install: %s", r.Name, err)
			s.Log("warning: %s", msg)
			r.Info.Status = release.StatusFailed
			r.Info.Description = msg
			s.recordRelease(r, true)
			return r, err
		}
	}

	r.Info.Status = release.StatusDeployed
	r.Info.Description = "Install complete"
	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	s.recordRelease(r, true)

	return r, nil
}
