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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/format"
)

type testHelmData struct {
	t      *testing.T
	mux    *http.ServeMux
	server *httptest.Server
	app    *cli.App
	output string
}

func testHelm(t *testing.T) *testHelmData {
	th := &testHelmData{t: t}

	th.app = cli.NewApp()
	th.app.Commands = commands

	th.app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "host,u",
			Value: "https://localhost:8000/",
		},
		cli.IntFlag{
			Name:  "timeout",
			Value: 10,
		},
		cli.BoolFlag{
			Name: "debug",
		},
	}
	th.app.Before = func(c *cli.Context) error {
		debug = c.GlobalBool("debug")
		return nil
	}

	th.mux = http.NewServeMux()
	th.server = httptest.NewServer(th.mux)

	return th
}

func (th *testHelmData) cleanup() {
	th.server.Close()
}

func (th *testHelmData) URL() string {
	return th.server.URL
}

// must gives a fatal error if err is not nil.
func (th *testHelmData) must(err error) {
	if err != nil {
		th.t.Fatal(err)
	}
}

// check gives a test non-fatal error if err is not nil.
func (th *testHelmData) check(err error) {
	if err != nil {
		th.t.Error(err)
	}
}

func (th *testHelmData) run(args ...string) {
	th.output = ""
	args = append([]string{"helm", "--host", th.URL()}, args...)
	th.output = captureOutput(func() {
		th.app.Run(args)
	})
}

// captureOutput redirect all log/std streams, capture and replace
func captureOutput(fn func()) string {
	osStdout, osStderr := os.Stdout, os.Stderr
	logStdout, logStderr := format.Stdout, format.Stderr
	defer func() {
		os.Stdout, os.Stderr = osStdout, osStderr
		format.Stdout, format.Stderr = logStdout, logStderr
	}()

	r, w, _ := os.Pipe()

	os.Stdout, os.Stderr = w, w
	format.Stdout, format.Stderr = w, w

	fn()

	// read test output and restore previous stdout
	w.Close()
	b, _ := ioutil.ReadAll(r)
	return string(b)
}
