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

type testHelm struct {
	t      *testing.T
	mux    *http.ServeMux
	server *httptest.Server
	app    *cli.App
}

func setup() *testHelm {
	th := &testHelm{}

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

func (th *testHelm) teardown() {
	th.server.Close()
}

func (th *testHelm) URL() string {
	return th.server.URL
}

func (th *testHelm) Run(args ...string) {
	args = append([]string{"helm", "--host", th.URL()}, args...)
	th.app.Run(args)
}

// CaptureOutput redirect all log/std streams, capture and replace
func CaptureOutput(fn func()) string {
	logStderr := format.Stderr
	logStdout := format.Stdout
	osStdout := os.Stdout
	osStderr := os.Stderr

	defer func() {
		format.Stderr = logStderr
		format.Stdout = logStdout
		os.Stdout = osStdout
		os.Stderr = osStderr
	}()

	r, w, _ := os.Pipe()

	format.Stderr = w
	format.Stdout = w
	os.Stdout = w
	os.Stderr = w

	fn()

	// read test output and restore previous stdout
	w.Close()
	b, _ := ioutil.ReadAll(r)
	return string(b)
}

func TestHelm(t *testing.T) {
	th := setup()
	defer th.teardown()

	th.mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["guestbook.yaml"]`))
	})

	expected := "guestbook.yaml\n"

	actual := CaptureOutput(func() {
		th.Run("deployment", "list")
	})

	if expected != actual {
		t.Errorf("Expected %v got %v", expected, actual)
	}
}
