package chart

import (
	"io/ioutil"
	"os"
	"testing"
)

var testChart = `#helm:generate foo
name: frobniz
description: This is a frobniz.
version: 1.2.3-alpha.1+12345
keywords:
	- frobnitz
	- sprocket
	- dodad
maintainers:
	- name: The Helm Team
	  email: helm@example.com
	- name: Someone Else
	  email: nobody@example.com
source: https://example.com/foo/bar
home: http://example.com
dependencies:
	- name: thingerbob
	  location: https://example.com/charts/thingerbob-3.2.1.tgz
	  version: ^3
environment:
	- name: Kubernetes
	  version: ~1.1
	  extensions:
	  	- extensions/v1beta1
	  	- extensions/v1beta1/daemonset
	  apiGroups:
	  	- 3rdParty
`

func TestLoadChartfile(t *testing.T) {
	out, err := ioutil.TempFile("", "chartfile-")
	if err != nil {
		t.Fatal(err)
	}
	tname := out.Name()
	defer func() {
		os.Remove(tname)
	}()

	out.Write([]byte(testChart))
	out.Close()

	c, err := LoadChartfile(tname)
	if err != nil {
		t.Errorf("Failed to open %s: %s", tname, err)
		return
	}

	if len(c.Environment[0].Extensions) != 2 {
		t.Errorf("Expected two extensions, got %d", len(c.Environment[0].Extensions))
	}
}
