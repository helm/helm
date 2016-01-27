package chart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/helm/helm/manifest"
)

// Chart represents a complete chart.
//
// A chart consists of the following parts:
//
// 	- Chart.yaml: In code, we refer to this as the Chartfile
// 	- manifests/*.yaml: The Kubernetes manifests
//
// On the Chart object, the manifests are sorted by type into a handful of
// recognized Kubernetes API v1 objects.
//
// TODO: Investigate treating these as unversioned.
type Chart struct {
	Chartfile *Chartfile

	// Kind is a map of Kind to an array of manifests.
	//
	// For example, Kind["Pod"] has an array of Pod manifests.
	Kind map[string][]*manifest.Manifest

	// Manifests is an array of Manifest objects.
	Manifests []*manifest.Manifest
}

// Load loads an entire chart.
//
// This includes the Chart.yaml (*Chartfile) and all of the manifests.
//
// If you are just reading the Chart.yaml file, it is substantially more
// performant to use LoadChartfile.
func Load(chart string) (*Chart, error) {
	if fi, err := os.Stat(chart); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("Chart %s is not a directory.", chart)
	}

	cf, err := LoadChartfile(filepath.Join(chart, "Chart.yaml"))
	if err != nil {
		return nil, err
	}

	c := &Chart{
		Chartfile: cf,
		Kind:      map[string][]*manifest.Manifest{},
	}

	ms, err := manifest.ParseDir(chart)
	if err != nil {
		return c, err
	}

	c.attachManifests(ms)

	return c, nil
}

const (
	// AnnFile is the annotation key for a file's origin.
	AnnFile = "chart.helm.sh/file"

	// AnnChartVersion is the annotation key for a chart's version.
	AnnChartVersion = "chart.helm.sh/version"

	// AnnChartDesc is the annotation key for a chart's description.
	AnnChartDesc = "chart.helm.sh/description"

	// AnnChartName is the annotation key for a chart name.
	AnnChartName = "chart.helm.sh/name"
)

// attachManifests sorts manifests into their respective categories, adding to the Chart.
func (c *Chart) attachManifests(manifests []*manifest.Manifest) {
	c.Manifests = manifests
	for _, m := range manifests {
		c.Kind[m.Kind] = append(c.Kind[m.Kind], m)
	}
}

// UnknownKinds returns a list of kinds that this chart contains, but which were not in the passed in array.
//
// A Chart will store all kinds that are given to it. This makes it possible to get a list of kinds that are not
// known beforehand.
func (c *Chart) UnknownKinds(known []string) []string {
	lookup := make(map[string]bool, len(known))
	for _, k := range known {
		lookup[k] = true
	}

	u := []string{}
	for n := range c.Kind {
		if _, ok := lookup[n]; !ok {
			u = append(u, n)
		}
	}

	return u
}
