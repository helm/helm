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
	"errors"
	"fmt"
	"log"
	"path"
	"regexp"
	"strings"

	"github.com/technosophos/moniker"
	ctx "golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/typed/discovery"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	reltesting "k8s.io/helm/pkg/releasetesting"
	relutil "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/timeconv"
	"k8s.io/helm/pkg/version"
)

// releaseNameMaxLen is the maximum length of a release name.
//
// As of Kubernetes 1.4, the max limit on a name is 63 chars. We reserve 10 for
// charts to add data. Effectively, that gives us 53 chars.
// See https://github.com/kubernetes/helm/issues/1528
const releaseNameMaxLen = 53

// NOTESFILE_SUFFIX that we want to treat special. It goes through the templating engine
// but it's not a yaml file (resource) hence can't have hooks, etc. And the user actually
// wants to see this file after rendering in the status command. However, it must be a suffix
// since there can be filepath in front of it.
const notesFileSuffix = "NOTES.txt"

var (
	// errMissingChart indicates that a chart was not provided.
	errMissingChart = errors.New("no chart provided")
	// errMissingRelease indicates that a release (name) was not provided.
	errMissingRelease = errors.New("no release provided")
	// errInvalidRevision indicates that an invalid release revision number was provided.
	errInvalidRevision = errors.New("invalid release revision")
)

// ListDefaultLimit is the default limit for number of items returned in a list.
var ListDefaultLimit int64 = 512

// ValidName is a regular expression for names.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
//
// We modified that. First, we added start and end delimiters. Second, we changed
// the final ? to + to require that the pattern match at least once. This modification
// prevents an empty string from matching.
var ValidName = regexp.MustCompile("^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$")

// ReleaseServer implements the server-side gRPC endpoint for the HAPI services.
type ReleaseServer struct {
	env       *environment.Environment
	clientset internalclientset.Interface
}

// NewReleaseServer creates a new release server.
func NewReleaseServer(env *environment.Environment, clientset internalclientset.Interface) *ReleaseServer {
	return &ReleaseServer{
		env:       env,
		clientset: clientset,
	}
}

// ListReleases lists the releases found by the server.
func (s *ReleaseServer) ListReleases(req *services.ListReleasesRequest, stream services.ReleaseService_ListReleasesServer) error {
	if len(req.StatusCodes) == 0 {
		req.StatusCodes = []release.Status_Code{release.Status_DEPLOYED}
	}

	//rels, err := s.env.Releases.ListDeployed()
	rels, err := s.env.Releases.ListFilterAll(func(r *release.Release) bool {
		for _, sc := range req.StatusCodes {
			if sc == r.Info.Status.Code {
				return true
			}
		}
		return false
	})
	if err != nil {
		return err
	}

	if req.Namespace != "" {
		rels, err = filterByNamespace(req.Namespace, rels)
		if err != nil {
			return err
		}
	}

	if len(req.Filter) != 0 {
		rels, err = filterReleases(req.Filter, rels)
		if err != nil {
			return err
		}
	}

	total := int64(len(rels))

	switch req.SortBy {
	case services.ListSort_NAME:
		relutil.SortByName(rels)
	case services.ListSort_LAST_RELEASED:
		relutil.SortByDate(rels)
	}

	if req.SortOrder == services.ListSort_DESC {
		ll := len(rels)
		rr := make([]*release.Release, ll)
		for i, item := range rels {
			rr[ll-i-1] = item
		}
		rels = rr
	}

	l := int64(len(rels))
	if req.Offset != "" {

		i := -1
		for ii, cur := range rels {
			if cur.Name == req.Offset {
				i = ii
			}
		}
		if i == -1 {
			return fmt.Errorf("offset %q not found", req.Offset)
		}

		if len(rels) < i {
			return fmt.Errorf("no items after %q", req.Offset)
		}

		rels = rels[i:]
		l = int64(len(rels))
	}

	if req.Limit == 0 {
		req.Limit = ListDefaultLimit
	}

	next := ""
	if l > req.Limit {
		next = rels[req.Limit].Name
		rels = rels[0:req.Limit]
		l = int64(len(rels))
	}

	res := &services.ListReleasesResponse{
		Next:     next,
		Count:    l,
		Total:    total,
		Releases: rels,
	}
	return stream.Send(res)
}

func filterByNamespace(namespace string, rels []*release.Release) ([]*release.Release, error) {
	matches := []*release.Release{}
	for _, r := range rels {
		if namespace == r.Namespace {
			matches = append(matches, r)
		}
	}
	return matches, nil
}

func filterReleases(filter string, rels []*release.Release) ([]*release.Release, error) {
	preg, err := regexp.Compile(filter)
	if err != nil {
		return rels, err
	}
	matches := []*release.Release{}
	for _, r := range rels {
		if preg.MatchString(r.Name) {
			matches = append(matches, r)
		}
	}
	return matches, nil
}

// GetVersion sends the server version.
func (s *ReleaseServer) GetVersion(c ctx.Context, req *services.GetVersionRequest) (*services.GetVersionResponse, error) {
	v := version.GetVersionProto()
	return &services.GetVersionResponse{Version: v}, nil
}

// GetReleaseStatus gets the status information for a named release.
func (s *ReleaseServer) GetReleaseStatus(c ctx.Context, req *services.GetReleaseStatusRequest) (*services.GetReleaseStatusResponse, error) {
	if !ValidName.MatchString(req.Name) {
		return nil, errMissingRelease
	}

	var rel *release.Release

	if req.Version <= 0 {
		var err error
		rel, err = s.env.Releases.Last(req.Name)
		if err != nil {
			return nil, fmt.Errorf("getting deployed release %q: %s", req.Name, err)
		}
	} else {
		var err error
		if rel, err = s.env.Releases.Get(req.Name, req.Version); err != nil {
			return nil, fmt.Errorf("getting release '%s' (v%d): %s", req.Name, req.Version, err)
		}
	}

	if rel.Info == nil {
		return nil, errors.New("release info is missing")
	}
	if rel.Chart == nil {
		return nil, errors.New("release chart is missing")
	}

	sc := rel.Info.Status.Code
	statusResp := &services.GetReleaseStatusResponse{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Info:      rel.Info,
	}

	// Ok, we got the status of the release as we had jotted down, now we need to match the
	// manifest we stashed away with reality from the cluster.
	kubeCli := s.env.KubeClient
	resp, err := kubeCli.Get(rel.Namespace, bytes.NewBufferString(rel.Manifest))
	if sc == release.Status_DELETED || sc == release.Status_FAILED {
		// Skip errors if this is already deleted or failed.
		return statusResp, nil
	} else if err != nil {
		log.Printf("warning: Get for %s failed: %v", rel.Name, err)
		return nil, err
	}
	rel.Info.Status.Resources = resp
	return statusResp, nil
}

// GetReleaseContent gets all of the stored information for the given release.
func (s *ReleaseServer) GetReleaseContent(c ctx.Context, req *services.GetReleaseContentRequest) (*services.GetReleaseContentResponse, error) {
	if !ValidName.MatchString(req.Name) {
		return nil, errMissingRelease
	}

	if req.Version <= 0 {
		rel, err := s.env.Releases.Deployed(req.Name)
		return &services.GetReleaseContentResponse{Release: rel}, err
	}

	rel, err := s.env.Releases.Get(req.Name, req.Version)
	return &services.GetReleaseContentResponse{Release: rel}, err
}

// UpdateRelease takes an existing release and new information, and upgrades the release.
func (s *ReleaseServer) UpdateRelease(c ctx.Context, req *services.UpdateReleaseRequest) (*services.UpdateReleaseResponse, error) {
	currentRelease, updatedRelease, err := s.prepareUpdate(req)
	if err != nil {
		return nil, err
	}

	res, err := s.performUpdate(currentRelease, updatedRelease, req)
	if err != nil {
		return res, err
	}

	if !req.DryRun {
		if err := s.env.Releases.Create(updatedRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *ReleaseServer) performUpdate(originalRelease, updatedRelease *release.Release, req *services.UpdateReleaseRequest) (*services.UpdateReleaseResponse, error) {
	res := &services.UpdateReleaseResponse{Release: updatedRelease}

	if req.DryRun {
		log.Printf("Dry run for %s", updatedRelease.Name)
		res.Release.Info.Description = "Dry run complete"
		return res, nil
	}

	// pre-upgrade hooks
	if !req.DisableHooks {
		if err := s.execHook(updatedRelease.Hooks, updatedRelease.Name, updatedRelease.Namespace, hooks.PreUpgrade, req.Timeout); err != nil {
			return res, err
		}
	}

	if err := s.performKubeUpdate(originalRelease, updatedRelease, req.Recreate, req.Timeout, req.Wait); err != nil {
		msg := fmt.Sprintf("Upgrade %q failed: %s", updatedRelease.Name, err)
		log.Printf("warning: %s", msg)
		originalRelease.Info.Status.Code = release.Status_SUPERSEDED
		updatedRelease.Info.Status.Code = release.Status_FAILED
		updatedRelease.Info.Description = msg
		s.recordRelease(originalRelease, true)
		s.recordRelease(updatedRelease, false)
		return res, err
	}

	// post-upgrade hooks
	if !req.DisableHooks {
		if err := s.execHook(updatedRelease.Hooks, updatedRelease.Name, updatedRelease.Namespace, hooks.PostUpgrade, req.Timeout); err != nil {
			return res, err
		}
	}

	originalRelease.Info.Status.Code = release.Status_SUPERSEDED
	s.recordRelease(originalRelease, true)

	updatedRelease.Info.Status.Code = release.Status_DEPLOYED
	updatedRelease.Info.Description = "Upgrade complete"

	return res, nil
}

// reuseValues copies values from the current release to a new release if the
// new release does not have any values.
//
// If the request already has values, or if there are no values in the current
// release, this does nothing.
//
// This is skipped if the req.ResetValues flag is set, in which case the
// request values are not altered.
func (s *ReleaseServer) reuseValues(req *services.UpdateReleaseRequest, current *release.Release) {
	if req.ResetValues {
		// If ResetValues is set, we comletely ignore current.Config.
		log.Print("Reset values to the chart's original version.")
		return
	}
	// If req.Values is empty, but current. config is not, copy current into the
	// request.
	if (req.Values == nil || req.Values.Raw == "" || req.Values.Raw == "{}\n") &&
		current.Config != nil &&
		current.Config.Raw != "" &&
		current.Config.Raw != "{}\n" {
		log.Printf("Copying values from %s (v%d) to new release.", current.Name, current.Version)
		req.Values = current.Config
	}
}

// prepareUpdate builds an updated release for an update operation.
func (s *ReleaseServer) prepareUpdate(req *services.UpdateReleaseRequest) (*release.Release, *release.Release, error) {
	if !ValidName.MatchString(req.Name) {
		return nil, nil, errMissingRelease
	}

	if req.Chart == nil {
		return nil, nil, errMissingChart
	}

	// finds the non-deleted release with the given name
	currentRelease, err := s.env.Releases.Last(req.Name)
	if err != nil {
		return nil, nil, err
	}

	// If new values were not supplied in the upgrade, re-use the existing values.
	s.reuseValues(req, currentRelease)

	// Increment revision count. This is passed to templates, and also stored on
	// the release object.
	revision := currentRelease.Version + 1

	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{
		Name:      req.Name,
		Time:      ts,
		Namespace: currentRelease.Namespace,
		IsUpgrade: true,
		Revision:  int(revision),
	}

	caps, err := capabilities(s.clientset.Discovery())
	if err != nil {
		return nil, nil, err
	}
	valuesToRender, err := chartutil.ToRenderValuesCaps(req.Chart, req.Values, options, caps)
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
			Status:        &release.Status{Code: release.Status_UNKNOWN},
			Description:   "Preparing upgrade", // This should be overwritten later.
		},
		Version:  revision,
		Manifest: manifestDoc.String(),
		Hooks:    hooks,
	}

	if len(notesTxt) > 0 {
		updatedRelease.Info.Status.Notes = notesTxt
	}
	err = validateManifest(s.env.KubeClient, currentRelease.Namespace, manifestDoc.Bytes())
	return currentRelease, updatedRelease, err
}

// RollbackRelease rolls back to a previous version of the given release.
func (s *ReleaseServer) RollbackRelease(c ctx.Context, req *services.RollbackReleaseRequest) (*services.RollbackReleaseResponse, error) {
	currentRelease, targetRelease, err := s.prepareRollback(req)
	if err != nil {
		return nil, err
	}

	res, err := s.performRollback(currentRelease, targetRelease, req)
	if err != nil {
		return res, err
	}

	if !req.DryRun {
		if err := s.env.Releases.Create(targetRelease); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *ReleaseServer) performRollback(currentRelease, targetRelease *release.Release, req *services.RollbackReleaseRequest) (*services.RollbackReleaseResponse, error) {
	res := &services.RollbackReleaseResponse{Release: targetRelease}

	if req.DryRun {
		log.Printf("Dry run for %s", targetRelease.Name)
		return res, nil
	}

	// pre-rollback hooks
	if !req.DisableHooks {
		if err := s.execHook(targetRelease.Hooks, targetRelease.Name, targetRelease.Namespace, hooks.PreRollback, req.Timeout); err != nil {
			return res, err
		}
	}

	if err := s.performKubeUpdate(currentRelease, targetRelease, req.Recreate, req.Timeout, req.Wait); err != nil {
		msg := fmt.Sprintf("Rollback %q failed: %s", targetRelease.Name, err)
		log.Printf("warning: %s", msg)
		currentRelease.Info.Status.Code = release.Status_SUPERSEDED
		targetRelease.Info.Status.Code = release.Status_FAILED
		targetRelease.Info.Description = msg
		s.recordRelease(currentRelease, true)
		s.recordRelease(targetRelease, false)
		return res, err
	}

	// post-rollback hooks
	if !req.DisableHooks {
		if err := s.execHook(targetRelease.Hooks, targetRelease.Name, targetRelease.Namespace, hooks.PostRollback, req.Timeout); err != nil {
			return res, err
		}
	}

	currentRelease.Info.Status.Code = release.Status_SUPERSEDED
	s.recordRelease(currentRelease, true)

	targetRelease.Info.Status.Code = release.Status_DEPLOYED

	return res, nil
}

func (s *ReleaseServer) performKubeUpdate(currentRelease, targetRelease *release.Release, recreate bool, timeout int64, shouldWait bool) error {
	kubeCli := s.env.KubeClient
	current := bytes.NewBufferString(currentRelease.Manifest)
	target := bytes.NewBufferString(targetRelease.Manifest)
	return kubeCli.Update(targetRelease.Namespace, current, target, recreate, timeout, shouldWait)
}

// prepareRollback finds the previous release and prepares a new release object with
//  the previous release's configuration
func (s *ReleaseServer) prepareRollback(req *services.RollbackReleaseRequest) (*release.Release, *release.Release, error) {
	switch {
	case !ValidName.MatchString(req.Name):
		return nil, nil, errMissingRelease
	case req.Version < 0:
		return nil, nil, errInvalidRevision
	}

	crls, err := s.env.Releases.Last(req.Name)
	if err != nil {
		return nil, nil, err
	}

	rbv := req.Version
	if req.Version == 0 {
		rbv = crls.Version - 1
	}

	log.Printf("rolling back %s (current: v%d, target: v%d)", req.Name, crls.Version, rbv)

	prls, err := s.env.Releases.Get(req.Name, rbv)
	if err != nil {
		return nil, nil, err
	}

	// Store a new release object with previous release's configuration
	target := &release.Release{
		Name:      req.Name,
		Namespace: crls.Namespace,
		Chart:     prls.Chart,
		Config:    prls.Config,
		Info: &release.Info{
			FirstDeployed: crls.Info.FirstDeployed,
			LastDeployed:  timeconv.Now(),
			Status: &release.Status{
				Code:  release.Status_UNKNOWN,
				Notes: prls.Info.Status.Notes,
			},
			// Because we lose the reference to rbv elsewhere, we set the
			// message here, and only override it later if we experience failure.
			Description: fmt.Sprintf("Rollback to %d", rbv),
		},
		Version:  crls.Version + 1,
		Manifest: prls.Manifest,
		Hooks:    prls.Hooks,
	}

	return crls, target, nil
}

func (s *ReleaseServer) uniqName(start string, reuse bool) (string, error) {

	// If a name is supplied, we check to see if that name is taken. If not, it
	// is granted. If reuse is true and a deleted release with that name exists,
	// we re-grant it. Otherwise, an error is returned.
	if start != "" {

		if len(start) > releaseNameMaxLen {
			return "", fmt.Errorf("release name %q exceeds max length of %d", start, releaseNameMaxLen)
		}

		h, err := s.env.Releases.History(start)
		if err != nil || len(h) < 1 {
			return start, nil
		}
		relutil.Reverse(h, relutil.SortByRevision)
		rel := h[0]

		if st := rel.Info.Status.Code; reuse && (st == release.Status_DELETED || st == release.Status_FAILED) {
			// Allowe re-use of names if the previous release is marked deleted.
			log.Printf("reusing name %q", start)
			return start, nil
		} else if reuse {
			return "", errors.New("cannot re-use a name that is still in use")
		}

		return "", fmt.Errorf("a release named %q already exists", start)
	}

	maxTries := 5
	for i := 0; i < maxTries; i++ {
		namer := moniker.New()
		name := namer.NameSep("-")
		if len(name) > releaseNameMaxLen {
			name = name[:releaseNameMaxLen]
		}
		if _, err := s.env.Releases.Get(name, 1); err == driver.ErrReleaseNotFound {
			return name, nil
		}
		log.Printf("info: Name %q is taken. Searching again.", name)
	}
	log.Printf("warning: No available release names found after %d tries", maxTries)
	return "ERROR", errors.New("no available release name found")
}

func (s *ReleaseServer) engine(ch *chart.Chart) environment.Engine {
	renderer := s.env.EngineYard.Default()
	if ch.Metadata.Engine != "" {
		if r, ok := s.env.EngineYard.Get(ch.Metadata.Engine); ok {
			renderer = r
		} else {
			log.Printf("warning: %s requested non-existent template engine %s", ch.Metadata.Name, ch.Metadata.Engine)
		}
	}
	return renderer
}

// InstallRelease installs a release and stores the release record.
func (s *ReleaseServer) InstallRelease(c ctx.Context, req *services.InstallReleaseRequest) (*services.InstallReleaseResponse, error) {
	rel, err := s.prepareRelease(req)
	if err != nil {
		log.Printf("Failed install prepare step: %s", err)
		res := &services.InstallReleaseResponse{Release: rel}

		// On dry run, append the manifest contents to a failed release. This is
		// a stop-gap until we can revisit an error backchannel post-2.0.
		if req.DryRun && strings.HasPrefix(err.Error(), "YAML parse error") {
			err = fmt.Errorf("%s\n%s", err, rel.Manifest)
		}
		return res, err
	}

	res, err := s.performRelease(rel, req)
	if err != nil {
		log.Printf("Failed install perform step: %s", err)
	}
	return res, err
}

// capabilities builds a Capabilities from discovery information.
func capabilities(disc discovery.DiscoveryInterface) (*chartutil.Capabilities, error) {
	sv, err := disc.ServerVersion()
	if err != nil {
		return nil, err
	}
	vs, err := getVersionSet(disc)
	if err != nil {
		return nil, fmt.Errorf("Could not get apiVersions from Kubernetes: %s", err)
	}
	return &chartutil.Capabilities{
		APIVersions:   vs,
		KubeVersion:   sv,
		TillerVersion: version.GetVersionProto(),
	}, nil
}

// prepareRelease builds a release for an install operation.
func (s *ReleaseServer) prepareRelease(req *services.InstallReleaseRequest) (*release.Release, error) {
	if req.Chart == nil {
		return nil, errMissingChart
	}

	name, err := s.uniqName(req.Name, req.ReuseName)
	if err != nil {
		return nil, err
	}

	caps, err := capabilities(s.clientset.Discovery())
	if err != nil {
		return nil, err
	}

	revision := 1
	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{
		Name:      name,
		Time:      ts,
		Namespace: req.Namespace,
		Revision:  revision,
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
				Status:        &release.Status{Code: release.Status_UNKNOWN},
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
			Status:        &release.Status{Code: release.Status_UNKNOWN},
			Description:   "Initial install underway", // Will be overwritten.
		},
		Manifest: manifestDoc.String(),
		Hooks:    hooks,
		Version:  int32(revision),
	}
	if len(notesTxt) > 0 {
		rel.Info.Status.Notes = notesTxt
	}

	err = validateManifest(s.env.KubeClient, req.Namespace, manifestDoc.Bytes())
	return rel, err
}

func getVersionSet(client discovery.ServerGroupsInterface) (chartutil.VersionSet, error) {
	groups, err := client.ServerGroups()
	if err != nil {
		return chartutil.DefaultVersionSet, err
	}

	// FIXME: The Kubernetes test fixture for cli appears to always return nil
	// for calls to Discovery().ServerGroups(). So in this case, we return
	// the default API list. This is also a safe value to return in any other
	// odd-ball case.
	if groups == nil {
		return chartutil.DefaultVersionSet, nil
	}

	versions := unversioned.ExtractGroupVersions(groups)
	return chartutil.NewVersionSet(versions...), nil
}

func (s *ReleaseServer) renderResources(ch *chart.Chart, values chartutil.Values, vs chartutil.VersionSet) ([]*release.Hook, *bytes.Buffer, string, error) {
	renderer := s.engine(ch)
	files, err := renderer.Render(ch, values)
	if err != nil {
		return nil, nil, "", err
	}

	// NOTES.txt gets rendered like all the other files, but because it's not a hook nor a resource,
	// pull it out of here into a separate file so that we can actually use the output of the rendered
	// text file. We have to spin through this map because the file contains path information, so we
	// look for terminating NOTES.txt. We also remove it from the files so that we don't have to skip
	// it in the sortHooks.
	notes := ""
	for k, v := range files {
		if strings.HasSuffix(k, notesFileSuffix) {
			// Only apply the notes if it belongs to the parent chart
			// Note: Do not use filePath.Join since it creates a path with \ which is not expected
			if k == path.Join(ch.Metadata.Name, "templates", notesFileSuffix) {
				notes = v
			}
			delete(files, k)
		}
	}

	// Sort hooks, manifests, and partials. Only hooks and manifests are returned,
	// as partials are not used after renderer.Render. Empty manifests are also
	// removed here.
	hooks, manifests, err := sortManifests(files, vs, InstallOrder)
	if err != nil {
		// By catching parse errors here, we can prevent bogus releases from going
		// to Kubernetes.
		//
		// We return the files as a big blob of data to help the user debug parser
		// errors.
		b := bytes.NewBuffer(nil)
		for name, content := range files {
			if len(strings.TrimSpace(content)) == 0 {
				continue
			}
			b.WriteString("\n---\n# Source: " + name + "\n")
			b.WriteString(content)
		}
		return nil, b, "", err
	}

	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)
	for _, m := range manifests {
		b.WriteString("\n---\n# Source: " + m.name + "\n")
		b.WriteString(m.content)
	}

	return hooks, b, notes, nil
}

func (s *ReleaseServer) recordRelease(r *release.Release, reuse bool) {
	if reuse {
		if err := s.env.Releases.Update(r); err != nil {
			log.Printf("warning: Failed to update release %q: %s", r.Name, err)
		}
	} else if err := s.env.Releases.Create(r); err != nil {
		log.Printf("warning: Failed to record release %q: %s", r.Name, err)
	}
}

// performRelease runs a release.
func (s *ReleaseServer) performRelease(r *release.Release, req *services.InstallReleaseRequest) (*services.InstallReleaseResponse, error) {
	res := &services.InstallReleaseResponse{Release: r}

	if req.DryRun {
		log.Printf("Dry run for %s", r.Name)
		res.Release.Info.Description = "Dry run complete"
		return res, nil
	}

	// pre-install hooks
	if !req.DisableHooks {
		if err := s.execHook(r.Hooks, r.Name, r.Namespace, hooks.PreInstall, req.Timeout); err != nil {
			return res, err
		}
	}

	switch h, err := s.env.Releases.History(req.Name); {
	// if this is a replace operation, append to the release history
	case req.ReuseName && err == nil && len(h) >= 1:
		// get latest release revision
		relutil.Reverse(h, relutil.SortByRevision)

		// old release
		old := h[0]

		// update old release status
		old.Info.Status.Code = release.Status_SUPERSEDED
		s.recordRelease(old, true)

		// update new release with next revision number
		// so as to append to the old release's history
		r.Version = old.Version + 1

		if err := s.performKubeUpdate(old, r, false, req.Timeout, req.Wait); err != nil {
			msg := fmt.Sprintf("Release replace %q failed: %s", r.Name, err)
			log.Printf("warning: %s", msg)
			old.Info.Status.Code = release.Status_SUPERSEDED
			r.Info.Status.Code = release.Status_FAILED
			r.Info.Description = msg
			s.recordRelease(old, true)
			s.recordRelease(r, false)
			return res, err
		}

	default:
		// nothing to replace, create as normal
		// regular manifests
		b := bytes.NewBufferString(r.Manifest)
		if err := s.env.KubeClient.Create(r.Namespace, b, req.Timeout, req.Wait); err != nil {
			msg := fmt.Sprintf("Release %q failed: %s", r.Name, err)
			log.Printf("warning: %s", msg)
			r.Info.Status.Code = release.Status_FAILED
			r.Info.Description = msg
			s.recordRelease(r, false)
			return res, fmt.Errorf("release %s failed: %s", r.Name, err)
		}
	}

	// post-install hooks
	if !req.DisableHooks {
		if err := s.execHook(r.Hooks, r.Name, r.Namespace, hooks.PostInstall, req.Timeout); err != nil {
			msg := fmt.Sprintf("Release %q failed post-install: %s", r.Name, err)
			log.Printf("warning: %s", msg)
			r.Info.Status.Code = release.Status_FAILED
			r.Info.Description = msg
			s.recordRelease(r, false)
			return res, err
		}
	}

	r.Info.Status.Code = release.Status_DEPLOYED
	r.Info.Description = "Install complete"
	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	s.recordRelease(r, false)

	return res, nil
}

func (s *ReleaseServer) execHook(hs []*release.Hook, name, namespace, hook string, timeout int64) error {
	kubeCli := s.env.KubeClient
	code, ok := events[hook]
	if !ok {
		return fmt.Errorf("unknown hook %q", hook)
	}

	log.Printf("Executing %s hooks for %s", hook, name)
	for _, h := range hs {
		found := false
		for _, e := range h.Events {
			if e == code {
				found = true
			}
		}
		// If this doesn't implement the hook, skip it.
		if !found {
			continue
		}

		b := bytes.NewBufferString(h.Manifest)
		if err := kubeCli.Create(namespace, b, timeout, false); err != nil {
			log.Printf("warning: Release %q %s %s failed: %s", name, hook, h.Path, err)
			return err
		}
		// No way to rewind a bytes.Buffer()?
		b.Reset()
		b.WriteString(h.Manifest)
		if err := kubeCli.WatchUntilReady(namespace, b, timeout, false); err != nil {
			log.Printf("warning: Release %q %s %s could not complete: %s", name, hook, h.Path, err)
			return err
		}
		h.LastRun = timeconv.Now()
	}
	log.Printf("Hooks complete for %s %s", hook, name)
	return nil
}

func (s *ReleaseServer) purgeReleases(rels ...*release.Release) error {
	for _, rel := range rels {
		if _, err := s.env.Releases.Delete(rel.Name, rel.Version); err != nil {
			return err
		}
	}
	return nil
}

// UninstallRelease deletes all of the resources associated with this release, and marks the release DELETED.
func (s *ReleaseServer) UninstallRelease(c ctx.Context, req *services.UninstallReleaseRequest) (*services.UninstallReleaseResponse, error) {
	if !ValidName.MatchString(req.Name) {
		log.Printf("uninstall: Release not found: %s", req.Name)
		return nil, errMissingRelease
	}

	rels, err := s.env.Releases.History(req.Name)
	if err != nil {
		log.Printf("uninstall: Release not loaded: %s", req.Name)
		return nil, err
	}
	if len(rels) < 1 {
		return nil, errMissingRelease
	}

	relutil.SortByRevision(rels)
	rel := rels[len(rels)-1]

	// TODO: Are there any cases where we want to force a delete even if it's
	// already marked deleted?
	if rel.Info.Status.Code == release.Status_DELETED {
		if req.Purge {
			if err := s.purgeReleases(rels...); err != nil {
				log.Printf("uninstall: Failed to purge the release: %s", err)
				return nil, err
			}
			return &services.UninstallReleaseResponse{Release: rel}, nil
		}
		return nil, fmt.Errorf("the release named %q is already deleted", req.Name)
	}

	log.Printf("uninstall: Deleting %s", req.Name)
	rel.Info.Status.Code = release.Status_DELETING
	rel.Info.Deleted = timeconv.Now()
	rel.Info.Description = "Deletion in progress (or silently failed)"
	res := &services.UninstallReleaseResponse{Release: rel}

	if !req.DisableHooks {
		if err := s.execHook(rel.Hooks, rel.Name, rel.Namespace, hooks.PreDelete, req.Timeout); err != nil {
			return res, err
		}
	}

	vs, err := getVersionSet(s.clientset.Discovery())
	if err != nil {
		return nil, fmt.Errorf("Could not get apiVersions from Kubernetes: %s", err)
	}

	// From here on out, the release is currently considered to be in Status_DELETING
	// state.
	if err := s.env.Releases.Update(rel); err != nil {
		log.Printf("uninstall: Failed to store updated release: %s", err)
	}

	manifests := relutil.SplitManifests(rel.Manifest)
	_, files, err := sortManifests(manifests, vs, UninstallOrder)
	if err != nil {
		// We could instead just delete everything in no particular order.
		// FIXME: One way to delete at this point would be to try a label-based
		// deletion. The problem with this is that we could get a false positive
		// and delete something that was not legitimately part of this release.
		return nil, fmt.Errorf("corrupted release record. You must manually delete the resources: %s", err)
	}

	filesToKeep, filesToDelete := filterManifestsToKeep(files)
	if len(filesToKeep) > 0 {
		res.Info = summarizeKeptManifests(filesToKeep)
	}

	// Collect the errors, and return them later.
	es := []string{}
	for _, file := range filesToDelete {
		b := bytes.NewBufferString(file.content)
		if err := s.env.KubeClient.Delete(rel.Namespace, b); err != nil {
			log.Printf("uninstall: Failed deletion of %q: %s", req.Name, err)
			if err == kube.ErrNoObjectsVisited {
				// Rewrite the message from "no objects visited"
				err = errors.New("object not found, skipping delete")
			}
			es = append(es, err.Error())
		}
	}

	if !req.DisableHooks {
		if err := s.execHook(rel.Hooks, rel.Name, rel.Namespace, hooks.PostDelete, req.Timeout); err != nil {
			es = append(es, err.Error())
		}
	}

	rel.Info.Status.Code = release.Status_DELETED
	rel.Info.Description = "Deletion complete"

	if req.Purge {
		err := s.purgeReleases(rels...)
		if err != nil {
			log.Printf("uninstall: Failed to purge the release: %s", err)
		}
		return res, err
	}

	if err := s.env.Releases.Update(rel); err != nil {
		log.Printf("uninstall: Failed to store updated release: %s", err)
	}

	if len(es) > 0 {
		return res, fmt.Errorf("deletion completed with %d error(s): %s", len(es), strings.Join(es, "; "))
	}
	return res, nil
}

func validateManifest(c environment.KubeClient, ns string, manifest []byte) error {
	r := bytes.NewReader(manifest)
	_, err := c.BuildUnstructured(ns, r)
	return err
}

// RunReleaseTest runs pre-defined tests stored as hooks on a given release
func (s *ReleaseServer) RunReleaseTest(req *services.TestReleaseRequest, stream services.ReleaseService_RunReleaseTestServer) error {

	if !ValidName.MatchString(req.Name) {
		return errMissingRelease
	}

	// finds the non-deleted release with the given name
	rel, err := s.env.Releases.Last(req.Name)
	if err != nil {
		return err
	}

	testEnv := &reltesting.Environment{
		Namespace:  rel.Namespace,
		KubeClient: s.env.KubeClient,
		Timeout:    req.Timeout,
		Stream:     stream,
	}

	tSuite, err := reltesting.NewTestSuite(rel)
	if err != nil {
		log.Printf("Error creating test suite for %s", rel.Name)
		return err
	}

	if err := tSuite.Run(testEnv); err != nil {
		log.Printf("Error running test suite for %s", rel.Name)
		return err
	}

	rel.Info.Status.LastTestSuiteRun = &release.TestSuite{
		StartedAt:   tSuite.StartedAt,
		CompletedAt: tSuite.CompletedAt,
		Results:     tSuite.Results,
	}

	if req.Cleanup {
		testEnv.DeleteTestPods(tSuite.TestManifests)
	}

	return s.env.Releases.Update(rel)
}
