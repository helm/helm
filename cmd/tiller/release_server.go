package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"

	"github.com/kubernetes/helm/cmd/tiller/environment"
	"github.com/kubernetes/helm/pkg/proto/hapi/release"
	"github.com/kubernetes/helm/pkg/proto/hapi/services"
	"github.com/kubernetes/helm/pkg/storage"
	"github.com/kubernetes/helm/pkg/timeconv"
	"github.com/technosophos/moniker"
	ctx "golang.org/x/net/context"
)

var srv *releaseServer

func init() {
	srv = &releaseServer{
		env: env,
	}
	srv.env.Namespace = namespace
	services.RegisterReleaseServiceServer(rootServer, srv)
}

type releaseServer struct {
	env *environment.Environment
}

var (
	// errNotImplemented is a temporary error for uninmplemented callbacks.
	errNotImplemented = errors.New("not implemented")
	// errMissingChart indicates that a chart was not provided.
	errMissingChart = errors.New("no chart provided")
	// errMissingRelease indicates that a release (name) was not provided.
	errMissingRelease = errors.New("no release provided")
)

// ListDefaultLimit is the default limit for number of items returned in a list.
var ListDefaultLimit int64 = 512

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
	return nil, errNotImplemented
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

func (s *releaseServer) InstallRelease(c ctx.Context, req *services.InstallReleaseRequest) (*services.InstallReleaseResponse, error) {
	if req.Chart == nil {
		return nil, errMissingChart
	}

	ts := timeconv.Now()
	name, err := s.uniqName(req.Name)
	if err != nil {
		return nil, err
	}

	overrides := map[string]interface{}{
		"Release": map[string]interface{}{
			"Name":      name,
			"Time":      ts,
			"Namespace": s.env.Namespace,
			"Service":   "Tiller",
		},
		"Chart": req.Chart.Metadata,
	}

	// Render the templates
	// TODO: Fix based on whether chart has `engine: SOMETHING` set.
	files, err := s.env.EngineYard.Default().Render(req.Chart, req.Values, overrides)
	if err != nil {
		return nil, err
	}

	b := bytes.NewBuffer(nil)
	for name, file := range files {
		// Ignore empty documents because the Kubernetes library can't handle
		// them.
		if len(file) > 0 {
			b.WriteString("\n---\n# Source: " + name + "\n")
			b.WriteString(file)
		}
	}

	// Store a release.
	r := &release.Release{
		Name:   name,
		Chart:  req.Chart,
		Config: req.Values,
		Info: &release.Info{
			FirstDeployed: ts,
			LastDeployed:  ts,
			Status:        &release.Status{Code: release.Status_UNKNOWN},
		},
		Manifest: b.String(),
	}

	res := &services.InstallReleaseResponse{Release: r}

	if req.DryRun {
		log.Printf("Dry run for %s", name)
		return res, nil
	}

	if err := s.env.KubeClient.Create(s.env.Namespace, b); err != nil {
		r.Info.Status.Code = release.Status_FAILED
		log.Printf("warning: Release %q failed: %s", name, err)
		return res, fmt.Errorf("release %s failed: %s", name, err)
	}

	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	if err := s.env.Releases.Create(r); err != nil {
		log.Printf("warning: Failed to record release %q: %s", name, err)
	}
	return res, nil
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

	log.Printf("uninstall: Deleting %s", req.Name)
	rel.Info.Status.Code = release.Status_DELETED
	rel.Info.Deleted = timeconv.Now()

	b := bytes.NewBuffer([]byte(rel.Manifest))

	if err := s.env.KubeClient.Delete(s.env.Namespace, b); err != nil {
		log.Printf("uninstall: Failed deletion of %q: %s", req.Name, err)
		return nil, err
	}

	if err := s.env.Releases.Update(rel); err != nil {
		log.Printf("uninstall: Failed to store updated release: %s", err)
	}

	res := services.UninstallReleaseResponse{Release: rel}
	return &res, nil
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
