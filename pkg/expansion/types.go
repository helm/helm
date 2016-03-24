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
	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
)

// ServiceRequest defines the API to expander.
type ServiceRequest struct {
	ChartInvocation *common.Resource `json:"chart_invocation"`
	Chart           *chart.Content   `json:"chart"`
}

// ServiceResponse defines the API to expander.
type ServiceResponse struct {
	Resources []interface{} `json:"resources"`
}

// Expander abstracts interactions with the expander and deployer services.
type Expander interface {
	ExpandChart(request *ServiceRequest) (*ServiceResponse, error)
}
