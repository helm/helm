/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package chart

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/kubernetes/deployment-manager/log"
)

const (
	testfile    = "testdata/frobnitz/Chart.yaml"
	testdir     = "testdata/frobnitz/"
	testarchive = "testdata/frobnitz-0.0.1.tgz"
	testill     = "testdata/ill-1.2.3.tgz"
	testnochart = "testdata/nochart.tgz"
)

// Type canaries. If these fail, they will fail at compile time.
var _ chartLoader = &dirChart{}
var _ chartLoader = &tarChart{}

func init() {
	log.IsDebugging = true
}

func TestLoadDir(t *testing.T) {

	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected chart name to be 'frobnitz'. Got '%s'.", c.Chartfile().Name)
	}

	if c.Chartfile().Dependencies[0].Version != "^3" {
		d := c.Chartfile().Dependencies[0].Version
		t.Errorf("Expected dependency 0 to have version '^3'. Got '%s'.", d)
	}
}

func TestLoad(t *testing.T) {
	c, err := Load(testarchive)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	defer c.Close()

	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected name to be frobnitz, got %q", c.Chartfile().Name)
	}
}

func TestLoadData(t *testing.T) {
	data, err := ioutil.ReadFile(testarchive)
	if err != nil {
		t.Errorf("Failed to read testarchive file: %s", err)
		return
	}
	c, err := LoadData(data)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected name to be frobnitz, got %q", c.Chartfile().Name)
	}
}

func TestLoadIll(t *testing.T) {
	c, err := Load(testill)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	defer c.Close()

	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	// Ill does not have an icon.
	if i, err := c.Icon(); err == nil {
		t.Errorf("Expected icon to be missing. Got %s", i)
	}
}

func TestLoadNochart(t *testing.T) {
	_, err := Load(testnochart)
	if err == nil {
		t.Error("Nochart should not have loaded at all.")
	}
}

func TestChart(t *testing.T) {
	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}
	defer c.Close()

	if c.Dir() != c.loader.dir() {
		t.Errorf("Unexpected location for directory: %s", c.Dir())
	}

	if c.Chartfile().Name != c.loader.chartfile().Name {
		t.Errorf("Unexpected chart file name: %s", c.Chartfile().Name)
	}

	d := c.DocsDir()
	if d != filepath.Join(testdir, preDocs) {
		t.Errorf("Unexpectedly, docs are in %s", d)
	}

	d = c.TemplatesDir()
	if d != filepath.Join(testdir, preTemplates) {
		t.Errorf("Unexpectedly, templates are in %s", d)
	}

	d = c.HooksDir()
	if d != filepath.Join(testdir, preHooks) {
		t.Errorf("Unexpectedly, hooks are in %s", d)
	}

	i, err := c.Icon()
	if err != nil {
		t.Errorf("No icon found in test chart: %s", err)
	}
	if i != filepath.Join(testdir, preIcon) {
		t.Errorf("Unexpectedly, icon is in %s", i)
	}
}
