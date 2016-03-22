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

package service

import (
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"

	"errors"
	"fmt"
	"net/http"

	restful "github.com/emicklei/go-restful"
)

// A Service wraps a web service that performs template expansion.
type Service struct {
	*restful.WebService
}

// NewService creates and returns a new Service, initialized with a new
// restful.WebService configured with a route that dispatches to the supplied
// handler. The new Service must be registered before accepting traffic by
// calling Register.
func NewService(handler restful.RouteFunction) *Service {
	restful.EnableTracing(true)
	webService := new(restful.WebService)
	webService.Consumes(restful.MIME_JSON, restful.MIME_XML)
	webService.Produces(restful.MIME_JSON, restful.MIME_XML)
	webService.Route(webService.POST("/expand").To(handler).
		Doc("Expand a template.").
		Reads(&common.ExpansionRequest{}).
		Writes(&common.ExpansionResponse{}))
	return &Service{webService}
}

// Register adds the web service wrapped by the Service to the supplied
// container. If the supplied container is nil, then the default container is
// used, instead.
func (s *Service) Register(container *restful.Container) {
	if container == nil {
		container = restful.DefaultContainer
	}

	container.Add(s.WebService)
}

// NewExpansionHandler returns a route function that handles an incoming
// template expansion request, bound to the supplied expander.
func NewExpansionHandler(backend common.Expander) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		util.LogHandlerEntry("expandybird: expand", req.Request)
		request := &common.ExpansionRequest{}
		if err := req.ReadEntity(&request); err != nil {
			logAndReturnErrorFromHandler(http.StatusBadRequest, err.Error(), resp)
			return
		}

		response, err := backend.ExpandChart(request)
		if err != nil {
			message := fmt.Sprintf("error expanding chart: %s", err)
			logAndReturnErrorFromHandler(http.StatusBadRequest, message, resp)
			return
		}

		util.LogHandlerExit("expandybird", http.StatusOK, "OK", resp.ResponseWriter)
		message := fmt.Sprintf("\nResources:\n%s\n", response.Resources)
		util.LogHandlerText("expandybird", message)
		resp.WriteEntity(response)
	}
}

func logAndReturnErrorFromHandler(statusCode int, message string, resp *restful.Response) {
	util.LogHandlerExit("expandybird: expand", statusCode, message, resp.ResponseWriter)
	resp.WriteError(statusCode, errors.New(message))
}
