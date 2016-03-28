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

package expansion

import (
	"github.com/kubernetes/helm/pkg/util"

	"errors"
	"fmt"
	"net/http"

	restful "github.com/emicklei/go-restful"
)

// A Service wraps a web service that performs template expansion.
type Service struct {
	webService *restful.WebService
	server     *http.Server
	container  *restful.Container
}

// NewService encapsulates code to open an HTTP server on the given address:port that serves the
// expansion API using the given Expander backend to do the actual expansion.  After calling
// NewService, call ListenAndServe to start the returned service.
func NewService(address string, port int, backend Expander) *Service {

	restful.EnableTracing(true)
	webService := new(restful.WebService)
	webService.Consumes(restful.MIME_JSON)
	webService.Produces(restful.MIME_JSON)
	handler := func(req *restful.Request, resp *restful.Response) {
		util.LogHandlerEntry("expansion service", req.Request)
		request := &ServiceRequest{}
		if err := req.ReadEntity(&request); err != nil {
			badRequest(resp, err.Error())
			return
		}
		response, err := backend.ExpandChart(request)
		if err != nil {
			badRequest(resp, fmt.Sprintf("error expanding chart: %s", err))
			return
		}
		util.LogHandlerExit("expansion service", http.StatusOK, "OK", resp.ResponseWriter)
		message := fmt.Sprintf("\nResources:\n%s\n", response.Resources)
		util.LogHandlerText("expansion service", message)
		resp.WriteEntity(response)
	}
	webService.Route(
		webService.POST("/expand").
			To(handler).
			Doc("Expand a chart.").
			Reads(&ServiceRequest{}).
			Writes(&ServiceResponse{}))

	container := restful.DefaultContainer
	container.Add(webService)
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", address, port),
		Handler: container,
	}

	return &Service{
		webService: webService,
		server:     server,
		container:  container,
	}
}

// ListenAndServe blocks forever, handling expansion requests.
func (s *Service) ListenAndServe() error {
	return s.server.ListenAndServe()
}

func badRequest(resp *restful.Response, message string) {
	statusCode := http.StatusBadRequest
	util.LogHandlerExit("expansion service", statusCode, message, resp.ResponseWriter)
	resp.WriteError(statusCode, errors.New(message))
}
