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
package chartutil

import (
	"sort"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestLoadRequirements(t *testing.T) {
	c, err := Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyRequirements(t, c)
}

func TestLoadRequirementsLock(t *testing.T) {
	c, err := Load("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyRequirementsLock(t, c)
}
func TestRequirementsTagsNonValue(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags with no effect
	v := &chart.Config{Raw: "tags:\n  nothinguseful: false\n\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1", "subcharta", "subchartb"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsTagsDisabledL1(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags disabling a group
	v := &chart.Config{Raw: "tags:\n  front-end: false\n\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsTagsEnabledL1(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags disabling a group and enabling a different group
	v := &chart.Config{Raw: "tags:\n  front-end: false\n\n  back-end: true\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart2", "subchartb", "subchartc"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsTagsDisabledL2(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags disabling only children
	v := &chart.Config{Raw: "tags:\n  subcharta: false\n\n  subchartb: false\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsTagsDisabledL1Mixed(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags disabling all parents/children with additional tag re-enabling a parent
	v := &chart.Config{Raw: "tags:\n  front-end: false\n\n  subchart1: true\n\n  back-end: false\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsConditionsNonValue(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags with no effect
	v := &chart.Config{Raw: "subchart1:\n  nothinguseful: false\n\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1", "subcharta", "subchartb"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsConditionsEnabledL1Both(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// conditions enabling the parent charts, effectively enabling children
	v := &chart.Config{Raw: "subchart1:\n  enabled: true\nsubchart2:\n  enabled: true\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1", "subchart2", "subcharta", "subchartb", "subchartb", "subchartc"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsConditionsDisabledL1Both(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// conditions disabling the parent charts, effectively disabling children
	v := &chart.Config{Raw: "subchart1:\n  enabled: false\nsubchart2:\n  enabled: false\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart"}

	verifyRequirementsEnabled(t, c, v, e)
}

func TestRequirementsConditionsSecond(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// conditions a child using the second condition path of child's condition
	v := &chart.Config{Raw: "subchart1:\n  subcharta:\n    enabled: false\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1", "subchartb"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsCombinedDisabledL2(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags enabling a parent/child group with condition disabling one child
	v := &chart.Config{Raw: "subchartc:\n  enabled: false\ntags:\n  back-end: true\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart", "subchart1", "subchart2", "subcharta", "subchartb", "subchartb"}

	verifyRequirementsEnabled(t, c, v, e)
}
func TestRequirementsCombinedDisabledL1(t *testing.T) {
	c, err := Load("testdata/subpop")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	// tags will not enable a child if parent is explicitly disabled with condition
	v := &chart.Config{Raw: "subchart1:\n  enabled: false\ntags:\n  front-end: true\n"}
	// expected charts including duplicates in alphanumeric order
	e := []string{"parentchart"}

	verifyRequirementsEnabled(t, c, v, e)
}

func verifyRequirementsEnabled(t *testing.T, c *chart.Chart, v *chart.Config, e []string) {
	out := []*chart.Chart{}
	err := ProcessRequirementsEnabled(c, v)
	if err != nil {
		t.Errorf("Error processing enabled requirements %v", err)
	}
	out = extractCharts(c, out)
	// build list of chart names
	p := []string{}
	for _, r := range out {
		p = append(p, r.Metadata.Name)
	}
	//sort alphanumeric and compare to expectations
	sort.Strings(p)
	if len(p) != len(e) {
		t.Errorf("Error slice lengths do not match got %v, expected %v", len(p), len(e))
		return
	}
	for i := range p {
		if p[i] != e[i] {
			t.Errorf("Error slice values do not match got %v, expected %v", p[i], e[i])
		}
	}
}

// extractCharts recursively searches chart dependencies returning all charts found
func extractCharts(c *chart.Chart, out []*chart.Chart) []*chart.Chart {

	if len(c.Metadata.Name) > 0 {
		out = append(out, c)
	}
	for _, d := range c.Dependencies {
		out = extractCharts(d, out)
	}
	return out
}
