/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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

package main // import "k8s.io/helm/cmd/rudder"

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/Mirantis/k8s-appcontroller/cmd/format"
	"github.com/technosophos/moniker"
	"google.golang.org/grpc/grpclog"

	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/releaseutil"
)

type Dependency struct {
	Name   string
	Child  string
	Parent string
}

func (d Dependency) String() string {
	return fmt.Sprintf(`apiVersion: appcontroller.k8s/v1alpha1
kind: Dependency
metadata:
  name: %s
parent: %s
child: %s`, d.Name, d.Parent, d.Child)
}

func wrapManifest(r *release.Release) (*bytes.Buffer, error) {
	b := bytes.NewBuffer(nil)
	var err error
	sep := "\n---\n"

	manifests, err := releaseutil.SplitManifestsWithHeads(r.Manifest)
	if err != nil {
		grpclog.Printf("Could not split manifest")
		return nil, err
	}

	for _, m := range manifests {
		grpclog.Printf("Trying to wrap %s", m.Metadata.Name)
		wrapped, err := format.Yaml{}.Wrap(getInput(m.Content, 2))
		if err != nil {
			grpclog.Printf("didn't wrap %s: %s", m.Metadata.Name, err)
			//TODO: should we return b here?
			return nil, err
		}
		grpclog.Printf("wrapped %s", m.Metadata.Name)
		b.WriteString(wrapped)
		b.WriteString(sep)
	}

	deps := getDependencies(r.Chart, manifests)

	for _, d := range deps {
		b.WriteString(d.String())
		b.WriteString(sep)
	}

	return b, err
}

func getInput(in string, indent int) string {
	spaces := strings.Repeat(" ", indent)
	result := spaces + strings.Replace(in, "\n", "\n"+spaces, -1)
	return result

}

func getDependencies(ch *chart.Chart, manifests []releaseutil.Manifest) []Dependency {
	dependencies := []Dependency{}

	for _, dep := range ch.Dependencies {
		dependencies = append(dependencies, getDependencies(dep, manifests)...)
		dependencies = append(dependencies, getInterChartDependencies(ch, dep, manifests)...)
	}

	return dependencies
}

func getInterChartDependencies(child, parent *chart.Chart, manifests []releaseutil.Manifest) []Dependency {
	parentNames := []string{}
	childNames := []string{}
	for _, m := range manifests {
		kind := m.Kind
		name := m.Metadata.Name
		grpclog.Printf("comparing %s and %s with %s/%s", child.Metadata.Name, parent.Metadata.Name, kind, name)
		//so hacky I'm gonna cry. need to find a better way
		//I had to remove first segment of helm release name for this to work (chart names were truncated from release name)
		if strings.Contains(name, parent.Metadata.Name) {
			grpclog.Printf("adding %s to parents", strings.ToLower(kind)+"/"+name)
			parentNames = append(parentNames, strings.ToLower(kind)+"/"+name)
		}
		if strings.Contains(name, child.Metadata.Name) {
			grpclog.Printf("adding %s to children", strings.ToLower(kind)+"/"+name)
			childNames = append(childNames, strings.ToLower(kind)+"/"+name)
		}
	}

	deps := make([]Dependency, 0, len(parentNames)*len(childNames))

	for _, parent := range parentNames {
		for _, child := range childNames {
			namer := moniker.New()
			deps = append(deps, Dependency{
				Name:   namer.NameSep("-"),
				Child:  child,
				Parent: parent,
			})
		}
	}
	return deps
}
