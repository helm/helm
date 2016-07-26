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
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	"github.com/technosophos/moniker"
	ctx "golang.org/x/net/context"

	"k8s.io/helm/cmd/tiller/environment"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/timeconv"
)

var srv *releaseServer

func init() {
	srv = &releaseServer{
		env: env,
	}
	services.RegisterReleaseServiceServer(rootServer, srv)
}

var (
	// errMissingChart indicates that a chart was not provided.
	errMissingChart = errors.New("no chart provided")
	// errMissingRelease indicates that a release (name) was not provided.
	errMissingRelease = errors.New("no release provided")
)

// ListDefaultLimit is the default limit for number of items returned in a list.
var ListDefaultLimit int64 = 512

type releaseServer struct {
	env *environment.Environment
}

func (s *releaseServer) ListReleases(req *services.ListReleasesRequest, stream services.ReleaseService_ListReleasesServer) error {
	rels, err := s.env.Releases.List()
	if err != nil {
		return err
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
		sort.Sort(byName(rels))
	case services.ListSort_LAST_RELEASED:
		sort.Sort(byDate(rels))
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
	stream.Send(res)
	return nil
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

func (s *releaseServer) GetReleaseStatus(c ctx.Context, req *services.GetReleaseStatusRequest) (*services.GetReleaseStatusResponse, error) {
	if req.Name == "" {
		return nil, errMissingRelease
	}
	rel, err := s.env.Releases.Read(req.Name)
	if err != nil {
		return nil, err
	}
	if rel.Info == nil {
		return nil, errors.New("release info is missing")
	}
	return &services.GetReleaseStatusResponse{Info: rel.Info}, nil
}

func (s *releaseServer) GetReleaseContent(c ctx.Context, req *services.GetReleaseContentRequest) (*services.GetReleaseContentResponse, error) {
	if req.Name == "" {
		return nil, errMissingRelease
	}
	rel, err := s.env.Releases.Read(req.Name)
	return &services.GetReleaseContentResponse{Release: rel}, err
}

func (s *releaseServer) UpdateRelease(c ctx.Context, req *services.UpdateReleaseRequest) (*services.UpdateReleaseResponse, error) {
	rel, err := s.prepareUpdate(req)
	if err != nil {
		return nil, err
	}

	// TODO: perform update

	return &services.UpdateReleaseResponse{Release: rel}, nil
}

// prepareUpdate builds a release for an update operation.
func (s *releaseServer) prepareUpdate(req *services.UpdateReleaseRequest) (*release.Release, error) {
	if req.Name == "" {
		return nil, errMissingRelease
	}

	if req.Chart == nil {
		return nil, errMissingChart
	}

	// finds the non-deleted release with the given name
	rel, err := s.env.Releases.Read(req.Name)
	if err != nil {
		return nil, err
	}

	//validate chart name is same as previous release
	givenChart := req.Chart.Metadata.Name
	releasedChart := rel.Chart.Metadata.Name
	if givenChart != releasedChart {
		return nil, fmt.Errorf("Given chart, %s, does not match chart originally released, %s", givenChart, releasedChart)
	}

	// validate new chart version is higher than old

	givenChartVersion := req.Chart.Metadata.Version
	releasedChartVersion := rel.Chart.Metadata.Version
	c, err := semver.NewConstraint("> " + releasedChartVersion)
	if err != nil {
		return nil, err
	}

	v, err := semver.NewVersion(givenChartVersion)
	if err != nil {
		return nil, err
	}

	if a := c.Check(v); !a {
		return nil, fmt.Errorf("Given chart (%s-%v) must be a higher version than released chart (%s-%v)", givenChart, givenChartVersion, releasedChart, releasedChartVersion)
	}

	// Store an updated release.
	updatedRelease := &release.Release{
		Name:    req.Name,
		Chart:   req.Chart,
		Config:  req.Values,
		Version: rel.Version + 1,
	}
	return updatedRelease, nil
}

func (s *releaseServer) uniqName(start string) (string, error) {

	// If a name is supplied, we check to see if that name is taken. Right now,
	// we fail if it is already taken. We could instead fall-thru and allow
	// an automatically generated name, but this seems to violate the principle
	// of least surprise.
	if start != "" {
		if _, err := s.env.Releases.Read(start); err == storage.ErrNotFound {
			return start, nil
		}
		return "", fmt.Errorf("a release named %q already exists", start)
	}

	maxTries := 5
	for i := 0; i < maxTries; i++ {
		namer := moniker.New()
		name := namer.NameSep("-")
		if _, err := s.env.Releases.Read(name); err == storage.ErrNotFound {
			return name, nil
		}
		log.Printf("info: Name %q is taken. Searching again.", name)
	}
	log.Printf("warning: No available release names found after %d tries", maxTries)
	return "ERROR", errors.New("no available release name found")
}

func (s *releaseServer) engine(ch *chart.Chart) environment.Engine {
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

func (s *releaseServer) InstallRelease(c ctx.Context, req *services.InstallReleaseRequest) (*services.InstallReleaseResponse, error) {
	rel, err := s.prepareRelease(req)
	if err != nil {
		log.Printf("Failed install prepare step: %s", err)
		return nil, err
	}

	res, err := s.performRelease(rel, req)
	if err != nil {
		log.Printf("Failed install perform step: %s", err)
	}
	return res, err
}

// prepareRelease builds a release for an install operation.
func (s *releaseServer) prepareRelease(req *services.InstallReleaseRequest) (*release.Release, error) {
	if req.Chart == nil {
		return nil, errMissingChart
	}

	name, err := s.uniqName(req.Name)
	if err != nil {
		return nil, err
	}

	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{Name: name, Time: ts, Namespace: req.Namespace}
	valuesToRender, err := chartutil.ToRenderValues(req.Chart, req.Values, options)
	if err != nil {
		return nil, err
	}

	renderer := s.engine(req.Chart)
	files, err := renderer.Render(req.Chart, valuesToRender)
	if err != nil {
		return nil, err
	}

	// Sort hooks, manifests, and partials. Only hooks and manifests are returned,
	// as partials are not used after renderer.Render. Empty manifests are also
	// removed here.
	hooks, manifests, err := sortHooks(files)
	if err != nil {
		// By catching parse errors here, we can prevent bogus releases from going
		// to Kubernetes.
		return nil, err
	}

	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)
	for name, file := range manifests {
		b.WriteString("\n---\n# Source: " + name + "\n")
		b.WriteString(file)
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
		},
		Manifest: b.String(),
		Hooks:    hooks,
		Version:  1,
	}
	return rel, nil
}

// validateYAML checks to see if YAML is well-formed.
func validateYAML(data string) error {
	b := map[string]interface{}{}
	return yaml.Unmarshal([]byte(data), b)
}

// performRelease runs a release.
func (s *releaseServer) performRelease(r *release.Release, req *services.InstallReleaseRequest) (*services.InstallReleaseResponse, error) {
	res := &services.InstallReleaseResponse{Release: r}

	if req.DryRun {
		log.Printf("Dry run for %s", r.Name)
		return res, nil
	}

	// pre-install hooks
	if !req.DisableHooks {
		if err := s.execHook(r.Hooks, r.Name, r.Namespace, preInstall); err != nil {
			return res, err
		}
	}

	// regular manifests
	kubeCli := s.env.KubeClient
	b := bytes.NewBufferString(r.Manifest)
	if err := kubeCli.Create(r.Namespace, b); err != nil {
		r.Info.Status.Code = release.Status_FAILED
		log.Printf("warning: Release %q failed: %s", r.Name, err)
		if err := s.env.Releases.Create(r); err != nil {
			log.Printf("warning: Failed to record release %q: %s", r.Name, err)
		}
		return res, fmt.Errorf("release %s failed: %s", r.Name, err)
	}

	// post-install hooks
	if !req.DisableHooks {
		if err := s.execHook(r.Hooks, r.Name, r.Namespace, postInstall); err != nil {
			return res, err
		}
	}

	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	r.Info.Status.Code = release.Status_DEPLOYED
	if err := s.env.Releases.Create(r); err != nil {
		log.Printf("warning: Failed to record release %q: %s", r.Name, err)
	}
	return res, nil
}

func (s *releaseServer) execHook(hs []*release.Hook, name, namespace, hook string) error {
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
		if err := kubeCli.Create(namespace, b); err != nil {
			log.Printf("wrning: Release %q pre-install %s failed: %s", name, h.Path, err)
			return err
		}
		// No way to rewind a bytes.Buffer()?
		b.Reset()
		b.WriteString(h.Manifest)
		if err := kubeCli.WatchUntilReady(namespace, b); err != nil {
			log.Printf("warning: Release %q pre-install %s could not complete: %s", name, h.Path, err)
			return err
		}
		h.LastRun = timeconv.Now()
	}
	log.Printf("Hooks complete for %s %s", hook, name)
	return nil
}

func (s *releaseServer) UninstallRelease(c ctx.Context, req *services.UninstallReleaseRequest) (*services.UninstallReleaseResponse, error) {
	if req.Name == "" {
		log.Printf("uninstall: Release not found: %s", req.Name)
		return nil, errMissingRelease
	}

	rel, err := s.env.Releases.Read(req.Name)
	if err != nil {
		log.Printf("uninstall: Release not loaded: %s", req.Name)
		return nil, err
	}

	// TODO: Are there any cases where we want to force a delete even if it's
	// already marked deleted?
	if rel.Info.Status.Code == release.Status_DELETED {
		return nil, fmt.Errorf("the release named %q is already deleted", req.Name)
	}

	log.Printf("uninstall: Deleting %s", req.Name)
	rel.Info.Status.Code = release.Status_DELETED
	rel.Info.Deleted = timeconv.Now()
	res := &services.UninstallReleaseResponse{Release: rel}

	if !req.DisableHooks {
		if err := s.execHook(rel.Hooks, rel.Name, rel.Namespace, preDelete); err != nil {
			return res, err
		}
	}

	b := bytes.NewBuffer([]byte(rel.Manifest))
	if err := s.env.KubeClient.Delete(rel.Namespace, b); err != nil {
		log.Printf("uninstall: Failed deletion of %q: %s", req.Name, err)
		return nil, err
	}

	if !req.DisableHooks {
		if err := s.execHook(rel.Hooks, rel.Name, rel.Namespace, postDelete); err != nil {
			return res, err
		}
	}

	if err := s.env.Releases.Update(rel); err != nil {
		log.Printf("uninstall: Failed to store updated release: %s", err)
	}

	return res, nil
}

// byName implements the sort.Interface for []*release.Release.
type byName []*release.Release

func (r byName) Len() int {
	return len(r)
}
func (r byName) Swap(p, q int) {
	r[p], r[q] = r[q], r[p]
}
func (r byName) Less(i, j int) bool {
	return r[i].Name < r[j].Name
}

type byDate []*release.Release

func (r byDate) Len() int { return len(r) }
func (r byDate) Swap(p, q int) {
	r[p], r[q] = r[q], r[p]
}
func (r byDate) Less(p, q int) bool {
	return r[p].Info.LastDeployed.Seconds < r[q].Info.LastDeployed.Seconds
}
