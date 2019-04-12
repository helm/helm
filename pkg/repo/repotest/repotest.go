/*
Copyright The Helm Authors.

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

package repotest // import "helm.sh/helm/pkg/repo/repotest"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	// register inmemory driver for testing
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/listener"
)

type Server struct {
	Config *configuration.Configuration

	ln     net.Listener
	server *http.Server
	url    string
}

// NewServer returns a new Server and starts it.
//
// The caller should call Close or Shutdown when finished to shut it down.
func NewServer() *Server {
	s := NewUnstartedServer()
	s.Start()
	return s
}

// NewUnstartedServer returns a new Server but doesn't start it.
//
// After changing its configuration, the caller should call Start.
//
// The caller should call Close or Shutdown when finished to shut it down.
func NewUnstartedServer() *Server {
	config := &configuration.Configuration{
		Version: "0.1",
		Log: struct {
			AccessLog struct {
				Disabled bool `yaml:"disabled,omitempty"`
			} `yaml:"accesslog,omitempty"`
			Level     configuration.Loglevel  `yaml:"level,omitempty"`
			Formatter string                  `yaml:"formatter,omitempty"`
			Fields    map[string]interface{}  `yaml:"fields,omitempty"`
			Hooks     []configuration.LogHook `yaml:"hooks,omitempty"`
		}{
			Level: configuration.Loglevel("error"),
		},
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{},
		},
	}
	config.HTTP.Secret = "registrytest"

	app := handlers.NewApp(context.Background(), config)
	app.RegisterHealthChecks()

	server := &http.Server{
		Handler: app,
	}

	return &Server{
		Config: config,
		server: server,
	}
}

// Start starts a server from NewUnstartedServer.
func (s *Server) Start() {
	if s.url != "" {
		panic("server already started")
	}

	// only support localhost TCP listener addresses so we can determine the URL
	// TODO: support changes to s.Config.HTTP.Addr
	ln, err := listener.NewListener("", "")
	if err != nil {
		panic(err)
	}
	s.ln = ln
	s.url = fmt.Sprintf("%s://%s", s.ln.Addr().Network(), s.ln.Addr().String())

	// Start serving in goroutine and listen for stop signal from Stop or Shutdown
	go s.server.Serve(s.ln)
}

// URL returns the server's network address. This is compatible with Docker's URL syntax so it can be used
// in conjunction with reference.Parse
func (s *Server) URL() string {
	if s.url == "" {
		return ""
	}
	return fmt.Sprintf("localhost:%s", strings.TrimPrefix(s.url, "tcp://[::]:"))
}

// Close immediately closes all active net.Listeners and any active client connections. For a graceful shutdown, use Shutdown.
//
// Close returns any error returned from closing the Server's underlying Listener(s).
func (s *Server) Close() error {
	s.url = ""
	return s.server.Close()
}

// Shutdown shuts down the server and blocks until all outstanding requests on this server have completed.
//
// When Shutdown is called, Start immediately returns http.ErrServerClosed. Make sure the program doesn't exit and waits instead for Shutdown to return.
func (s *Server) Shutdown() error {
	s.url = ""
	// shutdown the server with a grace period of configured timeout
	c, cancel := context.WithTimeout(context.Background(), s.Config.HTTP.DrainTimeout)
	defer cancel()
	return s.server.Shutdown(c)
}
